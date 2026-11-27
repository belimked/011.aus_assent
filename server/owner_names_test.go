package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// UAT 3 item 6: every team agent can read registered owner names on demand;
// property reads and sync never carry them; each view is logged.
func TestOwnerNamesOnDemand(t *testing.T) {
	a, tokens := setup(t)
	john, mary, admin := tokens["john"], tokens["mary"], tokens["admi"]
	if err := createUser(a.db, "other@axleo.local", "Other", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	if err := createUser(a.db, "lead@axleo.local", "Lead", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE users SET team_id='other' WHERE email='other@axleo.local'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE users SET role='leader' WHERE email='lead@axleo.local'`); err != nil {
		t.Fatal(err)
	}
	login := func(email string) string {
		w := request(a, "POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": "password123", "device": "test"})
		if w.Code != 200 {
			t.Fatal(email, w.Body)
		}
		return obj(t, w)["token"].(string)
	}
	other, leader := login("other@axleo.local"), login("lead@axleo.local")
	var johnID, maryID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&johnID)
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&maryID)

	rp := `{"rpdataAttributes":{"propertyType":"House","owners":"Rp Fallback Owner","vendors":"Some Vendor","updatedAt":"2026-11-25T00:00:00Z"}}`
	for _, s := range []struct{ id, team, data string }{
		{"own-1", "default", rp},             // ownership row + RP attributes
		{"own-2", "default", rp},             // RP attributes only
		{"own-3", "default", `{"note":"x"}`}, // ownership row without names
		{"own-x", "other", rp},               // another team
		{"own-del", "default", rp},           // deleted
	} {
		if _, err := a.db.Exec(`INSERT INTO properties(id,team_id,no,street,suburb,data,created_at,updated_at,registered) VALUES(?,?,'15','Norinda St','Sunnybank',?,?,?,1)`, s.id, s.team, s.data, now(), now()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.db.Exec(`UPDATE properties SET data=json_set(data,'$.id',id) WHERE id GLOB 'own-*'`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE properties SET deleted_at=? WHERE id='own-del'`, now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES('own-import','file','rpdata','own.xlsx','own-sha',?,'completed')`, now()); err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		id, names string
		absentee  int
	}{{"own-1", "Jane Citizen & John Citizen", 1}, {"own-3", "", 0}, {"own-x", "Other Team Owner", 0}} {
		if _, err := a.db.Exec(`INSERT INTO property_ownership(property_id,owner_names,last_sale_price,last_sale_date,source,import_id,updated_at,absentee_owner) VALUES(?,?,800000,'2015-03-01','rpdata','own-import','2026-11-25T01:00:00Z',?)`, s.id, s.names, s.absentee); err != nil {
			t.Fatal(err)
		}
	}
	var contactsBefore, outboxBefore int
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts`).Scan(&contactsBefore)
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox`).Scan(&outboxBefore)

	// Sync: no names for anyone, hasOwners for properties that have them.
	for who, token := range map[string]string{"admin": admin, "john": john} {
		pull := obj(t, request(a, "GET", "/api/v1/sync", token, nil))
		seen := map[string]bool{}
		for _, it := range pull["properties"].([]any) {
			prop := it.(map[string]any)
			id := str(prop, "id")
			if !strings.HasPrefix(id, "own-") {
				continue
			}
			seen[id] = true
			market, _ := prop["market"].(map[string]any)
			if _, ok := market["owners"]; ok {
				t.Fatalf("%s: market.owners synced for %s", who, id)
			}
			if _, ok := market["absenteeOwner"]; ok {
				t.Fatalf("%s: market.absenteeOwner synced for %s", who, id)
			}
			if attrs, ok := prop["rpdataAttributes"].(map[string]any); ok && (attrs["owners"] != nil || attrs["vendors"] != nil) {
				t.Fatalf("%s: rpdataAttributes names synced for %s", who, id)
			}
			if want := id != "own-3"; market["hasOwners"] != want {
				t.Fatalf("%s: %s hasOwners %v", who, id, market["hasOwners"])
			}
			if b, _ := json.Marshal(prop); strings.Contains(string(b), "Citizen") || strings.Contains(string(b), "Rp Fallback") {
				t.Fatalf("%s: name in synced %s: %s", who, id, b)
			}
		}
		if !seen["own-1"] || !seen["own-2"] {
			t.Fatalf("%s: fixtures not synced %v", who, seen)
		}
	}

	get := func(token, id string) (int, map[string]any) {
		w := request(a, "GET", "/api/v1/properties/"+id+"/owner-names", token, nil)
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		return w.Code, m
	}
	views := func(user, prop string) int {
		var n int
		_ = a.db.QueryRow(`SELECT count(*) FROM owner_name_views WHERE user_id=? AND property_id=?`, user, prop).Scan(&n)
		return n
	}
	// John has no zone covering these properties.
	if a.covered("own-1", johnID) {
		t.Fatal("fixture: john covers own-1")
	}
	appConnGuard(t, a, "owner names", func() error {
		code, m := get(john, "own-1")
		if code != 200 || m["owners"] != "Jane Citizen & John Citizen" || m["absenteeOwner"] != true || m["source"] != "rpdata" || m["updatedAt"] != "2026-11-25T01:00:00Z" || m["notice"] != "RP Data · not for direct marketing" {
			return fmt.Errorf("own-1 %d %v", code, m)
		}
		return nil
	})
	if code, m := get(john, "own-2"); code != 200 || m["owners"] != "Rp Fallback Owner" || m["absenteeOwner"] != false || m["source"] != "rpdata" || m["updatedAt"] != "2026-11-25T00:00:00Z" {
		t.Fatal("own-2 fallback", code, m)
	}
	if code, m := get(john, "own-3"); code != 200 || m["owners"] != "" || m["notice"] == nil {
		t.Fatal("own-3 empty", code, m)
	}
	if views(johnID, "own-3") != 0 {
		t.Fatal("empty result logged")
	}
	for _, c := range []struct{ token, id string }{{other, "own-1"}, {john, "own-x"}, {admin, "own-x"}, {john, "own-del"}, {john, "missing"}} {
		if code, _ := get(c.token, c.id); code != 404 {
			t.Fatalf("%s: want 404, got %d", c.id, code)
		}
	}
	if code, m := get(other, "own-x"); code != 200 || m["owners"] != "Other Team Owner" {
		t.Fatal("own team", code, m)
	}
	if w := request(a, "GET", "/api/v1/properties/own-1/owner-names", "", nil); w.Code != 401 {
		t.Fatal("unauthenticated", w.Code)
	}

	// Logging, with a 10-minute window per user and property.
	if views(johnID, "own-1") != 1 {
		t.Fatal("first view not logged", views(johnID, "own-1"))
	}
	get(john, "own-1")
	if views(johnID, "own-1") != 1 {
		t.Fatal("repeat view inside 10 minutes logged again")
	}
	get(mary, "own-1")
	if views(maryID, "own-1") != 1 {
		t.Fatal("another user's view not logged")
	}
	if _, err := a.db.Exec(`UPDATE owner_name_views SET at=? WHERE user_id=? AND property_id='own-1'`, time.Now().UTC().Add(-11*time.Minute).Format(layout), johnID); err != nil {
		t.Fatal(err)
	}
	get(john, "own-1")
	if views(johnID, "own-1") != 2 {
		t.Fatal("view after 10 minutes not logged")
	}
	var team, source string
	_ = a.db.QueryRow(`SELECT team_id,source FROM owner_name_views WHERE user_id=? AND property_id='own-2'`, johnID).Scan(&team, &source)
	if team != "default" || source != "rpdata" {
		t.Fatal("view row", team, source)
	}

	// Admin log endpoint.
	for _, token := range []string{john, leader} {
		if w := request(a, "GET", "/api/v1/admin/owner-name-views", token, nil); w.Code == 200 {
			t.Fatal("non-admin read the view log")
		}
	}
	list := func(q string) []map[string]any {
		w := request(a, "GET", "/api/v1/admin/owner-name-views"+q, admin, nil)
		if w.Code != http.StatusOK {
			t.Fatal(q, w.Code, w.Body)
		}
		var out []map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	all := list("")
	if len(all) != 5 {
		t.Fatal("log size", len(all), all)
	}
	for i := 1; i < len(all); i++ {
		if all[i-1]["at"].(string) < all[i]["at"].(string) {
			t.Fatal("not newest first", all)
		}
	}
	if got := list("?propertyId=own-1&userId=" + maryID); len(got) != 1 || got[0]["userName"] != "Mary" || got[0]["address"] != "15 Norinda St, Sunnybank" || got[0]["propertyId"] != "own-1" || got[0]["userId"] != maryID {
		t.Fatal("filtered log", got)
	}
	if got := list("?limit=2"); len(got) != 2 {
		t.Fatal("limit", len(got))
	}
	if got := list("?limit=9999"); len(got) != 5 {
		t.Fatal("limit cap", len(got))
	}

	// Nothing flows out: no contacts, no Rex jobs, not exportable.
	var contactsAfter, outboxAfter int
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts`).Scan(&contactsAfter)
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox`).Scan(&outboxAfter)
	if contactsAfter != contactsBefore || outboxAfter != outboxBefore {
		t.Fatal("owner-name reads wrote contacts or Rex jobs")
	}
	for _, field := range []string{"ownerNames", "owners", "registeredOwner", "absenteeOwner"} {
		if w := request(a, "POST", "/api/v1/admin/export", admin, map[string]any{"fields": []string{"address", field}, "reason": "check", "acknowledge": true}); w.Code != 400 {
			t.Fatalf("export accepted %s: %d", field, w.Code)
		}
	}
	item := map[string]any{"ownerNames": "x", "nested": map[string]any{"ownerNames": "y", "absenteeOwner": true}}
	stripLicensed(item)
	if b, _ := json.Marshal(item); strings.Contains(string(b), "ownerNames") || strings.Contains(string(b), "absenteeOwner") {
		t.Fatal("licensed fields kept", string(b))
	}
}
