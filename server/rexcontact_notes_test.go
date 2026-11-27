package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func contactNoteJobs(t *testing.T, a *app) []queuedNote {
	t.Helper()
	rows, err := a.db.Query(`SELECT entity_id,json_extract(payload,'$.noteKey'),json_extract(payload,'$.t'),json_extract(payload,'$.actorId') FROM rex_outbox WHERE entity='rex_contact_note' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []queuedNote{}
	for rows.Next() {
		var q queuedNote
		if err := rows.Scan(&q.entityID, &q.key, &q.t, &q.actor); err != nil {
			t.Fatal(err)
		}
		out = append(out, q)
	}
	return out
}

func contactVersion(t *testing.T, a *app, id string) int {
	t.Helper()
	var v int
	if err := a.db.QueryRow(`SELECT version FROM contacts WHERE id=?`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// Notes already stored when this ships are never sent; only notes added by a later
// write are, once each, however often the write is repeated or replayed.
func TestContactNoteOutboxOnlyNewNotes(t *testing.T) {
	a, tokens := setup(t)
	mary := tokens["mary"]
	var maryID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&maryID)
	existing := map[string]any{"d": "2026-11-26", "a": "Mary", "t": "Teddy owns a lot of homes in Brisbane, treated as VIP."}
	mergeContactRow(t, a, "c-n", maryID, now(), now(), map[string]any{"first": "Teddie", "last": "Bear", "notes": []any{existing}})
	added := note("cn-2", "2026-11-27", "Mary", "Called, wants appraisal")
	added["propId"] = "p-9"
	notes := []any{added, existing}
	w := request(a, "PATCH", "/api/v1/contacts/c-n", mary, map[string]any{"version": contactVersion(t, a, "c-n"), "notes": notes})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	jobs := contactNoteJobs(t, a)
	if len(jobs) != 1 || jobs[0].entityID != "c-n:cn-2" || jobs[0].t != "Called, wants appraisal" || jobs[0].actor != maryID {
		t.Fatalf("jobs %+v", jobs)
	}
	var pid string
	_ = a.db.QueryRow(`SELECT json_extract(payload,'$.propertyId') FROM rex_outbox WHERE entity='rex_contact_note'`).Scan(&pid)
	if pid != "p-9" {
		t.Fatal("property of the note not queued:", pid)
	}
	// Same notes again, a PATCH without notes, removal then restore: nothing new.
	for _, body := range []map[string]any{{"notes": notes}, {"email": "t@x.com"}, {"notes": []any{existing}}, {"notes": notes}} {
		body["version"] = contactVersion(t, a, "c-n")
		if w = request(a, "PATCH", "/api/v1/contacts/c-n", mary, body); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if jobs = contactNoteJobs(t, a); len(jobs) != 1 {
		t.Fatalf("re-queued: %+v", jobs)
	}
	// App activity notes stay in the app; an id-less note from an old app gets a hash key.
	todo := map[string]any{"d": "2026-11-27", "a": "AxleoCollect", "t": "To-do: call · 2026-11-28 16:00 · me"}
	market := map[string]any{"id": "cn-m", "d": "2026-11-27", "a": "Mary", "t": "Market monitor: sold", "sys": "market"}
	legacy := map[string]any{"d": "2026-11-27", "a": "Mary", "t": "Old app note"}
	// /sync push goes through the same mutate path, and an offline replay of the same op adds nothing.
	op := map[string]any{"opId": "cn3", "kind": "contacts", "op": "upsert", "id": "c-n", "baseVersion": contactVersion(t, a, "c-n"), "data": map[string]any{"notes": append([]any{todo, market, legacy}, notes...)}}
	for i := 0; i < 2; i++ {
		if err := syncPushOps(a, mary, op); err != nil {
			t.Fatal(err)
		}
	}
	jobs = contactNoteJobs(t, a)
	if len(jobs) != 2 || jobs[1].key != propertyNoteKey(legacy) || jobs[1].entityID != "c-n:"+jobs[1].key {
		t.Fatalf("sync push jobs %+v", jobs)
	}
	// A new contact's first notes are new.
	if w = request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "c-new", "first": "New", "notes": []any{note("cn-9", "2026-11-27", "Mary", "Met at door")}}); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	if jobs = contactNoteJobs(t, a); len(jobs) != 3 || jobs[2].entityID != "c-new:cn-9" {
		t.Fatalf("new contact jobs %+v", jobs)
	}
}

// The legacy endpoint queues the same job keyed by its contact_notes id; the phone's
// next PATCH carrying that note does not queue it again.
func TestContactNoteEndpointSinglePath(t *testing.T) {
	a, tokens := setup(t)
	mary := tokens["mary"]
	if w := request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "c-e", "first": "E"}); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	w := request(a, "POST", "/api/v1/contacts/c-e/notes", mary, map[string]any{"d": "2026-11-27", "a": "Mary", "t": "hello"})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	nid := obj(t, w)["id"].(string)
	jobs := contactNoteJobs(t, a)
	if len(jobs) != 1 || jobs[0].entityID != "c-e:"+nid {
		t.Fatalf("jobs %+v", jobs)
	}
	var legacy int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='contacts' AND json_extract(payload,'$.t') IS NOT NULL`).Scan(&legacy)
	if legacy != 0 {
		t.Fatal("legacy contact note job queued")
	}
	item := obj(t, request(a, "GET", "/api/v1/contacts/c-e", mary, nil))
	notes := item["notes"].([]any)
	if len(notes) != 1 || notes[0].(map[string]any)["id"] != nid {
		t.Fatalf("notes %v", notes)
	}
	if w = request(a, "PATCH", "/api/v1/contacts/c-e", mary, map[string]any{"version": item["version"], "notes": notes, "email": "e@x.com"}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	if jobs = contactNoteJobs(t, a); len(jobs) != 1 {
		t.Fatalf("jobs %+v", jobs)
	}
}

func queueContactNoteJob(t *testing.T, a *app, contactID, key, d, propertyID string) {
	t.Helper()
	payload := ijson(map[string]any{"contactId": contactID, "noteKey": key, "t": "Teddy owns a lot of homes in Brisbane, treated as VIP.", "a": "Phone Name", "d": d, "at": d + "T01:00:00Z", "propertyId": propertyID, "actorId": "u"})
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_contact_note',?,'POST',?,'now')`, contactID+":"+key, payload); err != nil {
		t.Fatal(err)
	}
}

func TestRexContactNoteCreate(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a) // c -> Rex 91
	rexProperty(t, a)   // p -> Rex 23138536
	// An app-created contact the worker created in Rex gets its notes too.
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('app','u','{"first":"App"}','now','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','app','93','Contacts','2057','now')`); err != nil {
		t.Fatal(err)
	}
	today := time.Now().In(brisbane).Format("2006-01-02")
	queueContactNoteJob(t, a, "c", "old", "2026-01-02", "p")
	queueContactNoteJob(t, a, "c", "new", today, "p-not-in-rex")
	queueContactNoteJob(t, a, "app", "n1", today, "")
	bodies := []map[string]any{}
	w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		if path != "/v1/rex/notes/create" {
			t.Errorf("unexpected %s", path)
			return nil, 500
		}
		bodies = append(bodies, b)
		return 141116060 + len(bodies), 200
	})
	before := contactVersion(t, a, "c")
	for i := 0; i < 3; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for id := 1; id <= 3; id++ {
		if workerStatus(t, a, id) != "sent" {
			t.Fatal(id, workerStatus(t, a, id), *calls)
		}
	}
	want := `{"data":{"note":"[2026-01-02] Teddy owns a lot of homes in Brisbane, treated as VIP.\n— via AxleoCollect · Mary","note_type":{"id":"note"},"related":{"note_contacts":[{"contact":{"id":"91"}}],"note_properties":[{"property":{"id":"23138536"}}]}},"return_id":true}`
	if got := ijson(bodies[0]); got != want {
		t.Fatalf("create body\n got %s\nwant %s", got, want)
	}
	want = `{"data":{"note":"Teddy owns a lot of homes in Brisbane, treated as VIP.\n— via AxleoCollect · Mary","note_type":{"id":"note"},"related":{"note_contacts":[{"contact":{"id":"91"}}]}},"return_id":true}`
	if got := ijson(bodies[1]); got != want { // today's note: no date; property not in Rex: contact only
		t.Fatalf("create body\n got %s\nwant %s", got, want)
	}
	if !strings.Contains(ijson(bodies[2]), `"note_contacts":[{"contact":{"id":"93"}}]`) {
		t.Fatal(ijson(bodies[2]))
	}
	var rexID, service string
	if err := a.db.QueryRow(`SELECT rex_id,rex_service FROM rex_links WHERE entity='rex_contact_note' AND local_id='c:old' AND account_id='2057'`).Scan(&rexID, &service); err != nil || rexID != "141116061" || service != "Notes" {
		t.Fatal(err, rexID, service)
	}
	if contactVersion(t, a, "c") != before+2 {
		t.Fatal("contact not bumped for the phone's status pull")
	}
	// Queued again: the link wins, Rex is not called again.
	n := len(*calls)
	queueContactNoteJob(t, a, "c", "old", "2026-01-02", "p")
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 4) != "sent" || len(*calls) != n {
		t.Fatal(workerStatus(t, a, 4), *calls)
	}
}

func TestRexContactNoteAmbiguousCreateNotReplayed(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	queueContactNoteJob(t, a, "c", "n1", "2026-11-25", "")
	creates := 0
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		creates++
		return nil, 502
	})
	for i := 0; i < 2; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	var status, reason string
	_ = a.db.QueryRow(`SELECT status,last_error FROM rex_outbox WHERE id=1`).Scan(&status, &reason)
	if status != "failed" || !strings.Contains(reason, "reconcile") || creates != 1 {
		t.Fatal(status, reason, creates)
	}
}

// A contact not (yet) in Rex: same outcome as tags and relations.
func TestRexContactNoteUnlinkedContact(t *testing.T) {
	for _, tc := range []struct{ create, merge, status, reason string }{
		{"pending", "", "pending", "waiting for contact link"},
		{"failed", "", "failed", "contact creation failed; reconcile before retry"},
		{"skipped", "", "skipped", "contact not in Rex"}, // e.g. possible duplicate of a linked contact
		{"", "", "skipped", "contact not in Rex"},
		{"skipped", "waiting_auth", "pending", "waiting for contact merge"},
		{"gone", "", "skipped", "not linked to Rex sandbox record"},
	} {
		t.Run(tc.create+tc.merge, func(t *testing.T) {
			a := workerDB(t)
			if tc.create != "gone" {
				if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{"first":"New"}','now','now')`); err != nil {
					t.Fatal(err)
				}
			}
			queueContactNoteJob(t, a, "c", "n1", "2026-11-25", "")
			if tc.create != "" && tc.create != "gone" {
				if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at) VALUES('contacts','c','POST','{}',?,'now')`, tc.create); err != nil {
					t.Fatal(err)
				}
			}
			if tc.merge != "" {
				if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at) VALUES('rex_merge','merge:c:b','POST','{"primaryId":"c","secondaryId":"b"}',?,'now')`, tc.merge); err != nil {
					t.Fatal(err)
				}
			}
			w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) { return nil, 500 })
			if err := w.step(context.Background()); err != nil {
				t.Fatal(err)
			}
			var status, reason string
			_ = a.db.QueryRow(`SELECT status,COALESCE(last_error,'') FROM rex_outbox WHERE id=1`).Scan(&status, &reason)
			if status != tc.status || reason != tc.reason || len(*calls) != 0 {
				t.Fatal(status, reason, *calls)
			}
		})
	}
}

// A note of the secondary still queued at merge time goes once to the primary's Rex
// record; the merged notes keep their ids, so the next PATCH queues nothing.
func TestRexMergeQueuedContactNoteGoesToPrimary(t *testing.T) {
	a, f := rexMergeFixSetup(t)
	late := note("cn-late", "2026-02-03", "Mary", "late app note on b")
	if _, err := a.db.Exec(`UPDATE contacts SET data=json_set(data,'$.notes',json(?)) WHERE id='b'`, ijson([]any{late})); err != nil {
		t.Fatal(err)
	}
	queueContactNoteJob(t, a, "b", "cn-late", "2026-02-03", "")
	runMerge(t, a)
	var entityID, contactID string
	_ = a.db.QueryRow(`SELECT entity_id,json_extract(payload,'$.contactId') FROM rex_outbox WHERE entity='rex_contact_note'`).Scan(&entityID, &contactID)
	if entityID != "a:cn-late" || contactID != "a" {
		t.Fatal(entityID, contactID)
	}
	var raw string
	_ = a.db.QueryRow(`SELECT data FROM contacts WHERE id='a'`).Scan(&raw)
	var data map[string]any
	_ = json.Unmarshal([]byte(raw), &data)
	if !strings.Contains(ijson(data["notes"]), `"id":"cn-late"`) {
		t.Fatal("merged note lost its id:", ijson(data["notes"]))
	}
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	err = enqueueContactNotes(tx, "a", data, map[string]any{"notes": copyPropertyNotes(data["notes"])}, "u", now())
	_ = tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	drain(t, a, mergeRexWorker(t, a, f), 12)
	count := 0
	for _, n := range f.bodies["/v1/rex/notes/create"] {
		if s := ijson(n); strings.Contains(s, "late app note on b") || strings.Contains(s, "Teddy owns") {
			count++
			if !strings.Contains(s, `"note_contacts":[{"contact":{"id":"91"}}]`) {
				t.Fatalf("note not on primary: %s", s)
			}
		}
	}
	if count != 1 || len(contactNoteJobs(t, a)) != 1 {
		t.Fatalf("notes %v jobs %v", f.bodies["/v1/rex/notes/create"], contactNoteJobs(t, a))
	}
	var linked int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_links WHERE entity='rex_contact_note' AND local_id='a:cn-late'`).Scan(&linked)
	if linked != 1 {
		t.Fatal("note link not on the primary key")
	}
}

func TestMoveContactNoteJobsSameKeyOnPrimary(t *testing.T) {
	a := workerDB(t)
	queueContactNoteJob(t, a, "a", "k", "2026-11-25", "")
	queueContactNoteJob(t, a, "b", "k", "2026-11-25", "")
	queueContactNoteJob(t, a, "b", "other", "2026-11-25", "")
	queueContactNoteJob(t, a, "bb", "k", "2026-11-25", "") // prefix of another id: untouched
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = moveContactNoteJobs(tx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	rows, _ := a.db.Query(`SELECT entity_id,status FROM rex_outbox ORDER BY id`)
	for rows.Next() {
		var id, status string
		_ = rows.Scan(&id, &status)
		got = append(got, id+"|"+status)
	}
	rows.Close()
	if fmt.Sprint(got) != "[a:k|waiting_auth a:k|skipped a:other|waiting_auth bb:k|waiting_auth]" {
		t.Fatal(got)
	}
}

func TestContactNoteSyncStatusOnPull(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("REX_ACCOUNT_ID", "2057")
	mary := tokens["mary"]
	singleConnGuard(t, a.db, "contact notes", func() error {
		if err := syncPushOps(a, mary, map[string]any{"opId": "c", "kind": "contacts", "op": "upsert", "id": "c-s", "data": map[string]any{"id": "c-s", "first": "Teddie", "last": "Bear", "notes": []any{note("cn-1", "2026-11-25", "Mary", "Hi")}, "noteRexSync": map[string]any{"stale": map[string]any{"status": "sent"}}}}); err != nil {
			return err
		}
		if _, err := a.db.Exec(`UPDATE rex_outbox SET status='failed',last_error='Rex rejected: x' WHERE entity='rex_contact_note'`); err != nil {
			return err
		}
		w := request(a, "GET", "/api/v1/sync", mary, nil)
		var body struct {
			Contacts []map[string]any `json:"contacts"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &body)
		for _, c := range body.Contacts {
			if c["id"] == "c-s" {
				got := fmt.Sprint(c["noteRexSync"])
				if !strings.Contains(got, "cn-1:map[") || !strings.Contains(got, "status:failed") || strings.Contains(got, "stale") {
					return fmt.Errorf("noteRexSync %s", got)
				}
				return nil
			}
		}
		return fmt.Errorf("contact missing from pull %s", w.Body)
	})
}
