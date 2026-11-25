package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The pool has one connection. Soft-delete reconciliation used to run a nested
// QueryRow while iterating rex_links, which hung the whole server whenever a
// linked contact was missing from the Rex contact search.
func TestPollSoftDeleteDoesNotDeadlockOrDropNewLinks(t *testing.T) {
	a, _ := setup(t)
	if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	var mary string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	past, future := time.Now().UTC().Add(-time.Hour).Format(layout), time.Now().UTC().Add(time.Hour).Format(layout)
	for _, c := range []struct{ id, rex, linked string }{{"old-contact", "901", past}, {"new-contact", "902", future}} {
		if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,first,data,created_at,updated_at) VALUES(?,?,?,?,?,?)`, c.id, mary, c.id, `{"source":"app"}`, past, past); err != nil {
			t.Fatal(err)
		}
		if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('contacts',?,?,'Contacts',?,'2057')`, c.id, c.rex, c.linked); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("REX_POLL_SOFT_DELETE", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"rows": []any{}, "total": 0}, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token = "mock"
	c.Interval = 0
	c.ReadOnly = true
	c.AccountID = "2057"
	lastRelationPoll = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- pollRex(ctx, a.db, c, "2057") }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("pollRex deadlocked on the single database connection")
	}
	deleted := map[string]bool{}
	rows, err := a.db.Query(`SELECT id, deleted_at IS NOT NULL FROM contacts WHERE id IN ('old-contact','new-contact')`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		var d bool
		if err := rows.Scan(&id, &d); err != nil {
			t.Fatal(err)
		}
		deleted[id] = d
	}
	rows.Close()
	if !deleted["old-contact"] {
		t.Fatal("contact missing from Rex before the poll should be soft-deleted")
	}
	if deleted["new-contact"] {
		t.Fatal("contact linked after the search started must not be soft-deleted")
	}
}
