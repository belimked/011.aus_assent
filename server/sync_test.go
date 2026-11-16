package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	a, _ := setup(t)
	r := httptest.NewRequest("OPTIONS", "/api/v1/sync", nil)
	r.Header.Set("Origin", "http://localhost:8081")
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("preflight", w.Code, w.Header())
	}
}
func TestSyncPullAndPush(t *testing.T) {
	a, tok := setup(t)
	mary, john := tok["mary"], tok["john"]
	_ = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "mc", "first": "Mary"})
	_ = request(a, "POST", "/api/v1/contacts", john, map[string]any{"id": "jc", "first": "John"})
	_ = request(a, "POST", "/api/v1/properties", mary, map[string]any{"id": "registered"})
	_, err := a.db.Exec(`INSERT INTO properties(id,data,created_at,updated_at) VALUES('background','{"id":"background"}',?,?)`, now(), now())
	if err != nil {
		t.Fatal(err)
	}
	full := obj(t, request(a, "GET", "/api/v1/sync", mary, nil))
	contacts := full["contacts"].([]any)
	if len(contacts) != 1 || contacts[0].(map[string]any)["id"] != "mc" {
		t.Fatalf("scope: %v", full)
	}
	properties := full["properties"].([]any)
	if len(properties) != 1 {
		t.Fatalf("background leaked: %v", properties)
	}
	if request(a, "GET", "/api/v1/properties/background", mary, nil).Code != 200 {
		t.Fatal("background cannot be fetched")
	}
	cursor := full["cursor"].(string)
	push := func(op map[string]any) map[string]any {
		w := request(a, "POST", "/api/v1/sync/push", mary, map[string]any{"ops": []any{op}})
		if w.Code != 200 {
			t.Fatal(w.Body)
		}
		return obj(t, w)["results"].([]any)[0].(map[string]any)
	}
	op := map[string]any{"opId": "one", "kind": "contacts", "op": "upsert", "id": "mc", "baseVersion": 1, "data": map[string]any{"first": "New"}}
	first := push(op)
	if first["status"] != "ok" {
		t.Fatal(first)
	}
	second := push(op)
	b, _ := json.Marshal(first)
	c, _ := json.Marshal(second)
	if string(b) != string(c) {
		t.Fatal("not idempotent")
	}
	if push(map[string]any{"opId": "two", "kind": "contacts", "op": "upsert", "id": "mc", "baseVersion": 1, "data": map[string]any{"first": "Stale"}})["status"] != "conflict" {
		t.Fatal("expected conflict")
	}
	if push(map[string]any{"opId": "three", "kind": "contacts", "op": "delete", "id": "jc", "baseVersion": 1})["status"] != "notfound" {
		t.Fatal("scope escape")
	}
	increment := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, mary, nil))
	if len(increment["contacts"].([]any)) != 1 {
		t.Fatal(increment)
	}
	cursor = increment["cursor"].(string)
	if push(map[string]any{"opId": "four", "kind": "contacts", "op": "delete", "id": "mc", "baseVersion": 2})["status"] != "ok" {
		t.Fatal("delete")
	}
	deleted := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, mary, nil))["deleted"].(map[string]any)["contacts"].([]any)
	if len(deleted) != 1 || deleted[0] != "mc" {
		t.Fatal(deleted)
	}
	johnSync := obj(t, request(a, "GET", "/api/v1/sync", john, nil))
	if len(johnSync["contacts"].([]any)) != 1 {
		t.Fatal(johnSync)
	}
}

// Reassigned records must disappear from the previous owner's device.
func TestSyncReassignTombstone(t *testing.T) {
	a, tok := setup(t)
	mary, john, admin := tok["mary"], tok["john"], tok["admi"]
	_ = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "rc", "first": "Moved"})
	_ = request(a, "POST", "/api/v1/todos", mary, map[string]any{"id": "rt", "t": "Call"})
	cursor := obj(t, request(a, "GET", "/api/v1/sync", mary, nil))["cursor"].(string)
	var johnID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&johnID)
	if w := request(a, "PATCH", "/api/v1/contacts/rc", admin, map[string]any{"ownerId": johnID, "version": 1}); w.Code != 200 {
		t.Fatal("reassign contact", w.Code, w.Body)
	}
	if w := request(a, "PATCH", "/api/v1/todos/rt", admin, map[string]any{"assigneeId": johnID, "createdBy": johnID, "version": 1}); w.Code != 200 {
		t.Fatal("reassign todo", w.Code, w.Body)
	}
	inc := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, mary, nil))
	del := inc["deleted"].(map[string]any)
	has := func(list any, id string) bool {
		for _, x := range list.([]any) {
			if x == id {
				return true
			}
		}
		return false
	}
	if !has(del["contacts"], "rc") || !has(del["todos"], "rt") {
		t.Fatalf("no tombstone for previous owner: %v", del)
	}
	if len(inc["contacts"].([]any)) != 0 {
		t.Fatalf("reassigned contact still sent: %v", inc["contacts"])
	}
	if request(a, "GET", "/api/v1/contacts/rc", john, nil).Code != 200 {
		t.Fatal("new owner cannot see contact")
	}
}
