package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Integration of contact search / same-mobile dedupe (rexdedupe.go) with contact
// merge (contact_merge.go).

// (a) A merge clears the possibleDuplicateOf flag between the merged pair only.
func TestMergeClearsPossibleDuplicatePair(t *testing.T) {
	a := rexMergeSetup(t, "", "")
	mergeContactRow(t, a, "c", "u", t1, t1, map[string]any{"first": "Third", "mobile": "0400000003"})
	mergeContactRow(t, a, "d", "u", t1, t1, map[string]any{"first": "Fourth", "mobile": "0400000004"})
	for _, pair := range [][]string{{"a", "b"}, {"a", "c"}, {"d", "b"}} {
		if err := markPossibleDuplicate(a.db, pair...); err != nil {
			t.Fatal(err)
		}
	}
	plan := runMerge(t, a)
	if plan.PrimaryID != "a" || plan.SecondaryID != "b" {
		t.Fatal(plan.PrimaryID, plan.SecondaryID)
	}
	// The primary keeps its flag to c, and inherits b's flag to d; never a or b.
	if got := ijson(dupOf(t, a, "a")); got != `["c","d"]` {
		t.Fatalf("primary flags %s", got)
	}
	if got := dupOf(t, a, "b"); len(got) != 0 {
		t.Fatalf("secondary flags %v", got)
	}
	if got := ijson(dupOf(t, a, "c")); got != `["a"]` {
		t.Fatalf("third contact flags %s", got)
	}
	// d flagged b: b now lives in a, so the flag follows it.
	if got := ijson(dupOf(t, a, "d")); got != `["a"]` {
		t.Fatalf("flag to the merged-away record %s", got)
	}
	// The phone receives d's change.
	var v int
	_ = a.db.QueryRow(`SELECT version FROM contacts WHERE id='d'`).Scan(&v)
	if v < 2 {
		t.Fatal("d not bumped")
	}
}

// (a) Only the pair: a merge of a flagged pair with no other flags removes the key.
func TestMergeRemovesFlagKeyWhenOnlyPair(t *testing.T) {
	a := rexMergeSetup(t, "", "")
	if err := markPossibleDuplicate(a.db, "a", "b"); err != nil {
		t.Fatal(err)
	}
	runMerge(t, a)
	if _, ok := contactData(t, a, "a")["possibleDuplicateOf"]; ok {
		t.Fatal("flag key left on the primary")
	}
	if _, ok := contactData(t, a, "b")["possibleDuplicateOf"]; ok {
		t.Fatal("flag key left on the secondary")
	}
}

func pollUsers(t *testing.T, a *app) {
	t.Helper()
	for _, n := range []string{"mary", "john", "import", "admin"} {
		if err := createUser(a.db, n+"@axleo.local", n, "agent", "password123"); err != nil {
			t.Fatal(err)
		}
	}
}

// pollOnce runs pollRex against a mock that lists one contact (rid, etag) and
// reads it as detail.
func pollOnce(t *testing.T, a *app, rid, etag string, detail importRow) {
	t.Helper()
	t.Setenv("REX_POLL_SOFT_DELETE", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any = map[string]any{"rows": []any{}, "total": 0}
		switch r.URL.Path {
		case "/v1/rex/contacts/search":
			result = map[string]any{"rows": []any{map[string]any{"_id": rid, "etag": etag}}, "total": 1}
		case "/v1/rex/contacts/read":
			result = detail
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly, c.AccountID, c.Log = "mock", 0, true, "2057", rexDBLogger(a.db)
	lastRelationPoll = time.Now()
	appConnGuard(t, a, "poll", func() error { return pollRex(context.Background(), a.db, c, "2057") })
}

func aliveContacts(t *testing.T, a *app) []string {
	t.Helper()
	rows, err := a.db.Query(`SELECT id FROM contacts WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		out = append(out, id)
	}
	return out
}

// (b) The secondary's Rex link moves to the primary (rex_merge adopt); the poller
// then updates the primary for that rex_id and never creates a new contact.
func TestPollerAfterMergeLinkTransferUpdatesPrimary(t *testing.T) {
	a := rexMergeSetup(t, "", "")
	pollUsers(t, a)
	// b's Rex create is in flight at merge time, so a (older) is the primary.
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at,next_attempt_at) VALUES('contacts','b','POST','{}','pending',?,?)`, now(), "2999-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	plan := runMerge(t, a)
	if plan.PrimaryID != "a" || plan.Rex != "pending" {
		t.Fatal(plan.PrimaryID, plan.Rex)
	}
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at,etag) VALUES('contacts','b','93','Contacts','2057',?,'e0')`, now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='sent' WHERE entity='contacts' AND entity_id='b'`); err != nil {
		t.Fatal(err)
	}
	// Before rex_merge runs, the link still names the merged-away b: the poller
	// must neither undelete b nor import a second record.
	pollOnce(t, a, "93", "e1", rexDetail("93", "0400000002", time.Now()))
	if got := ijson(aliveContacts(t, a)); got != `["a"]` {
		t.Fatalf("poll before transfer: %s", got)
	}
	w, _ := mergeWorker(t, a, []any{map[string]any{"id": 900, "contact": map[string]any{"id": 93}, "reln_type": map[string]any{"id": "owner"}}})
	drain(t, a, w, 8)
	if linkOf(a, "a") != "93" || linkOf(a, "b") != "" {
		t.Fatalf("link not transferred: a=%s b=%s", linkOf(a, "a"), linkOf(a, "b"))
	}
	var pending int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE status IN ('pending','waiting_auth')`).Scan(&pending)
	if pending != 0 {
		t.Fatalf("jobs left %v", outboxJobs(t, a))
	}
	// Rex changes the (now primary's) record: the poller updates a.
	detail := rexDetail("93", "0400000002", time.Now())
	detail["etag"] = "e9"
	pollOnce(t, a, "93", "e9", detail)
	if got := ijson(aliveContacts(t, a)); got != `["a"]` {
		t.Fatalf("poll after transfer created or undeleted: %s", got)
	}
	var first, etag string
	_ = a.db.QueryRow(`SELECT first FROM contacts WHERE id='a'`).Scan(&first)
	_ = a.db.QueryRow(`SELECT etag FROM rex_links WHERE entity='contacts' AND rex_id='93'`).Scan(&etag)
	if first != "Rex" || etag != "e9" {
		t.Fatalf("primary not updated from Rex: first=%s etag=%s", first, etag)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts WHERE id LIKE 'rex-contacts-%'`).Scan(&n)
	if n != 0 {
		t.Fatal("poller imported a new contact")
	}
}

// (b) After the merge, a new app contact with the same mobile is matched to the
// Rex record now linked to the primary and flagged against the primary, not the
// merged-away secondary; it is never linked a second time.
func TestWorkerPreCreateAfterMergeFlagsPrimary(t *testing.T) {
	a := rexMergeSetup(t, "", "")
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at,next_attempt_at) VALUES('contacts','b','POST','{}','pending',?,?)`, now(), "2999-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	runMerge(t, a)
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','b','93','Contacts','2057',?)`, now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='sent' WHERE entity='contacts' AND entity_id='b'`); err != nil {
		t.Fatal(err)
	}
	w, _ := mergeWorker(t, a, []any{})
	drain(t, a, w, 8)
	if linkOf(a, "a") != "93" {
		t.Fatal("link not transferred")
	}
	// A new app contact with the primary's resolved mobile (the one Rex 93 holds).
	mobile := str(contactData(t, a, "a"), "mobile")
	mergeContactRow(t, a, "e", "u", t4, t4, map[string]any{"first": "New", "mobile": mobile})
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts','e','POST','{}',?)`, now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='pending' WHERE entity='contacts' AND entity_id='e'`); err != nil {
		t.Fatal(err)
	}
	drain(t, a, w, 2)
	if linkOf(a, "e") != "" {
		t.Fatal("second local contact linked to 93")
	}
	if got := ijson(dupOf(t, a, "e")); got != `["a"]` {
		t.Fatalf("flags %s", got)
	}
}

// (d) A contact search returns another agent's contact in the restricted view;
// that id cannot be previewed or merged by the searching agent.
func TestSearchRestrictedContactCannotBeMerged(t *testing.T) {
	a, tok := setup(t)
	disableSearchRex(a)
	searchContact(t, a, tok["mary"], "m-own", "Rita", "Own", "0401 555 111", "")
	searchContact(t, a, tok["john"], "j-other", "Rita", "Other", "0401 555 222", "")
	res, _ := doSearch(t, a, tok["mary"], "Rita")
	other := itemByID(res.Items, "id", "j-other")
	if other == nil {
		t.Fatalf("restricted item missing: %v", res.Items)
	}
	if other["full"] != false {
		t.Fatalf("another agent's contact in full view: %v", other)
	}
	for _, path := range []string{"/api/v1/contacts/merge-preview", "/api/v1/contacts/m-own/merge"} {
		body := map[string]any{"aId": "m-own", "bId": "j-other", "secondaryId": "j-other", "previewHash": "x"}
		if w := request(a, "POST", path, tok["mary"], body); w.Code != 404 {
			t.Fatalf("%s with a restricted contact: %d %s", path, w.Code, w.Body)
		}
	}
	// Reversed roles: the restricted contact as the primary.
	if w := request(a, "POST", "/api/v1/contacts/j-other/merge", tok["mary"], map[string]any{"secondaryId": "m-own", "previewHash": "x"}); w.Code != 404 {
		t.Fatalf("restricted primary: %d", w.Code)
	}
	var alive int
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts WHERE id IN ('m-own','j-other') AND deleted_at IS NULL`).Scan(&alive)
	if alive != 2 {
		t.Fatal("merge wrote")
	}
}
