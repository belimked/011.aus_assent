package main

import (
	"context"
	"testing"
)

func TestNormAUMobile(t *testing.T) {
	for _, s := range []string{"0401 999 088", "+61 401 999 088", "61401999088", "0401999088", "0061 401 999 088"} {
		if got := normAUMobile(s); got != "0401999088" {
			t.Errorf("%s: %s", s, got)
		}
	}
}
func TestPollOwnerMapping(t *testing.T) {
	a := workerDB(t)
	t.Setenv("REX_USER_MAP", "80559:u@x")
	t.Setenv("REX_DEFAULT_OWNER_EMAIL", "u@x")
	_, err := a.db.Exec(`INSERT INTO users(id,email,name,role,password_hash,created_at,updated_at) VALUES('v','v@x','Agent','agent','hash','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO properties(id,owner_id,data,registered,created_at,updated_at) VALUES('p','v','{"rex":{"system_owner_user":{"id":80559}}}',1,'now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','v','{"rex":{}}','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO property_owners(property_id,contact_id,relation) VALUES('p','c','owner')`)
	if err != nil {
		t.Fatal(err)
	}
	props, contacts := map[string]bool{"p": true}, map[string]bool{"c": true}
	if err = assignPollOwners(a.db, props, contacts); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"properties", "contacts"} {
		var owner string
		_ = a.db.QueryRow(`SELECT owner_id FROM `+table+` WHERE id=?`, map[string]string{"properties": "p", "contacts": "c"}[table]).Scan(&owner)
		if owner != "u" {
			t.Fatal(table, owner)
		}
	}
}
func TestCreateLinkThenRelation(t *testing.T) {
	a := workerDB(t)
	_, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{"first":"Teddie","last":"Bear","mobile":"+61 401 999 088"}','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO properties(id,owner_id,data,registered,created_at,updated_at) VALUES('p','u','{}',1,'now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('properties','p','123','Properties','2057','now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts','c','POST','{}','now')`)
	if err != nil {
		t.Fatal(err)
	}
	workerQueue(t, a, "rex_contact_relation", "c:p", `{"contactId":"c","relation":{"type":"Owner of","propertyId":"p"}}`)
	creates, relations := 0, 0
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/create":
			creates++
			return map[string]any{"id": 456}, 200
		case "/v1/rex/properties/update":
			relations++
			return map[string]any{"id": 123}, 200
		case "/v1/rex/properties/read":
			return map[string]any{"id": 123, "etag": "new"}, 200
		}
		t.Errorf("unexpected call %s", path)
		return nil, 500
	})
	for i := 0; i < 2; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if creates != 1 || relations != 1 || workerStatus(t, a, 1) != "sent" || workerStatus(t, a, 2) != "sent" {
		t.Fatal(creates, relations, workerStatus(t, a, 1), workerStatus(t, a, 2))
	}
	var linked string
	_ = a.db.QueryRow(`SELECT rex_id FROM rex_links WHERE entity='contacts' AND local_id='c'`).Scan(&linked)
	if linked != "456" {
		t.Fatal(linked)
	}
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if creates != 1 {
		t.Fatal("duplicate create")
	}
}
