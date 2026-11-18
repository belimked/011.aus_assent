package main

import "testing"

// Prototype: an agent reassigns a to-do or contact straight to a teammate, no approval.
func TestAgentDirectReassign(t *testing.T) {
	a, tok := setup(t)
	mary, john := tok["mary"], tok["john"]
	var johnID, maryID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&johnID)
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&maryID)

	// New to-do assigned straight to a teammate.
	created := obj(t, request(a, "POST", "/api/v1/todos", mary, map[string]any{"id": "dt-new", "t": "Call", "assigneeId": johnID}))
	if created["assigneeId"] != johnID || created["createdBy"] != maryID {
		t.Fatalf("create for teammate: %v", created)
	}
	if request(a, "GET", "/api/v1/todos/dt-new", john, nil).Code != 200 {
		t.Fatal("assignee cannot see new to-do")
	}

	// Existing to-do reassigned via PATCH.
	_ = request(a, "POST", "/api/v1/todos", mary, map[string]any{"id": "dt", "t": "Keys"})
	w := request(a, "PATCH", "/api/v1/todos/dt", mary, map[string]any{"version": 1, "assigneeId": johnID})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	got := obj(t, w)
	if got["assigneeId"] != johnID || got["createdBy"] != maryID {
		t.Fatalf("reassign: %v", got)
	}
	if request(a, "GET", "/api/v1/todos/dt", john, nil).Code != 200 {
		t.Fatal("new assignee cannot see to-do")
	}

	// Outsiders and unknown ids are ignored, not applied.
	_ = request(a, "POST", "/api/v1/todos", mary, map[string]any{"id": "dt2", "t": "X"})
	got = obj(t, request(a, "PATCH", "/api/v1/todos/dt2", mary, map[string]any{"version": 1, "assigneeId": "nobody"}))
	if got["assigneeId"] != maryID {
		t.Fatalf("unknown assignee applied: %v", got)
	}

	// Contact handed to a teammate.
	_ = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "dc", "first": "Move"})
	if w := request(a, "PATCH", "/api/v1/contacts/dc", mary, map[string]any{"version": 1, "ownerId": johnID}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if request(a, "GET", "/api/v1/contacts/dc", john, nil).Code != 200 {
		t.Fatal("new owner cannot see contact")
	}
	if request(a, "GET", "/api/v1/contacts/dc", mary, nil).Code != 404 {
		t.Fatal("previous owner still sees contact")
	}

	// Activity feed syncs per agent.
	if w := request(a, "POST", "/api/v1/activity", mary, map[string]any{"id": "act1", "d": "2026-11-07 10:00", "a": "To-do", "t": "x"}); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	if request(a, "GET", "/api/v1/activity/act1", john, nil).Code != 404 {
		t.Fatal("activity leaked to teammate")
	}
}
