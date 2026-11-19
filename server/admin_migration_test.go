package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestConsoleMigrationPreservesForeignKeysAndSession(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, e := open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if _, e = db.Exec(`CREATE TABLE schema_migrations(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL)`); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"001_init.sql", "002_sync.sql", "003_b3.sql", "004_market.sql", "005_rex_links.sql", "006_market_absentee.sql", "007_activity.sql", "008_photos.sql", "009_market_review.sql"} {
		b, e := assets.ReadFile("migrations/" + name)
		if e != nil {
			t.Fatal(e)
		}
		tx, e := db.Begin()
		if e != nil {
			t.Fatal(e)
		}
		if _, e = tx.Exec(string(b)); e == nil {
			_, e = tx.Exec(`INSERT INTO schema_migrations VALUES(?,?)`, name, now())
		}
		if e != nil {
			tx.Rollback()
			t.Fatalf("%s: %v", name, e)
		}
		if e = tx.Commit(); e != nil {
			t.Fatal(e)
		}
	}
	if e = createUser(db, "legacy@example.com", "Legacy", "agent", "password123"); e != nil {
		t.Fatal(e)
	}
	var id string
	_ = db.QueryRow(`SELECT id FROM users WHERE email='legacy@example.com'`).Scan(&id)
	a := &app{db: db, failures: map[string][]time.Time{}}
	w := request(a, "POST", "/api/v1/auth/login", "", map[string]any{"email": "legacy@example.com", "password": "password123", "device": "legacy"})
	if w.Code != 200 {
		t.Fatalf("legacy login %s", w.Body)
	}
	token := obj(t, w)["token"].(string)
	if _, e = db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('old-contact',?,'{}',?,?)`, id, now(), now()); e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec(`INSERT INTO todos(id,assignee_id,created_by,data,created_at,updated_at) VALUES('old-todo',?,?,'{}',?,?)`, id, id, now(), now()); e != nil {
		t.Fatal(e)
	}
	if e = migrate(db, path); e != nil {
		t.Fatal(e)
	}
	for table, want := range map[string]int{"users": 1, "sessions": 1, "contacts": 1, "todos": 1} {
		var n int
		if e = db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); e != nil || n != want {
			t.Fatalf("%s: %d %v", table, n, e)
		}
	}
	rows, e := db.Query(`PRAGMA foreign_key_check`)
	if e != nil {
		t.Fatal(e)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("foreign key violation after users rebuild")
	}
	w = request(a, "GET", "/api/v1/me", token, nil)
	if w.Code != 200 {
		t.Fatalf("session lost: %d %s", w.Code, w.Body)
	}
	if _, e = db.Exec(`UPDATE users SET role='leader' WHERE id=?`, id); e != nil {
		t.Fatalf("leader constraint: %v", e)
	}
}
