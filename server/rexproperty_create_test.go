package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// UAT 3 问题 3: an owner added to a property not in Rex creates (or links) the
// Rex property before the Owner relation is written. rexproperty_create.go.

type propMock struct {
	dup                  any // findPossibleDuplicate result
	createID             any
	createFails          bool // no envelope: outcome unknown
	finds, creates       int
	updates              []string // Rex property id of each relation write
	createBody, findBody map[string]any
}

func (m *propMock) handle(t *testing.T) func(string, map[string]any) (any, int) {
	return func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/properties/find-possible-duplicate":
			m.finds++
			m.findBody = b
			return m.dup, 200
		case "/v1/rex/properties/create":
			m.creates++
			m.createBody = b
			if m.createFails {
				return nil, -1
			}
			return m.createID, 200
		case "/v1/rex/properties/update":
			d := b["data"].(map[string]any)
			m.updates = append(m.updates, istring(d["id"]))
			return map[string]any{"id": d["id"]}, 200
		case "/v1/rex/properties/read":
			return map[string]any{"id": b["id"], "etag": "e1"}, 200
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	}
}

// propWorker is sandboxWorker with a mock that can drop the response body
// (code -1), which the Rex client sees as an ambiguous failure.
func propWorker(t *testing.T, a *app, m *propMock) (*rexWorker, *[]string) {
	t.Helper()
	t.Setenv("REX_WRITE_ACCOUNT_ID", "2057")
	calls := &[]string{}
	h := m.handle(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		*calls = append(*calls, r.URL.Path)
		var result any = "token"
		code := 200
		switch r.URL.Path {
		case "/v1/rex/authentication/login":
		case "/v1/rex/AccountSettings/read":
			result = map[string]any{"id": 2057, "name": "Sandbox", "is_test_account": true}
		default:
			result, code = h(r.URL.Path, b)
		}
		if code == -1 {
			w.WriteHeader(502)
			_, _ = w.Write([]byte("<html>Bad gateway</html>"))
			return
		}
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	t.Cleanup(srv.Close)
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p", account: "2057"}
	w.client.AccountID, w.client.Interval, w.client.Retries = "2057", 0, 0
	return w, calls
}

func propFixture(t *testing.T, a *app, owner, pid, no, street string) {
	t.Helper()
	data := ijson(map[string]any{"no": no, "street": street, "suburb": "Sunnybank", "lat": -27.5794, "lng": 153.0606})
	if _, e := a.db.Exec(`INSERT INTO properties(id,owner_id,no,street,suburb,data,registered,created_at,updated_at) VALUES(?,?,?,?,'Sunnybank',?,1,'now','now')`, pid, owner, no, street, data); e != nil {
		t.Fatal(e)
	}
}

// ownerJob mirrors enqueuePropertyOwners for one contact/property pair.
func ownerJob(t *testing.T, a *app, cid, pid, kind string) {
	t.Helper()
	if _, e := a.db.Exec(`INSERT OR IGNORE INTO property_owners(property_id,contact_id) VALUES(?,?)`, pid, cid); e != nil {
		t.Fatal(e)
	}
	payload := ijson(map[string]any{"contactId": cid, "relation": map[string]any{"type": kind, "propertyId": pid}})
	if _, e := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_contact_relation',?,'POST',?,'now')`, cid+":"+pid, payload); e != nil {
		t.Fatal(e)
	}
}

func outboxRow(t *testing.T, a *app, id int) (status, reason string) {
	t.Helper()
	if e := a.db.QueryRow(`SELECT status,COALESCE(last_error,'') FROM rex_outbox WHERE id=?`, id).Scan(&status, &reason); e != nil {
		t.Fatal(e)
	}
	return
}

func propLink(a *app, pid string) string {
	var rid string
	_ = a.db.QueryRow(`SELECT rex_id FROM rex_links WHERE entity='properties' AND local_id=? AND account_id='2057'`, pid).Scan(&rid)
	return rid
}

func steps(t *testing.T, w *rexWorker, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		singleConnGuard(t, w.db, "worker step", func() error { return w.step(context.Background()) })
	}
}

func TestPropertyCreateLinksExistingRexProperty(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	propFixture(t, a, "u", "p", "11", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	m := &propMock{dup: 555}
	w, _ := propWorker(t, a, m)
	steps(t, w, 1)
	if s, r := outboxRow(t, a, 1); s != "sent" {
		t.Fatal(s, r)
	}
	if m.creates != 0 || propLink(a, "p") != "555" || len(m.updates) != 1 || m.updates[0] != "555" {
		t.Fatal(m.creates, propLink(a, "p"), m.updates)
	}
	if m.findBody["adr_street_number"] != "11" || m.findBody["adr_street_name"] != "Boorman St" || m.findBody["adr_postcode"] != "4109" || m.findBody["adr_state_or_region"] != "QLD" {
		t.Fatal(m.findBody)
	}
	var state string
	_ = a.db.QueryRow(`SELECT state FROM rex_property_creates WHERE property_id='p'`).Scan(&state)
	if state != "linked" {
		t.Fatal(state)
	}
}

func TestPropertyCreateNewThenRelation(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	propFixture(t, a, "u", "p", "3/11", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	m := &propMock{dup: false, createID: 777}
	w, _ := propWorker(t, a, m)
	steps(t, w, 1)
	if s, r := outboxRow(t, a, 1); s != "sent" {
		t.Fatal(s, r)
	}
	if m.creates != 1 || propLink(a, "p") != "777" || len(m.updates) != 1 || m.updates[0] != "777" {
		t.Fatal(m.creates, propLink(a, "p"), m.updates)
	}
	d := m.createBody["data"].(map[string]any)
	if d["adr_unit_number"] != "3" || d["adr_street_number"] != "11" || imap(d["property_category"])["id"] != "residential" || d["adr_latitude"] != -27.5794 || m.createBody["return_id"] != true {
		t.Fatal(m.createBody)
	}
	if _, ok := d["system_owner_user"]; ok {
		t.Fatal("system_owner_user must be left to Rex, as for contacts")
	}
}

func TestPropertyCreateIncompleteAddressSkipped(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	propFixture(t, a, "u", "p", "", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	m := &propMock{}
	w, calls := propWorker(t, a, m)
	steps(t, w, 1)
	s, r := outboxRow(t, a, 1)
	if s != "skipped" || !strings.Contains(r, "no street number") || len(*calls) != 0 {
		t.Fatal(s, r, *calls)
	}
}

func TestPropertyCreateOnlyForOwnerAndSwitch(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	propFixture(t, a, "u", "p", "11", "Boorman St")
	ownerJob(t, a, "c", "p", "Tenant of")
	m := &propMock{}
	w, calls := propWorker(t, a, m)
	steps(t, w, 1)
	if s, r := outboxRow(t, a, 1); s != "skipped" || r != "not in Rex" || len(*calls) != 0 {
		t.Fatal(s, r, *calls)
	}
	t.Setenv("REX_PROPERTY_CREATE", "0")
	ownerJob(t, a, "c", "p", "Owner of")
	steps(t, w, 1)
	if s, r := outboxRow(t, a, 2); s != "skipped" || r != "not in Rex" || len(*calls) != 0 {
		t.Fatal(s, r, *calls)
	}
}

func TestPropertyCreateRetryIsIdempotent(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	propFixture(t, a, "u", "p", "11", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	m := &propMock{dup: false, createID: 777}
	w, _ := propWorker(t, a, m)
	steps(t, w, 1)
	// The same job replayed (requeue, or a relation write retried later).
	if _, e := a.db.Exec(`UPDATE rex_outbox SET status='pending' WHERE id=1`); e != nil {
		t.Fatal(e)
	}
	steps(t, w, 1)
	if m.creates != 1 || m.finds != 1 || len(m.updates) != 2 || m.updates[1] != "777" {
		t.Fatal(m.creates, m.finds, m.updates)
	}
}

func TestPropertyCreateTwoOwnersOneProperty(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	if _, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c2','u','{"first":"Teddie","last":"Bear"}','now','now')`); e != nil {
		t.Fatal(e)
	}
	if _, e := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','c2','92','Contacts','2057','now')`); e != nil {
		t.Fatal(e)
	}
	propFixture(t, a, "u", "p", "11", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	ownerJob(t, a, "c2", "p", "Owner of")
	m := &propMock{dup: false, createID: 777}
	w, _ := propWorker(t, a, m)
	steps(t, w, 2)
	s1, _ := outboxRow(t, a, 1)
	s2, _ := outboxRow(t, a, 2)
	if s1 != "sent" || s2 != "sent" || m.creates != 1 || len(m.updates) != 2 || m.updates[1] != "777" {
		t.Fatal(s1, s2, m.creates, m.updates)
	}
	// The phone sees each owner's relation state.
	sync, err := ownerRexSyncAll(a.db)
	if err != nil || imap(sync["p"]["c"])["status"] != "sent" || imap(sync["p"]["c2"])["status"] != "sent" {
		t.Fatal(sync, err)
	}
}

func TestPropertyCreateUnknownOutcomeBlocksThenReconciles(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	if _, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c2','u','{"first":"Teddie"}','now','now')`); e != nil {
		t.Fatal(e)
	}
	if _, e := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','c2','92','Contacts','2057','now')`); e != nil {
		t.Fatal(e)
	}
	propFixture(t, a, "u", "p", "11", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	ownerJob(t, a, "c2", "p", "Owner of")
	m := &propMock{dup: false, createFails: true}
	w, _ := propWorker(t, a, m)
	steps(t, w, 2)
	for _, id := range []int{1, 2} {
		if s, r := outboxRow(t, a, id); s != "failed" || r != propertyCreateUnknown {
			t.Fatal(id, s, r)
		}
	}
	if m.creates != 1 || propLink(a, "p") != "" {
		t.Fatal("second create after an unknown outcome", m.creates)
	}
	// Without --retry-unknown nothing is requeued.
	r, err := backfillPropertyOwners(a.db, "2057", false, false)
	if err != nil || r.Requeued != 0 {
		t.Fatal(r, err)
	}
	// Dry run reports but changes nothing.
	r, err = backfillPropertyOwners(a.db, "2057", true, true)
	if err != nil || r.Requeued != 2 {
		t.Fatal(r, err)
	}
	if s, _ := outboxRow(t, a, 1); s != "failed" {
		t.Fatal("dry run wrote", s)
	}
	// Operator checked Rex: the create did happen. The retry finds and links it.
	if r, err = backfillPropertyOwners(a.db, "2057", false, true); err != nil || r.Requeued != 2 {
		t.Fatal(r, err)
	}
	m.dup, m.createFails = 888, false
	steps(t, w, 2)
	s1, _ := outboxRow(t, a, 1)
	s2, _ := outboxRow(t, a, 2)
	if s1 != "sent" || s2 != "sent" || m.creates != 1 || propLink(a, "p") != "888" {
		t.Fatal(s1, s2, m.creates, propLink(a, "p"))
	}
}

func TestPropertyCreateRexRecordLinkedElsewhere(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	propFixture(t, a, "u", "p", "11", "Boorman St")
	propFixture(t, a, "u", "p2", "11", "Boorman Street")
	if _, e := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('properties','p2','555','Properties','2057','now')`); e != nil {
		t.Fatal(e)
	}
	ownerJob(t, a, "c", "p", "Owner of")
	m := &propMock{dup: 555}
	w, _ := propWorker(t, a, m)
	steps(t, w, 1)
	s, r := outboxRow(t, a, 1)
	if s != "skipped" || !strings.Contains(r, "11 Boorman Street, Sunnybank") || propLink(a, "p") != "" || len(m.updates) != 0 {
		t.Fatal(s, r, propLink(a, "p"), m.updates)
	}
}

func TestPropertyBackfillHistoricalSkipped(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	propFixture(t, a, "u", "p", "11", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	propFixture(t, a, "u", "gone", "1", "Acorus Pl")
	ownerJob(t, a, "c", "gone", "Owner of")
	// Old behaviour left these skipped; the second owner was later removed in the app.
	if _, e := a.db.Exec(`UPDATE rex_outbox SET status='skipped',last_error='not in Rex'`); e != nil {
		t.Fatal(e)
	}
	if _, e := a.db.Exec(`DELETE FROM property_owners WHERE property_id='gone'`); e != nil {
		t.Fatal(e)
	}
	r, err := backfillPropertyOwners(a.db, "2057", false, false)
	if err != nil || r.Requeued != 1 || r.Jobs[0] != "c:p" {
		t.Fatal(r, err)
	}
	m := &propMock{dup: false, createID: 777}
	w, _ := propWorker(t, a, m)
	steps(t, w, 1)
	if s, _ := outboxRow(t, a, 1); s != "sent" || propLink(a, "p") != "777" {
		t.Fatal(s, propLink(a, "p"))
	}
	if s, _ := outboxRow(t, a, 2); s != "skipped" {
		t.Fatal(s)
	}
}

// After the app creates the Rex property, the poller sees it in Properties::search
// and must update the same local row: no second rex-properties-* property.
func TestPropertyCreatePollDoesNotDuplicate(t *testing.T) {
	a, tokens := setup(t)
	if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	var mary string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary)
	if _, e := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c',?,'{"first":"Teddie","last":"Bear"}','now','now')`, mary); e != nil {
		t.Fatal(e)
	}
	if _, e := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at,etag) VALUES('contacts','c','456','Contacts','2057','now','b')`); e != nil {
		t.Fatal(e)
	}
	propFixture(t, a, mary, "p", "11", "Boorman St")
	ownerJob(t, a, "c", "p", "Owner of")
	m := &propMock{dup: false, createID: 777}
	w, _ := propWorker(t, a, m)
	steps(t, w, 1)
	if propLink(a, "p") != "777" {
		t.Fatal(propLink(a, "p"))
	}
	t.Setenv("REX_POLL_SOFT_DELETE", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		var result any
		switch r.URL.Path {
		case "/v1/rex/properties/search":
			result = map[string]any{"rows": []any{map[string]any{"_id": "777", "etag": "e2"}}, "total": 1}
		case "/v1/rex/contacts/search":
			result = map[string]any{"rows": []any{map[string]any{"_id": "456", "etag": "b"}}, "total": 1}
		case "/v1/rex/properties/read":
			result = map[string]any{"id": "777", "etag": "e2", "adr_street_number": "11", "adr_street_name": "Boorman Street", "adr_suburb_or_town": "Sunnybank", "adr_postcode": "4109", "related": map[string]any{"contact_reln_property": []any{map[string]any{"contact": map[string]any{"id": "456"}, "reln_type": map[string]any{"id": "owner"}}}}}
		default:
			result = map[string]any{"rows": []any{}, "total": 0}
		}
		_ = json.NewEncoder(rw).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly, c.AccountID = "mock", 0, true, "2057"
	lastRelationPoll = time.Now()
	if err := pollRex(context.Background(), a.db, c, "2057"); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE id LIKE 'rex-properties-%'`).Scan(&n)
	if n != 0 {
		t.Fatal("poll created a second local property", n)
	}
	var owners int
	_ = a.db.QueryRow(`SELECT count(*) FROM property_owners WHERE property_id='p' AND contact_id='c'`).Scan(&owners)
	if owners != 1 {
		t.Fatal(owners)
	}
	// The phone receives rexId (Open in Rex) and the per-owner status.
	t.Setenv("REX_ACCOUNT_ID", "2057")
	res := request(a, "GET", "/api/v1/sync", tokens["admi"], nil)
	var pull struct {
		Properties []map[string]any `json:"properties"`
	}
	_ = json.Unmarshal(res.Body.Bytes(), &pull)
	found := false
	for _, p := range pull.Properties {
		if p["id"] == "p" {
			found = true
			if p["rexId"] != "777" || imap(imap(p["ownerRexSync"])["c"])["status"] != "sent" {
				t.Fatal(p["rexId"], p["ownerRexSync"])
			}
		}
	}
	if !found {
		t.Fatal("property missing from pull", res.Code)
	}
}
