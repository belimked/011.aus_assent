package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPollSavesOwnerRelationWithoutPropertyEtagChange(t *testing.T) {
	a, _ := setup(t)
	if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REX_POLL_SOFT_DELETE", "0")
	t.Setenv("REX_DEFAULT_OWNER_EMAIL", "mary@axleo.local")
	owner := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		var result any
		switch r.URL.Path {
		case "/v1/rex/properties/search":
			result = map[string]any{"rows": []any{map[string]any{"id": "123", "etag": "a"}}, "total": 1}
		case "/v1/rex/contacts/search":
			result = map[string]any{"rows": []any{}, "total": 0}
		case "/v1/rex/properties/read":
			rel := []any{}
			if owner {
				rel = append(rel, map[string]any{"contact": map[string]any{"id": "456"}, "reln_type": map[string]any{"id": "owner"}})
			}
			result = map[string]any{"id": "123", "etag": "a", "adr_street_number": "16", "adr_street_name": "Gager Street", "related": map[string]any{"contact_reln_property": rel}}
		case "/v1/rex/contacts/read":
			result = map[string]any{"id": "456", "etag": "b", "related": map[string]any{"contact_names": []any{map[string]any{"name_first": "Teddie", "name_last": "Bear"}}, "contact_phones": []any{map[string]any{"phone_number": "0401 999 088"}}}}
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token = "mock"
	c.Interval = 0
	c.ReadOnly = true
	c.AccountID = "2057"
	lastRelationPoll = time.Now()
	if err := pollRex(context.Background(), a.db, c, "2057"); err != nil {
		t.Fatal(err)
	}
	owner = true
	lastRelationPoll = time.Time{}
	if err := pollRex(context.Background(), a.db, c, "2057"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := a.db.QueryRow(`SELECT count(*) FROM property_owners WHERE relation='owner' AND contact_id='rex-contacts-2057-456'`).Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	var assigned string
	_ = a.db.QueryRow(`SELECT u.email FROM users u JOIN properties p ON p.owner_id=u.id WHERE p.id='rex-properties-2057-123'`).Scan(&assigned)
	if assigned != "mary@axleo.local" {
		t.Fatal(assigned)
	}
}
