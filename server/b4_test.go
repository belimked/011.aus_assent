package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestAdminWebAndEndpoints(t *testing.T) {
	a, tokens := setup(t)
	index := request(a, "GET", "/admin/", "", nil).Body.String()
	assets := regexp.MustCompile(`(?:src|href)="(/admin/assets/[^"]+)"`).FindAllStringSubmatch(index, -1)
	if len(assets) == 0 {
		t.Fatal("missing built assets")
	}
	for _, asset := range assets {
		if w := request(a, "GET", asset[1], "", nil); w.Code != 200 {
			t.Fatalf("embedded asset %s: %d", asset[1], w.Code)
		}
	}
	if w := request(a, "GET", "/admin/parcels.json", "", nil); w.Code != 200 || !strings.Contains(w.Body.String(), "State of Queensland") {
		t.Fatal("missing public cadastral asset")
	}
	for _, url := range []string{"/admin/"} {
		w := request(a, "GET", url, "", nil)
		if w.Code != 200 || !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self'") || w.Header().Get("X-Frame-Options") != "DENY" {
			t.Fatalf("%s: %d %v", url, w.Code, w.Header())
		}
	}
	for _, url := range []string{"overview", "data?kind=contacts", "approvals", "audit", "rex-outbox", "users"} {
		for _, test := range []struct {
			token string
			code  int
		}{{"", 401}, {tokens["mary"], 404}} {
			w := request(a, "GET", "/api/v1/admin/"+url, test.token, nil)
			if w.Code != test.code {
				t.Fatalf("%s: %d %s", url, w.Code, w.Body)
			}
		}
	}
	admin := tokens["admi"]
	if request(a, "POST", "/api/v1/contacts", tokens["mary"], map[string]any{"id": "b4-contact", "first": "Alice", "last": "Smith"}).Code != 201 {
		t.Fatal("create contact")
	}
	for _, url := range []string{"overview", "data?kind=contacts", "audit?entity=contacts&action=POST", "rex-outbox?status=waiting_auth", "users", "approvals?status=pending"} {
		w := request(a, "GET", "/api/v1/admin/"+url, admin, nil)
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", url, w.Code, w.Body)
		}
	}
	if got := obj(t, request(a, "GET", "/api/v1/admin/overview", admin, nil))["counts"].(map[string]any)["contacts"]; got != float64(1) {
		t.Fatal(got)
	}
	if w := request(a, "GET", "/api/v1/admin/data?kind=users", admin, nil); w.Code != 400 {
		t.Fatal(w.Code)
	}
	var johnID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&johnID)
	w := request(a, "POST", "/api/v1/approvals", tokens["mary"], map[string]any{"kind": "reassign", "entity": "contacts", "id": "b4-contact", "targetId": johnID})
	if w.Code != 201 {
		t.Fatal(w.Body)
	}
	w = request(a, "GET", "/api/v1/admin/approvals?status=pending", admin, nil)
	if !strings.Contains(w.Body.String(), "Alice Smith") || !strings.Contains(w.Body.String(), "Mary") {
		t.Fatal(w.Body)
	}
}
func TestBackupVerifyRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "db.db")
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db, path); err != nil {
		t.Fatal(err)
	}
	if err = createUser(db, "first@local", "First", "admin", "password123"); err != nil {
		t.Fatal(err)
	}
	file, err := backup(db, filepath.Join(dir, "snapshots"))
	if err != nil {
		t.Fatal(err)
	}
	counts, err := verify(file)
	if err != nil || counts["users"] != 1 {
		t.Fatalf("%v %v", counts, err)
	}
	if err = createUser(db, "second@local", "Second", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	saved, err := restore(path, file)
	if err != nil {
		t.Fatal(err)
	}
	old, err := verify(saved)
	if err != nil || old["users"] != 2 {
		t.Fatalf("%v %v", old, err)
	}
	current, err := verify(path)
	if err != nil || current["users"] != 1 {
		t.Fatalf("%v %v", current, err)
	}
	// Higher schema versions must fail without replacing or backing up the target.
	future := filepath.Join(dir, "future.db")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(future, b, 0600); err != nil {
		t.Fatal(err)
	}
	fdb, err := open(future)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fdb.Exec(`INSERT INTO schema_migrations VALUES ('999_future.sql','2026-01-01')`)
	fdb.Close()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = restore(path, future); err == nil || !strings.Contains(err.Error(), "unsupported schema") {
		t.Fatal(err)
	}
	if _, err = verify(path); err != nil {
		t.Fatal(err)
	}
	lock, err := serviceLock(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	if _, err = restore(path, file); err == nil || !strings.Contains(err.Error(), "in use") {
		t.Fatal(err)
	}
}
