package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func workerDB(t *testing.T) *app {
	t.Helper()
	path := filepath.Join(".tmp", "rex-worker-"+strings.ReplaceAll(t.Name(), "/", "-"), "db.sqlite")
	t.Cleanup(func() { os.RemoveAll(filepath.Dir(path)) })
	db, e := open(path)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { db.Close() })
	if e = migrate(db, path); e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`INSERT INTO users(id,email,name,role,password_hash,created_at,updated_at) VALUES('u','u@x','Mary','admin','hash','now','now')`)
	if e != nil {
		t.Fatal(e)
	}
	return &app{db: db}
}
func workerQueue(t *testing.T, a *app, entity, id, payload string) {
	t.Helper()
	_, e := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES(?,?, 'POST',?,'now')`, entity, id, payload)
	if e != nil {
		t.Fatal(e)
	}
}
func workerStatus(t *testing.T, a *app, id int) string {
	t.Helper()
	var s string
	if e := a.db.QueryRow(`SELECT status FROM rex_outbox WHERE id=?`, id).Scan(&s); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestRexWorkerContactLeadAndLink(t *testing.T) {
	a := workerDB(t)
	_, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u',?,'now','now')`, `{"first":"Jane","last":"Doe","mobile":"0411111111"}`)
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.db.Exec(`INSERT INTO leads(id,contact_id,owner_id,data,created_at,updated_at) VALUES('l','c','u',?,'now','now')`, `{"note":"Interested"}`)
	if e != nil {
		t.Fatal(e)
	}
	workerQueue(t, a, "contacts", "c", `{"first":"Jane"}`)
	workerQueue(t, a, "leads", "l", `{"contactId":"c"}`)
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.URL.Path)
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		var result any = "token"
		switch r.URL.Path {
		case "/v1/rex/contacts/create":
			result = map[string]any{"id": "91"}
		case "/v1/rex/leads/create":
			data := b["data"].(map[string]any)
			if data["contact"].(map[string]any)["id"] != "91" || !strings.Contains(data["note"].(string), "via AxleoCollect · Mary") {
				t.Errorf("lead mapping: %v", data)
			}
			result = map[string]any{"id": "92"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p", account: "a"}
	w.client.Interval = 0
	w.client.Retries = 0
	for i := 0; i < 2; i++ {
		if e := w.step(context.Background()); e != nil {
			t.Fatal(e)
		}
	}
	if workerStatus(t, a, 1) != "sent" || workerStatus(t, a, 2) != "sent" || w.link("leads", "l") != "92" {
		t.Fatalf("status/link: %s %s", workerStatus(t, a, 1), workerStatus(t, a, 2))
	}
	_, e = a.db.Exec(`UPDATE rex_outbox SET status='pending' WHERE id=1`)
	if e != nil {
		t.Fatal(e)
	}
	if e = w.step(context.Background()); e != nil {
		t.Fatal(e)
	}
	creates := 0
	for _, p := range calls {
		if p == "/v1/rex/contacts/create" {
			creates++
		}
	}
	if creates != 1 {
		t.Fatalf("duplicate contact: %v", calls)
	}
}
func TestRexWorkerDisabledAnd429(t *testing.T) {
	a := workerDB(t)
	workerQueue(t, a, "enquiries", "e", `{}`)
	t.Setenv("AXLEO_REX_ENABLED", "0")
	if newRexWorker(a.db) != nil {
		t.Fatal("worker enabled")
	}
	if workerStatus(t, a, 1) != "waiting_auth" {
		t.Fatal("changed while disabled")
	}
	_, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{}','now','now')`)
	if e != nil {
		t.Fatal(e)
	}
	workerQueue(t, a, "contacts", "c", `{}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "login") {
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "token", "error": nil})
			return
		}
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(429)
	}))
	defer srv.Close()
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p"}
	w.client.Interval = 0
	w.client.Retries = 0
	_ = w.step(context.Background())
	_ = w.step(context.Background())
	if workerStatus(t, a, 2) != "pending" {
		t.Fatal("429 not deferred")
	}
	var tries int
	_ = a.db.QueryRow(`SELECT tries FROM rex_outbox WHERE id=2`).Scan(&tries)
	if tries != 1 {
		t.Fatal(tries)
	}
}
