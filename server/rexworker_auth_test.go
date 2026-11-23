package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRexWorkerRejectedUpdateDoesNotSpin(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	workerQueue(t, a, "contacts", "c", `{}`)
	writes := 0
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		if path == "/v1/rex/contacts/read" {
			return map[string]any{"id": 91, "related": map[string]any{"contact_names": []any{map[string]any{"id": 11}}, "contact_phones": []any{map[string]any{"id": 22}}}}, 200
		}
		if path == "/v1/rex/contacts/update" {
			writes++
			return nil, 500
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	})
	for i := 0; i < 2; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if writes != 1 || workerStatus(t, a, 1) != "pending" {
		t.Fatalf("writes=%d status=%s", writes, workerStatus(t, a, 1))
	}
	var tries int
	_ = a.db.QueryRow(`SELECT tries FROM rex_outbox WHERE id=1`).Scan(&tries)
	if tries != 1 {
		t.Fatal(tries)
	}
}
func TestRexWorkerReloginReverifies(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	workerQueue(t, a, "contacts", "c", `{}`)
	t.Setenv("REX_WRITE_ACCOUNT_ID", "2057")
	logins, verified, writes := 0, 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/rex/authentication/login":
			logins++
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "token", "error": nil})
		case "/v1/rex/AccountSettings/read":
			verified++
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": 2057, "is_test_account": true}, "error": nil})
		case "/v1/rex/contacts/read":
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": 91, "related": map[string]any{"contact_names": []any{map[string]any{"id": 11}}, "contact_phones": []any{map[string]any{"id": 22}}}}, "error": nil})
		case "/v1/rex/contacts/update":
			writes++
			if writes == 1 {
				w.WriteHeader(401)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"type": "TokenException", "message": "expired"}})
			} else {
				_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": 91, "etag": "new"}, "error": nil})
			}
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p", account: "2057"}
	w.client.AccountID = "2057"
	w.client.Interval = 0
	w.client.Retries = 0
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if logins != 2 || verified != 2 || writes != 2 || workerStatus(t, a, 1) != "sent" {
		t.Fatalf("logins=%d verified=%d writes=%d status=%s", logins, verified, writes, workerStatus(t, a, 1))
	}
}
