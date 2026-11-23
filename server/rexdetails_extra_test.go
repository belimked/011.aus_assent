package main

import (
	"encoding/json"
	"testing"
)

func TestRexDetailsChangesAndDNDOrder(t *testing.T) {
	a := workerDB(t)
	prior := map[string]any{"id": "c", "tags": []any{"Left card"}, "flags": map[string]any{"dnc": true}, "consent": map[string]any{"letter": float64(0)}}
	current := map[string]any{"id": "c", "tags": []any{"Left card", "No junk mail"}, "flags": map[string]any{"dnc": false}, "consent": map[string]any{"letter": float64(0)}}
	b, _ := json.Marshal(prior)
	_, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at,status) VALUES('contacts','c','POST',?,'now','sent')`, string(b))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	b, _ = json.Marshal(current)
	_, err = tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts','c','PATCH',?,'now')`, string(b))
	if err != nil {
		t.Fatal(err)
	}
	if err = enqueueRexDetails(tx, "contacts", "c", current, now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	rows, err := a.db.Query(`SELECT entity FROM rex_outbox WHERE id>2 ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var s string
		if err = rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		got = append(got, s)
	}
	want := []string{"rex_contact_tag", "rex_contact_dnd", "rex_contact_letter"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}
func TestRexMergeApprovalQueuedWaitingAuth(t *testing.T) {
	a := workerDB(t)
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = enqueueRexMerge(tx, "approval-1", map[string]any{"primaryId": "a", "secondaryId": "b"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var entity, status, payload string
	if err = a.db.QueryRow(`SELECT entity,status,payload FROM rex_outbox WHERE entity_id='approval-1'`).Scan(&entity, &status, &payload); err != nil {
		t.Fatal(err)
	}
	if entity != "rex_merge" || status != "waiting_auth" || payload != `{"primaryId":"a","secondaryId":"b"}` {
		t.Fatal(entity, status, payload)
	}
}
func TestRexDetailsCompletionOnlyOnTransition(t *testing.T) {
	a := workerDB(t)
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	current := map[string]any{"done": true, "cid": "c", "completionNote": "Finished", "createdBy": "u"}
	if err = enqueueRexDetails(tx, "todos", "todo", current, now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity IN ('rex_todo_completion','rex_completion_note')`).Scan(&n)
	if n != 2 {
		t.Fatal(n)
	}
}
