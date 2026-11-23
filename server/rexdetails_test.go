package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func rexTestWorker(t *testing.T, a *app, check func(string, map[string]any) any) (*rexWorker, *[]string) {
	t.Helper()
	calls := &[]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		*calls = append(*calls, r.URL.Path)
		result := any("token")
		if r.URL.Path != "/v1/rex/authentication/login" {
			result = check(r.URL.Path, body)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	t.Cleanup(srv.Close)
	worker := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p"}
	worker.client.Interval = 0
	worker.client.Retries = 0
	return worker, calls
}
func rexLink(t *testing.T, a *app, entity, local, id string) {
	t.Helper()
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at) VALUES(?,?,?,'Contacts','now')`, entity, local, id); err != nil {
		t.Fatal(err)
	}
}
func rexStep(t *testing.T, w *rexWorker, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRexDetailsNotes(t *testing.T) {
	a := workerDB(t)
	_, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{}','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	rexLink(t, a, "contacts", "c", "91")
	_, err = a.db.Exec(`INSERT INTO contact_notes(id,contact_id,user_id,t,created_at) VALUES('n','c','u','Hello','now')`)
	if err != nil {
		t.Fatal(err)
	}
	workerQueue(t, a, "contacts", "c", `{"id":"n","contactId":"c","t":"Hello"}`)
	w, calls := rexTestWorker(t, a, func(path string, b map[string]any) any {
		if path == "/v1/rex/notes/create" {
			d := b["data"].(map[string]any)
			if d["note"] != "Hello\n— via AxleoCollect · Mary" || d["note_type"].(map[string]any)["id"] != "note" || d["related"].(map[string]any)["note_contacts"].([]any)[0].(map[string]any)["contact"].(map[string]any)["id"] != "91" {
				t.Error(d)
			}
			return map[string]any{"id": 301}
		}
		return "token"
	})
	rexStep(t, w, 1)
	if w.link("contact_notes", "n") != "301" {
		t.Fatal(*calls)
	}
	_, _ = a.db.Exec(`UPDATE rex_outbox SET status='pending' WHERE id=1`)
	rexStep(t, w, 1)
	if len(*calls) != 2 {
		t.Fatal(*calls)
	}
}
func TestRexDetailsTagsRelationsPreferences(t *testing.T) {
	a := workerDB(t)
	rexLink(t, a, "contacts", "c", "91")
	rexLink(t, a, "contacts", "other", "92")
	rexLink(t, a, "properties", "house", "401")
	var propertyUpdate map[string]any
	var updates []map[string]any
	w, calls := rexTestWorker(t, a, func(path string, b map[string]any) any {
		if path == "/v1/rex/properties/update" {
			propertyUpdate = b["data"].(map[string]any)
			return map[string]any{"id": 401}
		}
		if path == "/v1/rex/contacts/update" {
			d := b["data"].(map[string]any)
			updates = append(updates, d)
			if d["id"] != "91" {
				t.Error(d)
			}
			if d["is_dnd"] != nil && len(d) != 2 {
				t.Errorf("DND mixed: %v", d)
			}
			return map[string]any{"id": 91}
		}
		return "token"
	})
	for _, x := range []struct{ entity, payload string }{
		{"rex_contact_tag", `{"contactId":"c","tag":"Left card","group":"DK"}`},
		{"rex_contact_tag", `{"contactId":"c","tag":"Hot","group":"OFI"}`},
		{"rex_contact_tag", `{"contactId":"c","tag":"Warm","group":"Enquiry"}`},
		{"rex_contact_relation", `{"contactId":"c","relation":{"contactId":"other","type":"Partner of"}}`},
		{"rex_contact_relation", `{"contactId":"c","relation":{"propertyId":"house","type":"Tenant of"}}`},
		{"rex_contact_letter", `{"contactId":"c","value":0}`},
		{"rex_contact_dnd", `{"contactId":"c","value":true}`},
	} {
		workerQueue(t, a, x.entity, "c", x.payload)
	}
	rexStep(t, w, 7)
	want := []string{"/v1/rex/authentication/login"}
	for i := 0; i < 7; i++ {
		path := "/v1/rex/contacts/update"
		if i == 4 {
			path = "/v1/rex/properties/update"
		}
		want = append(want, path)
	}
	if !reflect.DeepEqual(*calls, want) {
		t.Fatal(*calls)
	}
	for i, tag := range []string{"DK - Left card", "OFI - Hot", "Enquiry - Warm"} {
		got := updates[i]["related"].(map[string]any)["contact_tags"].([]any)[0].(map[string]any)["tag"]
		if got != tag {
			t.Fatalf("tag %d: %v", i, got)
		}
	}
	relation := updates[3]["related"].(map[string]any)["contact_reln_contact"].([]any)[0].(map[string]any)
	if relation["contact"].(map[string]any)["id"] != "92" || relation["reln_type"].(map[string]any)["id"] != "Partner of" {
		t.Fatal(relation)
	}
	if propertyUpdate["id"] != "401" || propertyUpdate["related"].(map[string]any)["contact_reln_property"].([]any)[0].(map[string]any)["reln_type"].(map[string]any)["id"] != "tenant" {
		t.Fatal(propertyUpdate)
	}
	if updates[4]["contact_by_letter"] != float64(0) || updates[5]["is_dnd"] != true {
		t.Fatal(updates)
	}
}
func TestRexDetailsMergeAndCompletion(t *testing.T) {
	a := workerDB(t)
	rexLink(t, a, "contacts", "a", "91")
	rexLink(t, a, "contacts", "b", "92")
	rexLink(t, a, "todos", "todo", "201")
	workerQueue(t, a, "rex_merge", "approval", `{"primaryId":"a","secondaryId":"b"}`)
	workerQueue(t, a, "rex_todo_completion", "todo", `{"todoId":"todo","done":true}`)
	workerQueue(t, a, "rex_completion_note", "todo", `{"contactId":"a","note":"Completed","actorId":"u"}`)
	w, calls := rexTestWorker(t, a, func(path string, b map[string]any) any {
		switch path {
		case "/v1/rex/dedupe/combine-records":
			if b["service_name"] != "Contacts" || b["winning_id"] != "91" || !reflect.DeepEqual(b["losing_ids"], []any{"92"}) {
				t.Error(b)
			}
		case "/v1/rex/reminders/toggle-completion-state":
			if b["id"] != "201" || b["is_completed"] != true || b["create_activity_entry"] != true {
				t.Error(b)
			}
		case "/v1/rex/notes/create":
			if !strings.Contains(b["data"].(map[string]any)["note"].(string), "via AxleoCollect · Mary") {
				t.Error(b)
			}
			return map[string]any{"id": 303}
		}
		return true
	})
	rexStep(t, w, 3)
	if len(*calls) != 4 || w.link("rex_completion_note", "todo") != "303" {
		t.Fatal(*calls)
	}
}
func TestRexDetailsEnqueueAndDefer(t *testing.T) {
	a := workerDB(t)
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	first := map[string]any{"tags": []any{"No junk mail"}, "flags": map[string]any{"dnc": true}, "rels": []any{map[string]any{"type": "Partner of", "contactId": "other"}}}
	if err = enqueueRexDetails(tx, "contacts", "c", first, now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox`).Scan(&n)
	if n != 4 {
		t.Fatal(n)
	}
	w, calls := rexTestWorker(t, a, func(string, map[string]any) any { return "token" })
	rexStep(t, w, 1)
	if len(*calls) != 0 || workerStatus(t, a, 1) != "pending" {
		t.Fatal(*calls)
	}
	var tries int
	_ = a.db.QueryRow(`SELECT tries FROM rex_outbox WHERE id=1`).Scan(&tries)
	if tries != 0 {
		t.Fatal(tries)
	}
	rexLink(t, a, "contacts", "c", "91")
	rexLink(t, a, "contacts", "other", "92")
	_, _ = a.db.Exec(`UPDATE rex_outbox SET next_attempt_at=NULL WHERE id=1`)
	rexStep(t, w, 1)
}
