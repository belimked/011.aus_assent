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
	_, e := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES(?,?, 'PATCH',?,'now')`, entity, id, payload)
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
func linkedContact(t *testing.T, a *app) {
	t.Helper()
	_, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u',?,'now','now')`, `{"source":"rex_import","first":"Jane","last":"Doe","mobile":"0499999999"}`)
	if e != nil {
		t.Fatal(e)
	}
	_, e = a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at,etag) VALUES('contacts','c','91','Contacts','2057','now','old')`)
	if e != nil {
		t.Fatal(e)
	}
}

// rexTestError returned by a sandboxWorker handler is sent as {"error":{"message":...}}.
type rexTestError string

func sandboxWorker(t *testing.T, a *app, handle func(string, map[string]any) (any, int)) (*rexWorker, *[]string) {
	t.Helper()
	t.Setenv("REX_WRITE_ACCOUNT_ID", "2057")
	calls := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		*calls = append(*calls, r.URL.Path)
		var result any = "token"
		code := 200
		switch r.URL.Path {
		case "/v1/rex/authentication/login":
		case "/v1/rex/AccountSettings/read":
			result = map[string]any{"id": 2057, "name": "Sandbox", "is_test_account": true}
		default:
			result, code = handle(r.URL.Path, b)
		}
		w.WriteHeader(code)
		if e, ok := result.(rexTestError); ok { // error envelope, as Rex rejects a request
			_ = json.NewEncoder(w).Encode(map[string]any{"result": nil, "error": map[string]any{"message": string(e)}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	t.Cleanup(srv.Close)
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p", account: "2057"}
	w.client.AccountID = "2057"
	w.client.Interval = 0
	w.client.Retries = 0
	return w, calls
}
func TestRexWorkerContactUpdateAndEtag(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	workerQueue(t, a, "contacts", "c", `{"mobile":"0499999999"}`)
	w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/read":
			return map[string]any{"id": 91, "etag": "new", "related": map[string]any{"contact_names": []any{map[string]any{"id": 11}}, "contact_phones": []any{map[string]any{"id": 22}}}}, 200
		case "/v1/rex/contacts/update":
			d := b["data"].(map[string]any)
			related := d["related"].(map[string]any)
			phone := related["contact_phones"].([]any)[0].(map[string]any)
			if phone["id"] != float64(22) || phone["phone_number"] != "0499999999" || d["id"] != "91" {
				t.Errorf("update shape: %v", b)
			}
			return map[string]any{"id": 91, "etag": "new"}, 200
		default:
			t.Errorf("unexpected %s", path)
			return nil, 500
		}
	})
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 1) != "sent" {
		t.Fatal(workerStatus(t, a, 1))
	}
	var etag string
	_ = a.db.QueryRow(`SELECT etag FROM rex_links WHERE entity='contacts' AND local_id='c'`).Scan(&etag)
	if etag != "new" {
		t.Fatal(etag)
	}
	if len(*calls) != 4 {
		t.Fatal(*calls)
	}
}
func TestRexWorkerSkipsUnlinked(t *testing.T) {
	a := workerDB(t)
	_, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{"first":"Demo"}','now','now')`)
	if e != nil {
		t.Fatal(e)
	}
	workerQueue(t, a, "contacts", "c", `{}`)
	w, calls := sandboxWorker(t, a, func(string, map[string]any) (any, int) { t.Error("unexpected HTTP"); return nil, 500 })
	if e = w.step(context.Background()); e != nil {
		t.Fatal(e)
	}
	if workerStatus(t, a, 1) != "skipped" || len(*calls) != 0 {
		t.Fatal(workerStatus(t, a, 1), *calls)
	}
	var reason string
	_ = a.db.QueryRow(`SELECT last_error FROM rex_outbox WHERE id=1`).Scan(&reason)
	if reason != "not linked to Rex sandbox record" {
		t.Fatal(reason)
	}
}
func TestRexWorkerDisabledAnd429(t *testing.T) {
	a := workerDB(t)
	t.Setenv("AXLEO_REX_WRITE", "0")
	if newRexWorker(a.db) != nil {
		t.Fatal("worker enabled")
	}
	linkedContact(t, a)
	workerQueue(t, a, "contacts", "c", `{}`)
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) { return nil, 429 })
	_ = w.step(context.Background())
	if workerStatus(t, a, 1) != "pending" {
		t.Fatal(workerStatus(t, a, 1))
	}
}
