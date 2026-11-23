package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSandboxImportPlan(t *testing.T) {
	a, _ := setup(t)
	if _, err := sandboxUsers(a.db); err == nil {
		t.Fatal("missing import account accepted")
	}
	if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	ids, err := sandboxUsers(a.db)
	if err != nil {
		t.Fatal(err)
	}
	// More than both property caps; linked contacts win over unlinked fillers.
	np, nc := 2140, 4345
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			t.Error(err)
		}
		offset := int(b["offset"].(float64))
		size := int(b["limit"].(float64))
		rows := []any{}
		total := np
		prop := r.URL.Path == "/v1/rex/properties/search"
		if !prop {
			total = nc
		}
		for i := offset + 1; i <= total && i <= offset+size; i++ {
			id := fmt.Sprintf("%05d", i)
			if prop {
				row := map[string]any{"id": id, "etag": "1", "adr_latitude": "-27.5794", "adr_longitude": fmt.Sprintf("%.5f", 153.0606+float64(total-i)*0.00001)}
				if i <= np-1 {
					row["related"] = map[string]any{"contact_reln_property": []any{map[string]any{"contact": map[string]any{"id": id}}}}
				}
				rows = append(rows, row)
			} else {
				rows = append(rows, map[string]any{"id": id, "etag": "1", "name_first": "Test"})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"rows": rows, "total": total}, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Interval = 0
	run := func() {
		t.Helper()
		if err := runSandboxImport(context.Background(), a.db, c, "2057", false, 0, ids); err != nil {
			t.Fatal(err)
		}
	}
	run()
	var n int
	if err := a.db.QueryRow(`SELECT count(*) FROM sessions WHERE user_id IN (?,?,?,?)`, ids["mary"], ids["john"], ids["import"], ids["admin"]).Scan(&n); err != nil || n != 0 {
		t.Fatalf("assignment did not revoke sessions: %d %v", n, err)
	}
	w := request(a, "POST", "/api/v1/auth/login", "", map[string]any{"email": "mary@axleo.local", "password": "password123", "device": "replay"})
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, v := range []struct {
		name                 string
		properties, contacts int
	}{{"mary", 200, 200}, {"john", 200, 200}, {"import", 1734, 3938}, {"admin", 6, 7}} {
		if err := a.db.QueryRow(`SELECT count(*) FROM properties WHERE owner_id=? AND json_extract(data,'$.source')='rex_import'`, ids[v.name]).Scan(&n); err != nil || n != v.properties {
			t.Fatalf("%s properties: %d %v", v.name, n, err)
		}
		if err := a.db.QueryRow(`SELECT count(*) FROM contacts WHERE owner_id=? AND json_extract(data,'$.source')='rex_import'`, ids[v.name]).Scan(&n); err != nil || n != v.contacts {
			t.Fatalf("%s contacts: %d %v", v.name, n, err)
		}
	}
	// Relations precede distance: first ranked property is closest linked one (2140 has no relation).
	for _, v := range []struct {
		number int
		name   string
	}{{2139, "mary"}, {2138, "john"}, {1739, "import"}, {2140, "admin"}} {
		id := fmt.Sprintf("rex-properties-2057-%05d", v.number)
		var owner string
		if err := a.db.QueryRow(`SELECT owner_id FROM properties WHERE id=?`, id).Scan(&owner); err != nil || owner != ids[v.name] {
			t.Fatalf("%s owner=%s want=%s: %v", id, owner, v.name, err)
		}
	}
	// Replay must leave locally changed assignments untouched (including the JSON owner).
	if _, err := a.db.Exec(`UPDATE contacts SET owner_id=?,data=json_set(data,'$.owner',?) WHERE id='rex-contacts-2057-02139'`, ids["john"], ids["john"]); err != nil {
		t.Fatal(err)
	}
	run()
	if err := a.db.QueryRow(`SELECT count(*) FROM sessions WHERE user_id=?`, ids["mary"]).Scan(&n); err != nil || n != 1 {
		t.Fatalf("replay revoked session: %d %v", n, err)
	}
	var owner, jsonOwner string
	if err := a.db.QueryRow(`SELECT owner_id,json_extract(data,'$.owner') FROM contacts WHERE id='rex-contacts-2057-02139'`).Scan(&owner, &jsonOwner); err != nil || owner != ids["john"] || jsonOwner != owner {
		t.Fatalf("replay changed owner %s %s %v", owner, jsonOwner, err)
	}
	if _, err := a.db.Exec(`UPDATE contacts SET owner_id=?,data=json_set(data,'$.owner',?) WHERE id='rex-contacts-2057-02139'`, ids["mary"], ids["mary"]); err != nil {
		t.Fatal(err)
	}
	np, nc = 2141, 4346
	run()
	for _, kind := range []string{"properties", "contacts"} {
		id := "rex-" + kind + "-2057-"
		if kind == "properties" {
			id += "02141"
		} else {
			id += "04346"
		}
		want := ids["admin"]
		if kind == "contacts" {
			want = ids["import"]
		} // Existing local reassignment can free an import slot.
		if err := a.db.QueryRow(`SELECT owner_id FROM `+kind+` WHERE id=?`, id).Scan(&owner); err != nil || owner != want {
			t.Fatalf("incremental %s: %s want=%s %v", id, owner, want, err)
		}
	}
}
