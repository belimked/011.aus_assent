package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// singleConnGuard runs fn against the production pool shape (open() sets
// MaxOpenConns(1)). A nested query while *sql.Rows or a *sql.Tx still holds the
// only connection blocks forever, so a hang here is a deadlock regression. After
// fn returns, no connection may remain checked out (an unclosed Rows/Tx/Row).
func singleConnGuard(t *testing.T, db *sql.DB, name string, fn func() error) {
	t.Helper()
	if n := db.Stats().MaxOpenConnections; n != 1 {
		t.Fatalf("%s: test pool must mirror production MaxOpenConns(1), got %d", name, n)
	}
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	case <-time.After(30 * time.Second):
		buf := make([]byte, 1<<20)
		n := runtime.Stack(buf, true)
		fmt.Fprintf(os.Stderr, "%s\n", buf[:n])
		panic(name + " deadlocked on the single database connection")
	}
	if s := db.Stats(); s.InUse != 0 {
		t.Fatalf("%s left %d database connection(s) checked out", name, s.InUse)
	}
}

func syncPushOps(a *app, token string, ops ...map[string]any) error {
	w := request(a, "POST", "/api/v1/sync/push", token, map[string]any{"ops": ops})
	if w.Code != 200 {
		return fmt.Errorf("push %d %s", w.Code, w.Body)
	}
	if strings.Contains(w.Body.String(), `"invalid"`) {
		return fmt.Errorf("push result %s", w.Body)
	}
	return nil
}

// /sync is hit by every phone; push runs mutate (owner lookups + tx) and pull
// walks every kind with per-id gets, zones, Rex links and lost-scope tombstones.
func TestSyncSingleConnection(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("REX_ACCOUNT_ID", "2057")
	mary := tokens["mary"]
	var cursor string
	appConnGuard(t, a, "sync push+pull", func() error {
		if err := syncPushOps(a, mary,
			map[string]any{"opId": "z", "kind": "zones", "op": "upsert", "id": "zone-1", "data": map[string]any{"id": "zone-1", "name": "Z", "poly": [][]float64{{-27.6, 153.0}, {-27.6, 153.1}, {-27.5, 153.1}, {-27.5, 153.0}}}},
			map[string]any{"opId": "c", "kind": "contacts", "op": "upsert", "id": "c-1", "data": map[string]any{"id": "c-1", "first": "Tom", "last": "Lee", "mobile": "0412345678", "roles": []string{"owner"}, "tags": []string{}, "flags": map[string]any{}, "notes": []any{}}},
			map[string]any{"opId": "p", "kind": "properties", "op": "upsert", "id": "p-1", "data": map[string]any{"id": "p-1", "no": "1", "street": "Test St", "suburb": "Sunnybank", "lat": -27.57, "lng": 153.06, "ownerIds": []string{"c-1"}, "flags": map[string]any{}}},
			map[string]any{"opId": "t", "kind": "todos", "op": "upsert", "id": "t-1", "data": map[string]any{"id": "t-1", "t": "Call Tom", "d": "2026-11-20", "propId": "p-1"}},
		); err != nil {
			return err
		}
		if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('properties','p-1','r1','Properties',?,'2057')`, now()); err != nil {
			return err
		}
		w := request(a, "GET", "/api/v1/sync", mary, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"p-1"`) {
			return fmt.Errorf("full pull %d %s", w.Code, w.Body)
		}
		var m map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		cursor, _ = m["cursor"].(string)
		var version int
		if err := a.db.QueryRow(`SELECT version FROM properties WHERE id='p-1'`).Scan(&version); err != nil {
			return err
		}
		// PATCH with unchanged owners re-reads property_owners inside mutate.
		if err := syncPushOps(a, mary, map[string]any{"opId": "p2", "kind": "properties", "op": "upsert", "id": "p-1", "baseVersion": version, "data": map[string]any{"pinned": true}}); err != nil {
			return err
		}
		// Reassign the to-do away so the incremental pull sends a lost-scope tombstone.
		var john string
		if err := a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&john); err != nil {
			return err
		}
		if _, err := a.db.Exec(`UPDATE todos SET assignee_id=?,created_by=? WHERE id='t-1'`, john, john); err != nil {
			return err
		}
		w = request(a, "GET", "/api/v1/sync?since="+cursor, mary, nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"pinned":true`) {
			return fmt.Errorf("incremental pull %d %s", w.Code, w.Body)
		}
		if w = request(a, "GET", "/api/v1/sync", tokens["admi"], nil); w.Code != 200 {
			return fmt.Errorf("admin pull %d", w.Code)
		}
		return nil
	})
}

// Background poller: Rex search/read, import transactions, relation targets,
// soft-delete reconcile, relation refresh and OFI listings, while every Rex
// call is also logged to the same database (rexDBLogger).
func TestRexPollSingleConnection(t *testing.T) {
	a, _ := setup(t)
	if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	var mary string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary)
	past := time.Now().UTC().Add(-time.Hour).Format(layout)
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,first,data,created_at,updated_at) VALUES('gone',?,'Gone','{"source":"rex_import"}',?,?)`, mary, past, past); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('contacts','gone','999','Contacts',?,'2057')`, past); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REX_POLL_SOFT_DELETE", "1")
	for k, v := range map[string]string{"REX_BASE_URL": "mock", "REX_APP_ID": "test", "REX_EMAIL": "test", "REX_PASSWORD": "test", "REX_ACCOUNT_ID": "2057"} {
		t.Setenv(k, v)
	}
	t.Setenv("AXLEO_REX_READ", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		args, _ := body["args"].(map[string]any)
		if args == nil {
			args = body
		}
		var result any = map[string]any{"rows": []any{}, "total": 0}
		switch r.URL.Path {
		case "/v1/rex/properties/search":
			result = map[string]any{"rows": []any{map[string]any{"_id": "p1", "etag": "e1"}}, "total": 1}
		case "/v1/rex/contacts/search":
			result = map[string]any{"rows": []any{map[string]any{"_id": "c1", "etag": "e1"}}, "total": 1}
		case "/v1/rex/properties/read":
			result = map[string]any{"id": "p1", "etag": "e1", "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank", "adr_latitude": "-27.57", "adr_longitude": "153.06",
				"related": map[string]any{"contact_reln_property": []any{map[string]any{"contact": map[string]any{"id": "c1"}, "reln_type": map[string]any{"text": "owner"}}, map[string]any{"contact": map[string]any{"id": "c2"}, "reln_type": map[string]any{"text": "owner"}}}}}
		case "/v1/rex/contacts/read":
			id := fmt.Sprint(args["id"])
			result = map[string]any{"id": id, "etag": "e1", "name_first": "Owner", "name_last": id}
		case "/v1/rex/listings/search":
			result = map[string]any{"rows": []any{map[string]any{"id": "l1", "property": map[string]any{"id": "p1"}}}}
		case "/v1/rex/listing-events/search":
			result = map[string]any{"rows": []any{map[string]any{"id": "e1", "listing": map[string]any{"id": "l1"}, "event_type": "open_home", "event_date": time.Now().In(time.FixedZone("AEST", 10*3600)).AddDate(0, 0, 1).Format("2006-01-02"), "event_time": "10:00:00", "event_end_time": "10:30:00"}}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly, c.AccountID = "mock", 0, true, "2057"
	c.Log = rexDBLogger(a.db)
	lastRelationPoll = time.Time{}
	t.Cleanup(func() { lastRelationPoll = time.Time{} })
	appConnGuard(t, a, "pollRex", func() error {
		for i := 0; i < 2; i++ { // second pass: etags unchanged, relation refresh only
			if err := pollRex(context.Background(), a.db, c, "2057"); err != nil {
				return err
			}
			lastRelationPoll = time.Time{}
		}
		return nil
	})
	var owners, deleted, logged int
	_ = a.db.QueryRow(`SELECT count(*) FROM property_owners WHERE property_id='rex-properties-2057-p1'`).Scan(&owners)
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts WHERE id='gone' AND deleted_at IS NOT NULL`).Scan(&deleted)
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_request_log`).Scan(&logged)
	var listings int
	_ = a.db.QueryRow(`SELECT count(*) FROM listings WHERE id='rex-listings-2057-l1'`).Scan(&listings)
	if owners != 2 || deleted != 1 || logged == 0 || listings != 1 {
		t.Fatalf("owners=%d deleted=%d logged=%d listings=%d", owners, deleted, logged, listings)
	}
}

// OFI import on its own schedule (pollRexOFI) with per-listing transactions.
func TestRexOFISingleConnection(t *testing.T) {
	a, _ := setup(t)
	for k, v := range map[string]string{"REX_BASE_URL": "mock", "REX_APP_ID": "test", "REX_EMAIL": "test", "REX_PASSWORD": "test", "REX_ACCOUNT_ID": "2057"} {
		t.Setenv(k, v)
	}
	t.Setenv("AXLEO_REX_READ", "1")
	day := time.Date(2026, 11, 20, 14, 5, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		var result any = map[string]any{"rows": []any{}}
		switch r.URL.Path {
		case "/v1/rex/listings/search":
			result = map[string]any{"rows": []any{map[string]any{"id": "l1", "property": map[string]any{"id": "p1"}}, map[string]any{"id": "l2", "property": map[string]any{"id": "p2"}}}}
		case "/v1/rex/listing-events/search":
			result = map[string]any{"rows": []any{map[string]any{"id": "e1", "listing": map[string]any{"id": "l1"}, "event_type": "open_home", "event_date": "2026-11-21", "event_time": "10:00:00", "event_end_time": "10:30:00"}, map[string]any{"id": "e2", "listing": map[string]any{"id": "l2"}, "event_type": "open_home", "event_date": "2026-11-21", "event_time": "11:00:00", "event_end_time": "11:30:00"}}}
		case "/v1/rex/properties/read":
			result = map[string]any{"id": fmt.Sprint(body["id"]), "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Interval, c.ReadOnly = 0, true
	c.Log = rexDBLogger(a.db)
	appConnGuard(t, a, "pollRexOFI", func() error {
		for i := 0; i < 2; i++ {
			if err := pollRexOFI(context.Background(), a.db, c, "2057", day.Add(time.Duration(i)*6*time.Minute)); err != nil {
				return err
			}
		}
		return nil
	})
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM listings WHERE id LIKE 'rex-listings-2057-%'`).Scan(&n)
	if n == 0 {
		t.Fatal("no listings imported")
	}
}

// Rex outbox worker: job claim, Rex calls (logged to the DB), local duplicate
// search over linked contacts, and the finish transaction.
func TestRexWorkerSingleConnection(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	for i, mobile := range []string{"0400000001", "0499999999", "0400000002"} {
		id := fmt.Sprintf("dup-%d", i)
		if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,mobile,data,created_at,updated_at) VALUES(?,'u',?,'{}','now','now')`, id, mobile); err != nil {
			t.Fatal(err)
		}
		if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts',?,?,'Contacts','2057','now')`, id, fmt.Sprint(500+i)); err != nil {
			t.Fatal(err)
		}
	}
	workerQueue(t, a, "contacts", "c", `{"mobile":"0499999999"}`)
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/read":
			return map[string]any{"id": 91, "etag": "new", "related": map[string]any{"contact_names": []any{map[string]any{"id": 11}}, "contact_phones": []any{map[string]any{"id": 22}}}}, 200
		case "/v1/rex/contacts/update":
			return map[string]any{"id": 91, "etag": "new"}, 200
		}
		return nil, 500
	})
	w.client.Log = rexDBLogger(a.db)
	appConnGuard(t, a, "rex worker", func() error {
		// Early return from inside the rows loop must still release the connection.
		if got := w.findLocalDuplicate("c", "+61 499 999 999"); got != "501" {
			return fmt.Errorf("duplicate %q", got)
		}
		return w.step(context.Background())
	})
	if s := workerStatus(t, a, 1); s != "sent" {
		t.Fatal(s)
	}
}

// Property notes: mutate snapshots the old notes, the record tx checks for an
// existing job, the worker reads links/users and finishes, and the pull lists
// per-note outbox rows while building each property.
func TestPropertyNoteSingleConnection(t *testing.T) {
	a, tokens := setup(t)
	t.Setenv("REX_ACCOUNT_ID", "2057")
	mary := tokens["mary"]
	var maryID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&maryID)
	created := 777
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) { created++; return created, 200 })
	w.client.Log = rexDBLogger(a.db)
	appConnGuard(t, a, "property notes", func() error {
		if err := syncPushOps(a, mary, map[string]any{"opId": "p", "kind": "properties", "op": "upsert", "id": "p-1", "data": map[string]any{"id": "p-1", "no": "1", "street": "Test St", "suburb": "Sunnybank", "ownerIds": []string{}, "flags": map[string]any{}, "notes": []any{note("n-1", "2026-11-25", "Mary", "Hi")}}}); err != nil {
			return err
		}
		if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('properties','p-1','r1','Properties',?,'2057')`, now()); err != nil {
			return err
		}
		var version int
		if err := a.db.QueryRow(`SELECT version FROM properties WHERE id='p-1'`).Scan(&version); err != nil {
			return err
		}
		if err := syncPushOps(a, mary, map[string]any{"opId": "p2", "kind": "properties", "op": "upsert", "id": "p-1", "baseVersion": version, "data": map[string]any{"notes": []any{note("n-2", "2026-11-25", "Mary", "Again"), note("n-1", "2026-11-25", "Mary", "Hi")}}}); err != nil {
			return err
		}
		for i := 0; i < 4; i++ { // two property PATCH jobs interleave with the note jobs
			if err := w.step(context.Background()); err != nil {
				return err
			}
		}
		if r := request(a, "GET", "/api/v1/sync", mary, nil); r.Code != 200 || !strings.Contains(r.Body.String(), `"noteRexSync"`) {
			return fmt.Errorf("pull %d %s", r.Code, r.Body)
		}
		return nil
	})
	var sent int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_links WHERE entity='rex_property_note'`).Scan(&sent)
	if sent != 2 {
		t.Fatal("notes linked:", sent)
	}
}

// Admin market import (RP Data + Pricefinder-style attributes) in one transaction,
// followed by the import list/detail endpoints.
func TestMarketImportSingleConnection(t *testing.T) {
	a, tokens := setup(t)
	if err := seed(a.db, true); err != nil {
		t.Fatal(err)
	}
	var admin string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE role='admin' LIMIT 1`).Scan(&admin)
	p := principal{id: admin, role: "admin"}
	b := rpFixture(t, "10 Jul 2016", "First Example")
	appConnGuard(t, a, "market import", func() error {
		for _, dry := range []bool{true, false} {
			report, err := a.importMarket(b, "propertyExport_20261125103259.xlsx", "rpdata", "auto", dry, p)
			if err != nil {
				return err
			}
			if report.Success != 1 {
				return fmt.Errorf("dry=%v %+v", dry, report)
			}
		}
		if w := request(a, "GET", "/api/v1/admin/market/imports", tokens["admi"], nil); w.Code != 200 {
			return fmt.Errorf("imports list %d", w.Code)
		}
		if w := request(a, "GET", "/api/v1/admin/properties?suburb=Sunnybank", tokens["admi"], nil); w.Code != 200 {
			return fmt.Errorf("admin properties %d", w.Code)
		}
		return nil
	})
}
