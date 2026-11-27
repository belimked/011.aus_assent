package main

import (
	"context"
	"strings"
	"testing"
)

func TestRexDetailsDNDIndependentUpdate(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	workerQueue(t, a, "rex_contact_dnd", "c", `{"contactId":"c","value":true}`)
	w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		if path != "/v1/rex/contacts/update" {
			t.Errorf("unexpected %s", path)
			return nil, 500
		}
		d := b["data"].(map[string]any)
		if len(d) != 2 || d["id"] != "91" || d["is_dnd"] != true {
			t.Errorf("DND mixed: %v", d)
		}
		return map[string]any{"id": 91}, 200
	})
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 1) != "sent" || len(*calls) != 3 {
		t.Fatal(workerStatus(t, a, 1), *calls)
	}
}
func TestRexDetailsUnlinkedJobsSkipped(t *testing.T) {
	a := workerDB(t)
	for _, entity := range []string{"rex_contact_tag", "rex_contact_relation", "rex_completion_note", "rex_merge", "rex_todo_completion"} {
		workerQueue(t, a, entity, "c", `{"contactId":"c"}`)
	}
	w, calls := sandboxWorker(t, a, func(string, map[string]any) (any, int) { t.Error("unlinked write"); return nil, 500 })
	for i := 0; i < 5; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
		if workerStatus(t, a, i+1) != "skipped" {
			t.Fatal(i)
		}
	}
	if len(*calls) != 0 {
		t.Fatal(*calls)
	}
}
func TestRexDetailsEnqueueAndSkip(t *testing.T) {
	a := workerDB(t)
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	current := map[string]any{"tags": []any{"No junk mail"}, "flags": map[string]any{"dnc": true}, "rels": []any{map[string]any{"type": "Partner of", "contactId": "other"}}}
	if err = enqueueRexDetails(tx, "contacts", "c", current, now()); err != nil {
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
	w, calls := sandboxWorker(t, a, func(string, map[string]any) (any, int) { t.Error("unlinked write"); return nil, 500 })
	for i := 0; i < n; i++ {
		if err = w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if len(*calls) != 0 {
		t.Fatal(*calls)
	}
	var skipped int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE status='skipped'`).Scan(&skipped)
	if skipped != n {
		t.Fatal(skipped)
	}
}

// UAT 3: tags go to Rex exactly as shown in the app; a legacy job's "group" no longer adds a "DK - " style prefix.
func TestRexDetailsTagWrittenVerbatim(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	t.Setenv("REX_TAG_PRESET", "0") // AdminTags preset: rextagpreset_test.go
	workerQueue(t, a, "rex_contact_tag", "c:Hot", `{"contactId":"c","tag":"Hot","group":"Enquiry"}`)
	workerQueue(t, a, "rex_contact_tag", "c:Not at home", `{"contactId":"c","tag":"Not at home","group":"DK"}`)
	workerQueue(t, a, "rex_contact_tag", "c:Pool", `{"contactId":"c","tag":"Pool"}`)
	var sent []string
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/update":
			d := b["data"].(map[string]any)
			for _, r := range d["related"].(map[string]any)["contact_tags"].([]any) {
				sent = append(sent, r.(map[string]any)["tag"].(string))
			}
			return map[string]any{"id": 91}, 200
		case "/v1/rex/contacts/read":
			return map[string]any{"id": 91, "etag": "new"}, 200
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	})
	for i := 1; i <= 3; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
		if workerStatus(t, a, i) != "sent" {
			t.Fatal(i, workerStatus(t, a, i))
		}
	}
	if strings.Join(sent, "|") != "Hot|Not at home|Pool" {
		t.Fatal(sent)
	}
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = enqueueRexDetails(tx, "contacts", "c", map[string]any{"source": "dk", "tags": []any{"Friendly"}}, now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var payload string
	_ = a.db.QueryRow(`SELECT payload FROM rex_outbox WHERE entity='rex_contact_tag' AND entity_id='c:Friendly'`).Scan(&payload)
	if payload != `{"contactId":"c","tag":"Friendly"}` {
		t.Fatal(payload)
	}
}
