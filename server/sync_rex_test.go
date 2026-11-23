package main

import "testing"

func TestSyncOmitsRexRawDataButDetailsRetainIt(t *testing.T) {
	a, tokens := setup(t)
	var mary string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('rex-c',?, '{"id":"rex-c","source":"rex_import","rex":{"email_address":"secret@example.com"}}','now','now')`,
		`INSERT INTO properties(id,owner_id,data,registered,created_at,updated_at) VALUES('rex-p',?, '{"id":"rex-p","source":"rex_import","geometry":[[[151,-33]]],"rex":{"adr_street_name":"secret"}}',1,'now','now')`,
	} {
		if _, err := a.db.Exec(q, mary); err != nil {
			t.Fatal(err)
		}
	}
	assertPull := func(url string) {
		t.Helper()
		w := request(a, "GET", url, tokens["mary"], nil)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
		m := obj(t, w)
		for _, kind := range []string{"contacts", "properties"} {
			rows := m[kind].([]any)
			if len(rows) != 1 {
				t.Fatalf("%s: %d", kind, len(rows))
			}
			row := rows[0].(map[string]any)
			if _, ok := row["rex"]; ok {
				t.Fatalf("raw Rex leaked through %s sync", kind)
			}
			if kind == "properties" {
				if _, ok := row["geometry"]; ok {
					t.Fatal("parcel geometry leaked into sync")
				}
			}
			if row["source"] != "rex_import" {
				t.Fatal("source tag lost", row)
			}
		}
	}
	initial := obj(t, request(a, "GET", "/api/v1/sync", tokens["mary"], nil))
	assertPull("/api/v1/sync")
	if _, err := a.db.Exec(`UPDATE contacts SET updated_at='later' WHERE id='rex-c'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE properties SET updated_at='later' WHERE id='rex-p'`); err != nil {
		t.Fatal(err)
	}
	assertPull("/api/v1/sync?since=" + initial["cursor"].(string))
	for _, kind := range []string{"contacts", "properties"} {
		id := "rex-c"
		if kind == "properties" {
			id = "rex-p"
		}
		w := request(a, "GET", "/api/v1/"+kind+"/"+id, tokens["mary"], nil)
		if w.Code != 200 || obj(t, w)["rex"] == nil {
			t.Fatal("detail lost Rex provenance", kind, w.Code)
		}
		if kind == "properties" && obj(t, w)["geometry"] == nil {
			t.Fatal("detail lost parcel geometry")
		}
	}
}
