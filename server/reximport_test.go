package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRexImportReadOnlyAndReplay(t *testing.T) {
	a, tokens := setup(t)
	ids := []string{}
	for _, email := range []string{"mary@axleo.local", "john@axleo.local"} {
		var id string
		if err := a.db.QueryRow(`SELECT id FROM users WHERE email=?`, email).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var b map[string]any
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			t.Error(err)
		}
		offset := int(b["offset"].(float64))
		rows := []any{}
		switch r.URL.Path {
		case "/v1/rex/properties/search":
			if _, ok := b["extra_options"].(map[string]any); !ok {
				t.Error("missing relations option")
			}
			if offset == 0 {
				for i := 1; i <= 2; i++ {
					rows = append(rows, map[string]any{"id": string(rune('0' + i)), "etag": "v1", "adr_street_number": "10", "adr_street_name": "Main", "adr_suburb_or_town": "Sydney", "related": map[string]any{"contact_reln_property": []any{map[string]any{"contact": map[string]any{"id": string(rune('0' + i))}, "reln_type": map[string]any{"text": "owner"}}}}})
				}
			}
		case "/v1/rex/contacts/search":
			if offset == 0 {
				for i := 1; i <= 3; i++ {
					rows = append(rows, map[string]any{"id": string(rune('0' + i)), "etag": "v1", "name_first": "Test", "name_last": "Person", "is_dnd": "1", "contact_by_letter": "0"})
				}
			}
		default:
			t.Errorf("unexpected HTTP request: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"rows": rows, "total": len(rows)}, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Interval = 0
	c.AccountID = "2057"
	c.Log = rexDBLogger(a.db)
	if err := runRexImport(context.Background(), a.db, c, "2057", true, 0, ids); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts`).Scan(&n)
	if n != 0 {
		t.Fatal("dry-run wrote contacts")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_links`).Scan(&n)
	if n != 0 {
		t.Fatal("dry-run wrote links")
	}
	if _, _, err := c.Call(context.Background(), "Contacts", "update", map[string]any{"id": "1"}); err == nil {
		t.Fatal("write not blocked")
	}
	if requests != 2 {
		t.Fatalf("unexpected HTTP write: %d", requests)
	}
	if err := runRexImport(context.Background(), a.db, c, "2057", false, 0, ids); err != nil {
		t.Fatal(err)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE owner_id=?`, ids[0]).Scan(&n)
	if n != 1 {
		t.Fatalf("property assignment %d", n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts WHERE owner_id=?`, ids[0]).Scan(&n)
	if n != 2 {
		t.Fatalf("contact assignment %d", n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM property_owners`).Scan(&n)
	if n != 2 {
		t.Fatal(n)
	}
	if _, err := a.db.Exec(`UPDATE properties SET owner_id=? WHERE id='rex-properties-2057-1'`, ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := runRexImport(context.Background(), a.db, c, "2057", false, 0, ids); err != nil {
		t.Fatal(err)
	}
	var owner string
	_ = a.db.QueryRow(`SELECT owner_id FROM properties WHERE id='rex-properties-2057-1'`).Scan(&owner)
	if owner != ids[1] {
		t.Fatal("replay overwrote local assignment")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox`).Scan(&n)
	if n != 0 {
		t.Fatal("import queued Rex writes")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_request_log WHERE caller='import' AND kind='write' AND ok=0 AND error LIKE '%read-only%'`).Scan(&n)
	if n != 1 {
		t.Fatal("blocked write not logged", n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_request_log WHERE caller='import' AND kind='read'`).Scan(&n)
	if n < 6 {
		t.Fatal("missing read logs", n)
	}
	// Imported contacts now queue a linked update through the ordinary API.
	w := request(a, "PATCH", "/api/v1/contacts/rex-contacts-2057-1", tokens["mary"], map[string]any{"version": 1, "first": "Edited"})
	if w.Code != 200 {
		t.Fatalf("edit: %d %s", w.Code, w.Body.String())
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox`).Scan(&n)
	if n == 0 {
		t.Fatal("edit did not queue Rex update")
	}
}
