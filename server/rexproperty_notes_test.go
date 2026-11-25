package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

type queuedNote struct{ entityID, key, t, actor string }

func propertyNoteJobs(t *testing.T, a *app) []queuedNote {
	t.Helper()
	rows, err := a.db.Query(`SELECT entity_id,json_extract(payload,'$.noteKey'),json_extract(payload,'$.t'),json_extract(payload,'$.actorId') FROM rex_outbox WHERE entity='rex_property_note' ORDER BY id`)
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

func propertyVersion(t *testing.T, a *app, id string) int {
	t.Helper()
	var v int
	if err := a.db.QueryRow(`SELECT version FROM properties WHERE id=?`, id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func note(id, d, a, text string) map[string]any {
	n := map[string]any{"d": d, "at": d + "T01:00:00.000Z", "a": a, "t": text}
	if id != "" {
		n["id"] = id
	}
	return n
}

func TestPropertyNoteOutboxOnlyNewNotes(t *testing.T) {
	a, tokens := setup(t)
	mary := tokens["mary"]
	var maryID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&maryID)
	legacy := note("", "2026-10-01", "Mary", "Old note before ids")
	w := request(a, "POST", "/api/v1/properties", mary, map[string]any{"id": "p-n", "no": "1", "street": "Test St", "suburb": "Sunnybank", "ownerIds": []string{}, "flags": map[string]any{}, "notes": []any{legacy}})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	jobs := propertyNoteJobs(t, a)
	if len(jobs) != 1 || !strings.HasPrefix(jobs[0].key, "h") || len(jobs[0].key) != 41 || jobs[0].entityID != "p-n:"+jobs[0].key || jobs[0].actor != maryID {
		t.Fatalf("legacy note job %+v", jobs)
	}
	if jobs[0].key != propertyNoteKey(legacy) {
		t.Fatal("hash key not stable")
	}
	// A PATCH carrying the old notes plus one new note queues only the new one.
	notes := []any{note("n-2", "2026-11-25", "Mary", "Spoke to owner"), legacy}
	w = request(a, "PATCH", "/api/v1/properties/p-n", mary, map[string]any{"version": propertyVersion(t, a, "p-n"), "notes": notes})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	// The same notes again (and a PATCH without notes) queue nothing.
	for _, body := range []map[string]any{{"notes": notes}, {"pinned": true}} {
		body["version"] = propertyVersion(t, a, "p-n")
		if w = request(a, "PATCH", "/api/v1/properties/p-n", mary, body); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	jobs = propertyNoteJobs(t, a)
	if len(jobs) != 2 || jobs[1].entityID != "p-n:n-2" || jobs[1].t != "Spoke to owner" {
		t.Fatalf("jobs %+v", jobs)
	}
	// A note removed and then restored is not queued a second time.
	for _, list := range [][]any{{legacy}, notes} {
		if w = request(a, "PATCH", "/api/v1/properties/p-n", mary, map[string]any{"version": propertyVersion(t, a, "p-n"), "notes": list}); w.Code != 200 {
			t.Fatal(w.Code, w.Body)
		}
	}
	if jobs = propertyNoteJobs(t, a); len(jobs) != 2 {
		t.Fatalf("restored note re-queued: %+v", jobs)
	}
	// /sync push goes through the same mutate path.
	if err := syncPushOps(a, mary, map[string]any{"opId": "n3", "kind": "properties", "op": "upsert", "id": "p-n", "baseVersion": propertyVersion(t, a, "p-n"), "data": map[string]any{"notes": append([]any{note("n-3", "2026-11-25", "Mary", "Left card")}, notes...)}}); err != nil {
		t.Fatal(err)
	}
	if jobs = propertyNoteJobs(t, a); len(jobs) != 3 || jobs[2].entityID != "p-n:n-3" {
		t.Fatalf("sync push jobs %+v", jobs)
	}
}

func queueNoteJob(t *testing.T, a *app, key, d string) {
	t.Helper()
	payload := ijson(map[string]any{"propertyId": "p", "noteKey": key, "t": "Owner keen to sell", "a": "Phone Name", "d": d, "at": d + "T01:00:00Z", "actorId": "u"})
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_property_note',?,'POST',?,'now')`, "p:"+key, payload); err != nil {
		t.Fatal(err)
	}
}

func rexProperty(t *testing.T, a *app) {
	t.Helper()
	if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at) VALUES('p','16','Gager St','Sunnybank','{"id":"p"}','now','now')`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('properties','p','23138536','Properties','2057','now')`); err != nil {
		t.Fatal(err)
	}
}

func TestRexPropertyNoteSkippedWhenPropertyNotInRex(t *testing.T) {
	a := workerDB(t)
	if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at) VALUES('p','1','A St','B','{}','now','now')`); err != nil {
		t.Fatal(err)
	}
	queueNoteJob(t, a, "n1", "2026-11-25")
	w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) { return nil, 500 })
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	var status, reason string
	_ = a.db.QueryRow(`SELECT status,last_error FROM rex_outbox WHERE id=1`).Scan(&status, &reason)
	if status != "skipped" || reason != "not in Rex" || len(*calls) != 0 {
		t.Fatal(status, reason, *calls)
	}
}

func TestRexPropertyNoteCreate(t *testing.T) {
	a := workerDB(t)
	rexProperty(t, a)
	today := time.Now().In(brisbane).Format("2006-01-02")
	queueNoteJob(t, a, "old", "2026-01-02")
	queueNoteJob(t, a, "new", today)
	bodies := []map[string]any{}
	w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		if path != "/v1/rex/notes/create" {
			t.Errorf("unexpected %s", path)
			return nil, 500
		}
		bodies = append(bodies, b)
		return 141116060 + len(bodies), 200
	})
	before := propertyVersion(t, a, "p")
	for i := 0; i < 2; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	if workerStatus(t, a, 1) != "sent" || workerStatus(t, a, 2) != "sent" || len(bodies) != 2 {
		t.Fatal(workerStatus(t, a, 1), workerStatus(t, a, 2), *calls)
	}
	want := `{"data":{"note":"[2026-01-02] Owner keen to sell\n— via AxleoCollect · Mary","note_type":{"id":"note"},"related":{"note_properties":[{"property":{"id":"23138536"}}]}},"return_id":true}`
	if got := ijson(bodies[0]); got != want {
		t.Fatalf("create body\n got %s\nwant %s", got, want)
	}
	if note := bodies[1]["data"].(map[string]any)["note"]; note != "Owner keen to sell\n— via AxleoCollect · Mary" {
		t.Fatalf("today's note must not carry a date: %q", note)
	}
	var rexID, service string
	if err := a.db.QueryRow(`SELECT rex_id,rex_service FROM rex_links WHERE entity='rex_property_note' AND local_id='p:old' AND account_id='2057'`).Scan(&rexID, &service); err != nil || rexID != "141116061" || service != "Notes" {
		t.Fatal(err, rexID, service)
	}
	if propertyVersion(t, a, "p") != before+2 {
		t.Fatal("property not bumped for the phone's status pull")
	}
	// Re-queued (e.g. restored from backup): the link wins, Rex is not called again.
	n := len(*calls)
	queueNoteJob(t, a, "old", "2026-01-02")
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 3) != "sent" || len(*calls) != n {
		t.Fatal(workerStatus(t, a, 3), *calls)
	}
}

func TestRexPropertyNoteAmbiguousCreateNotReplayed(t *testing.T) {
	a := workerDB(t)
	rexProperty(t, a)
	queueNoteJob(t, a, "n1", "2026-11-25")
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

func TestPropertyNoteSyncStatusOnPull(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("REX_ACCOUNT_ID", "2057")
	mary := tokens["mary"]
	if err := syncPushOps(a, mary, map[string]any{"opId": "p", "kind": "properties", "op": "upsert", "id": "p-s", "data": map[string]any{"id": "p-s", "no": "1", "street": "Test St", "suburb": "Sunnybank", "ownerIds": []string{}, "flags": map[string]any{}, "notes": []any{note("n-1", "2026-11-25", "Mary", "Hi")}, "noteRexSync": map[string]any{"stale": map[string]any{"status": "sent"}}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='failed',last_error='Rex rejected: x' WHERE entity='rex_property_note'`); err != nil {
		t.Fatal(err)
	}
	w := request(a, "GET", "/api/v1/sync", mary, nil)
	var body struct {
		Properties []map[string]any `json:"properties"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	for _, p := range body.Properties {
		if p["id"] == "p-s" {
			got := fmt.Sprint(p["noteRexSync"])
			if !strings.Contains(got, "n-1:map[") || !strings.Contains(got, "status:failed") || strings.Contains(got, "stale") {
				t.Fatal(got)
			}
			return
		}
	}
	t.Fatal("property missing from pull", w.Body)
}
