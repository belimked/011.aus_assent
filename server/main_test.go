package main

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func setup(t *testing.T) (*app, map[string]string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for i := 0; i < 2; i++ {
		if err = migrate(db, path); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []struct{ email, name, role string }{{"mary@axleo.local", "Mary", "agent"}, {"john@axleo.local", "John", "agent"}, {"admin@axleo.local", "Admin", "admin"}} {
		if err = createUser(db, u.email, u.name, u.role, "password123"); err != nil {
			t.Fatal(err)
		}
	}
	a := &app{db: db, rdb: testReaders(t, path), failures: map[string][]time.Time{}}
	tokens := map[string]string{}
	for _, email := range []string{"mary@axleo.local", "john@axleo.local", "admin@axleo.local"} {
		w := request(a, "POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": "password123", "device": "test"})
		if w.Code != 200 {
			t.Fatalf("login: %s", w.Body)
		}
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		tokens[email[:4]] = m["token"].(string)
	}
	return a, tokens
}
func request(a *app, method, path, token string, data any) *httptest.ResponseRecorder {
	var b []byte
	if data != nil {
		b, _ = json.Marshal(data)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	return w
}
func obj(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return m
}
func TestAuthScopeCRUD(t *testing.T) {
	a, tok := setup(t)
	mary, john, admin := tok["mary"], tok["john"], tok["admi"]
	post := func(kind, token, id string, extra map[string]any) map[string]any {
		t.Helper()
		extra["id"] = id
		w := request(a, "POST", "/api/v1/"+kind, token, extra)
		if w.Code != 201 {
			t.Fatalf("post %s: %d %s", kind, w.Code, w.Body)
		}
		return obj(t, w)
	}
	post("contacts", mary, "mc", map[string]any{"first": "Mine"})
	post("contacts", john, "jc", map[string]any{"first": "John"})
	post("todos", john, "jt", map[string]any{"t": "John task"})
	post("properties", admin, "p", map[string]any{"ownerIds": []string{"mc", "jc"}})
	for _, kind := range []string{"contacts", "todos"} {
		id := "jc"
		if kind == "todos" {
			id = "jt"
		}
		for _, method := range []string{"GET", "PATCH", "DELETE"} {
			var payload any
			if method == "PATCH" {
				payload = map[string]any{"version": 1, "first": "bad"}
			}
			w := request(a, method, "/api/v1/"+kind+"/"+id, mary, payload)
			if w.Code != 404 {
				t.Fatalf("%s %s: %d", method, kind, w.Code)
			}
		}
	}
	w := request(a, "GET", "/api/v1/contacts", mary, nil)
	var list []any
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("mary contacts: %s", w.Body)
	}
	w = request(a, "GET", "/api/v1/contacts", admin, nil)
	_ = json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatal("admin missing contacts")
	}
	p := obj(t, request(a, "GET", "/api/v1/properties/p", mary, nil))
	if len(p["ownerIds"].([]any)) != 1 || p["ownerIds"].([]any)[0] != "mc" {
		t.Fatalf("owner leak: %v", p)
	}
	w = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "mc"})
	if w.Code != 200 {
		t.Fatal("duplicate post", w.Code)
	}
	w = request(a, "PATCH", "/api/v1/contacts/mc", mary, map[string]any{"version": 1, "first": "New"})
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	w = request(a, "PATCH", "/api/v1/contacts/mc", mary, map[string]any{"version": 1, "first": "Stale"})
	if w.Code != 409 || obj(t, w)["current"] == nil {
		t.Fatal("missing conflict current")
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity='contacts'`).Scan(&n)
	if n < 3 {
		t.Fatal("missing audit")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE status='waiting_auth'`).Scan(&n)
	if n < 4 {
		t.Fatal("missing outbox")
	}
	w = request(a, "POST", "/api/v1/admin/users", admin, map[string]any{"email": "new@local", "name": "New", "role": "agent", "password": "password123"})
	if w.Code != 201 {
		t.Fatal(w.Body)
	}
	id := obj(t, w)["id"].(string)
	var maryID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&maryID)
	w = request(a, "POST", "/api/v1/admin/users/"+maryID+"/revoke", admin, nil)
	if w.Code != 200 {
		t.Fatal(w.Body)
	}
	if request(a, "GET", "/api/v1/me", mary, nil).Code != 401 {
		t.Fatal("revoke failed")
	}
	_ = id
	if request(a, "POST", "/api/v1/auth/logout", john, nil).Code != 200 {
		t.Fatal("logout")
	}
	if request(a, "GET", "/api/v1/me", john, nil).Code != 401 {
		t.Fatal("logout failed")
	}
}
func TestNotesAndSince(t *testing.T) {
	a, tok := setup(t)
	mary := tok["mary"]
	w := request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "note-contact", "first": "Note"})
	if w.Code != 201 {
		t.Fatal(w.Body)
	}
	w = request(a, "POST", "/api/v1/contacts/note-contact/notes", mary, map[string]any{"d": "2026-11-07", "a": "Mary", "t": "hello"})
	if w.Code != 201 {
		t.Fatal(w.Body)
	}
	item := obj(t, request(a, "GET", "/api/v1/contacts/note-contact", mary, nil))
	if item["version"].(float64) != 2 || len(item["notes"].([]any)) != 1 {
		t.Fatalf("note missing: %v", item)
	}
	cursor := time.Now().Add(-time.Hour).UTC().Format(layout)
	if request(a, "DELETE", "/api/v1/contacts/note-contact", mary, nil).Code != 200 {
		t.Fatal("delete failed")
	}
	if request(a, "GET", "/api/v1/contacts/note-contact", mary, nil).Code != 404 {
		t.Fatal("deleted contact visible")
	}
	w = request(a, "GET", "/api/v1/contacts?since="+cursor, mary, nil)
	var list []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || len(list) != 1 || list[0]["deleted_at"] == nil {
		t.Fatalf("missing tombstone: %s", w.Body)
	}
}
func TestExpired(t *testing.T) {
	a, tok := setup(t)
	_, err := a.db.Exec(`UPDATE sessions SET expires_at=?`, time.Now().Add(-time.Hour).UTC().Format(layout))
	if err != nil {
		t.Fatal(err)
	}
	if request(a, "GET", "/api/v1/me", tok["mary"], nil).Code != 401 {
		t.Fatal("expired session accepted")
	}
}
