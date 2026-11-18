package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRexWorkerAmbiguousCreateStops(t *testing.T) {
	a := workerDB(t)
	_, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{}','now','now')`)
	if e != nil {
		t.Fatal(e)
	}
	workerQueue(t, a, "contacts", "c", `{}`)
	writes := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "login") {
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "token", "error": nil})
			return
		}
		writes++
		w.WriteHeader(500)
	}))
	defer srv.Close()
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p"}
	w.client.Interval = 0
	w.client.Retries = 0
	_ = w.step(context.Background())
	_ = w.step(context.Background())
	if writes != 1 || workerStatus(t, a, 1) != "failed" {
		t.Fatalf("ambiguous write replayed: %d", writes)
	}
}
func TestRexWorkerReloginAndUnknownWrite(t *testing.T) {
	a := workerDB(t)
	_, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{}','now','now')`)
	if e != nil {
		t.Fatal(e)
	}
	workerQueue(t, a, "contacts", "c", `{}`)
	logins, writes := 0, 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/rex/authentication/login" {
			logins++
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "token", "error": nil})
			return
		}
		writes++
		if writes == 1 {
			w.WriteHeader(401)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"type": "TokenException", "message": "expired"}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": "80"}, "error": nil})
	}))
	defer srv.Close()
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p"}
	w.client.Interval = 0
	w.client.Retries = 0
	if e = w.step(context.Background()); e != nil {
		t.Fatal(e)
	}
	if logins != 2 || writes != 2 || w.link("contacts", "c") != "80" {
		t.Fatalf("login=%d writes=%d link=%s", logins, writes, w.link("contacts", "c"))
	}
	workerQueue(t, a, "contacts", "c", `{}`)
	if e = w.step(context.Background()); e != nil {
		t.Fatal(e)
	} // update, never create again
	if writes != 3 || logins != 2 {
		t.Fatalf("unexpected calls %d %d", writes, logins)
	}
}
