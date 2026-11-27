package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mailingRowsOf(t *testing.T, db *sql.DB, cid string) map[string]string {
	t.Helper()
	rows, err := loadMailingRows(db, cid)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.name] = r.status
	}
	return out
}
func mailingJobs(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT entity,entity_id,payload,status FROM rex_outbox WHERE entity LIKE 'rex_contact_mailing_%' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var e, id, p, s string
		_ = rows.Scan(&e, &id, &p, &s)
		out = append(out, strings.TrimPrefix(e, "rex_contact_mailing_")+"|"+id+"|"+p+"|"+s)
	}
	return out
}
func mailingWrite(t *testing.T, db *sql.DB, cid string, data map[string]any) {
	t.Helper()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	// enqueueRexDetails runs after record() stored this write's own contacts outbox row.
	if _, err = tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at) VALUES('contacts',?,'PATCH',?,'sent',?)`, cid, ijson(data), now()); err != nil {
		t.Fatal(err)
	}
	if err = enqueueRexDetails(tx, "contacts", cid, data, now()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
func rexReadEnv(t *testing.T) {
	for k, v := range map[string]string{"REX_BASE_URL": "mock", "REX_APP_ID": "test", "REX_EMAIL": "test", "REX_PASSWORD": "test", "REX_ACCOUNT_ID": "2057", "AXLEO_REX_READ": "1"} {
		t.Setenv(k, v)
	}
}

func TestMailingCatalogRefreshThrottledAndServed(t *testing.T) {
	a, tok := setup(t)
	rexReadEnv(t)
	calls, fail := 0, false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/rex/admin-mailing-lists/search" {
			t.Errorf("unexpected %s", r.URL.Path)
		}
		calls++
		if fail {
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(map[string]any{"result": nil, "error": map[string]any{"message": "nope"}})
			return
		}
		rows := []any{map[string]any{"id": 2, "name": "Sellers  SMS", "list_type": map[string]any{"id": "sms"}}, map[string]any{"id": 1, "name": "Buyers newsletter", "list_type": "email"}, map[string]any{"id": 3, "name": ""}}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"rows": rows, "total": 3}, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly, c.Retries = "mock", 0, true, 0
	at := time.Now()
	singleConnGuard(t, a.db, "catalog refresh", func() error { return pollRexMailingLists(context.Background(), a.db, c, "2057", at) })
	if err := pollRexMailingLists(context.Background(), a.db, c, "2057", at.Add(5*time.Hour)); err != nil || calls != 1 {
		t.Fatal("throttle", calls, err)
	}
	w := request(a, "GET", "/api/v1/mailing-lists", tok["mary"], nil)
	if w.Code != 200 || ijson(obj(t, w)["items"]) != `[{"channel":"email","name":"Buyers newsletter"},{"channel":"sms","name":"Sellers SMS"}]` {
		t.Fatal(w.Code, w.Body)
	}
	// After the interval a failed refresh keeps the cached catalogue and backs off 30 minutes.
	fail = true
	if err := pollRexMailingLists(context.Background(), a.db, c, "2057", at.Add(7*time.Hour)); err == nil || calls != 2 {
		t.Fatal("refresh", calls, err)
	}
	_ = pollRexMailingLists(context.Background(), a.db, c, "2057", at.Add(7*time.Hour+10*time.Minute))
	if calls != 2 || !strings.Contains(request(a, "GET", "/api/v1/mailing-lists", tok["mary"], nil).Body.String(), "Buyers newsletter") {
		t.Fatal("backoff or cache lost", calls)
	}
	if w = request(a, "GET", "/api/v1/mailing-lists", "", nil); w.Code != 401 {
		t.Fatal(w.Code)
	}
}

func TestMailingImportPollKeepsPendingThenRexWins(t *testing.T) {
	a := workerDB(t)
	rex := []any{map[string]any{"id": 1, "mailing_list_name": "Buyers", "subscription_status": "active", "subscribe_timestamp": 1557818366}, map[string]any{"id": 2, "mailing_list_name": "Old list", "subscription_status": "unsubscribed", "unsubscribe_reason": "spam"}}
	row := func(etag string) importRow {
		return importRow{"id": "91", "etag": etag, "name_first": "Jane", "related": map[string]any{"contact_mailing_lists": rex}}
	}
	save := func(etag string) {
		t.Helper()
		singleConnGuard(t, a.db, "import "+etag, func() error { return importSave(a.db, "contacts", "2057", row(etag), "u", false, &importStats{}) })
	}
	save("e1")
	cid := "rex-contacts-2057-91"
	if got := mailingRowsOf(t, a.db, cid); len(got) != 2 || got["Buyers"] != "active" || got["Old list"] != "unsubscribed" {
		t.Fatal(got)
	}
	item := map[string]any{}
	attachMailingLists(a.db, cid, item)
	if ijson(item["mailingLists"]) != `["Buyers"]` || item["rexLinked"] != true {
		t.Fatal(item)
	}
	// A search row without the related key never wipes memberships.
	if err := importSave(a.db, "contacts", "2057", importRow{"id": "91", "etag": "e2"}, "u", false, &importStats{}); err != nil || len(mailingRowsOf(t, a.db, cid)) != 2 {
		t.Fatal(err, mailingRowsOf(t, a.db, cid))
	}
	// The phone removes Buyers and adds New; Rex has not seen either yet.
	mailingWrite(t, a.db, cid, map[string]any{"mailingLists": []any{"New"}, "mailingListsSeen": []any{"Buyers"}})
	if got := mailingRowsOf(t, a.db, cid); got["Buyers"] != "" || got["New"] != "" || len(got) != 2 {
		t.Fatal("local state", got)
	}
	save("e3")
	if got := mailingRowsOf(t, a.db, cid); len(got) != 2 || got["New"] != "" || got["Old list"] != "unsubscribed" {
		t.Fatal("poll clobbered pending changes", got)
	}
	if _, ok := mailingRowsOf(t, a.db, cid)["Buyers"]; ok {
		t.Fatal("pending remove undone by poll")
	}
	// Jobs done: Rex wins on the next read.
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='failed' WHERE entity LIKE 'rex_contact_mailing_%'`); err != nil {
		t.Fatal(err)
	}
	save("e4")
	if got := mailingRowsOf(t, a.db, cid); len(got) != 2 || got["Buyers"] != "active" {
		t.Fatal("Rex did not win", got)
	}
}

func TestMailingImportSearchRequestsExtraField(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"rows": []any{}, "total": 0}, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly = "mock", 0, true
	if err := importPages(context.Background(), c, "Contacts", 0, func(importRow) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if ijson(body["extra_options"]) != `{"extra_fields":["contact_mailing_lists"]}` {
		t.Fatal(body)
	}
}

func TestMailingEnqueueSeparateIdempotentAndStaleSafe(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", " buyers ", map[string]any{"name": "Sellers"}}})
	jobs := mailingJobs(t, a.db)
	if len(jobs) != 2 || jobs[0] != `add|c:buyers|{"contactId":"c","name":"Buyers"}|waiting_auth` || jobs[1] != `add|c:sellers|{"contactId":"c","name":"Sellers"}|waiting_auth` {
		t.Fatal(jobs)
	}
	// Same push again (or a second phone): no duplicate job or row.
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", "Sellers"}})
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"buyers", "Sellers"}, "mailingListsSeen": []any{}})
	if len(mailingJobs(t, a.db)) != 2 || len(mailingRowsOf(t, a.db, "c")) != 2 {
		t.Fatal(mailingJobs(t, a.db), mailingRowsOf(t, a.db, "c"))
	}
	// The push response carries the server state as the new baseline.
	if d := contactData(t, a, "c"); ijson(d["mailingListsSeen"]) != `["Buyers","Sellers"]` {
		t.Fatal(d)
	}
	// Rex added "From Rex"; a phone that has not pulled it yet edits something else.
	if _, err := a.db.Exec(`INSERT INTO contact_mailing_lists(contact_id,name,status,updated_at) VALUES('c','From Rex','active',?)`, now()); err != nil {
		t.Fatal(err)
	}
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", "Sellers"}, "mailingListsSeen": []any{"Buyers", "Sellers"}})
	if len(mailingJobs(t, a.db)) != 2 || mailingRowsOf(t, a.db, "c")["From Rex"] != "active" {
		t.Fatal("stale phone removed a Rex list", mailingJobs(t, a.db))
	}
	// Remove: a separate job; removing it again (or an absent list) queues nothing.
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", "From Rex"}, "mailingListsSeen": []any{"Buyers", "Sellers", "From Rex"}})
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", "From Rex"}, "mailingListsSeen": []any{"Buyers", "Sellers", "From Rex"}})
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", "From Rex"}, "mailingListsSeen": []any{"Buyers", "From Rex", "Never"}})
	jobs = mailingJobs(t, a.db)
	if len(jobs) != 3 || jobs[2] != `remove|c:sellers|{"contactId":"c","name":"Sellers"}|waiting_auth` {
		t.Fatal(jobs)
	}
	// Two pushes before a pull: the second still sees the first push's add.
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", "From Rex", "Quick"}, "mailingListsSeen": []any{"Buyers", "From Rex"}})
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers", "From Rex"}, "mailingListsSeen": []any{"Buyers", "From Rex"}})
	if jobs = mailingJobs(t, a.db); len(jobs) != 5 || !strings.HasPrefix(jobs[4], "remove|c:quick|") {
		t.Fatal(jobs)
	}
	// Writes without mailingLists (older app, reassign, partial PATCH) change nothing.
	mailingWrite(t, a.db, "c", map[string]any{"tags": []any{"x"}})
	if len(mailingJobs(t, a.db)) != 5 {
		t.Fatal(mailingJobs(t, a.db))
	}
}

func TestMailingWorkerRequestBodiesAndIdempotency(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	var rexRows []any
	updates := []map[string]any{}
	w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/read":
			if b["id"] != "91" {
				t.Errorf("read %v", b)
			}
			return map[string]any{"id": 91, "etag": "e", "related": map[string]any{"contact_mailing_lists": rexRows}}, 200
		case "/v1/rex/contacts/update":
			updates = append(updates, b)
			return map[string]any{"id": 91}, 200
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	})
	step := func(name string) {
		t.Helper()
		singleConnGuard(t, a.db, name, func() error { return w.step(context.Background()) })
	}
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers"}})
	step("add")
	if len(updates) != 1 || ijson(updates[0]) != `{"data":{"id":"91","related":{"contact_mailing_lists":[{"mailing_list_name":"Buyers"}]}}}` {
		t.Fatal(updates)
	}
	if got := mailingRowsOf(t, a.db, "c"); got["Buyers"] != "active" {
		t.Fatal(got)
	}
	// Duplicate add (job replayed, list already in Rex): read only, no second row.
	rexRows = []any{map[string]any{"id": 7, "mailing_list_name": "Buyers", "subscription_status": "active"}, map[string]any{"id": 8, "mailing_list_name": "Sellers", "subscription_status": "active"}}
	workerQueue(t, a, mailingAdd, "c:buyers", `{"contactId":"c","name":"buyers"}`)
	step("duplicate add")
	if len(updates) != 1 {
		t.Fatal("duplicate add wrote", updates)
	}
	// Remove destroys the Rex row by id; an absent list is a successful no-op.
	workerQueue(t, a, mailingRemove, "c:sellers", `{"contactId":"c","name":"Sellers"}`)
	step("remove")
	if len(updates) != 2 || ijson(updates[1]) != `{"data":{"id":"91","related":{"contact_mailing_lists":[{"_destroy":true,"id":8}]}}}` {
		t.Fatal(updates)
	}
	// Completed: Rex state is saved locally (Sellers gone, Buyers kept).
	if got := mailingRowsOf(t, a.db, "c"); len(got) != 1 || got["Buyers"] != "active" {
		t.Fatal(got)
	}
	rexRows = rexRows[:1]
	workerQueue(t, a, mailingRemove, "c:gone", `{"contactId":"c","name":"Gone"}`)
	step("remove absent")
	if len(updates) != 2 {
		t.Fatal(updates)
	}
	// Previously unsubscribed in Rex: not silently re-subscribed.
	rexRows = []any{map[string]any{"id": 9, "mailing_list_name": "Promo", "subscription_status": "unsubscribed"}}
	workerQueue(t, a, mailingAdd, "c:promo", `{"contactId":"c","name":"Promo"}`)
	step("unsubscribed")
	var statuses []string
	rows, _ := a.db.Query(`SELECT status||':'||COALESCE(last_error,'') FROM rex_outbox WHERE entity LIKE 'rex_contact_mailing_%' ORDER BY id`)
	for rows.Next() {
		var s string
		_ = rows.Scan(&s)
		statuses = append(statuses, s)
	}
	rows.Close()
	if fmt.Sprint(statuses) != "[sent: sent: sent: sent: failed:unsubscribed in Rex (unsubscribed); re-subscribe in Rex]" || len(updates) != 2 {
		t.Fatal(statuses, updates)
	}
	if len(*calls) < 3 || (*calls)[0] != "/v1/rex/authentication/login" {
		t.Fatal(*calls)
	}
}

func TestMailingUnlinkedContactLikeTags(t *testing.T) {
	a := workerDB(t)
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c','u','{"first":"Demo"}','now','now')`); err != nil {
		t.Fatal(err)
	}
	mailingWrite(t, a.db, "c", map[string]any{"mailingLists": []any{"Buyers"}})
	workerQueue(t, a, "rex_contact_tag", "c:x", `{"contactId":"c","tag":"x"}`)
	w, calls := sandboxWorker(t, a, func(string, map[string]any) (any, int) { t.Error("unlinked write"); return nil, 500 })
	for i := 0; i < 2; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	var mail, tag string
	_ = a.db.QueryRow(`SELECT status||':'||last_error FROM rex_outbox WHERE entity=?`, mailingAdd).Scan(&mail)
	_ = a.db.QueryRow(`SELECT status||':'||last_error FROM rex_outbox WHERE entity='rex_contact_tag'`).Scan(&tag)
	if mail != tag || mail != "skipped:contact not in Rex" || len(*calls) != 0 {
		t.Fatal(mail, tag, *calls)
	}
	// Kept locally, like the tag.
	item := map[string]any{}
	attachMailingLists(a.db, "c", item)
	if ijson(item["mailingLists"]) != `["Buyers"]` || item["rexLinked"] != false || imap(imap(item["mailingListSync"])["buyers"])["status"] != "skipped" {
		t.Fatal(item)
	}
}

func TestMailingSyncPushPull(t *testing.T) {
	a, tok := setup(t)
	mary := tok["mary"]
	singleConnGuard(t, a.db, "mailing push+pull", func() error {
		if err := syncPushOps(a, mary, map[string]any{"opId": "c", "kind": "contacts", "op": "upsert", "id": "c-1", "data": map[string]any{"id": "c-1", "first": "Tom", "mobile": "0412345678", "roles": []string{}, "tags": []string{}, "flags": map[string]any{}, "notes": []any{}, "mailingLists": []string{"Buyers"}}}); err != nil {
			return err
		}
		w := request(a, "GET", "/api/v1/sync", mary, nil)
		var m struct {
			Contacts []map[string]any `json:"contacts"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &m)
		for _, c := range m.Contacts {
			if c["id"] == "c-1" {
				if ijson(c["mailingLists"]) != `["Buyers"]` || imap(imap(c["mailingListSync"])["buyers"])["op"] != "add" || c["rexLinked"] != false {
					return fmt.Errorf("pulled %v", c)
				}
				return nil
			}
		}
		return fmt.Errorf("contact missing %s", w.Body)
	})
	// John cannot see Mary's contact, so not its lists either.
	if strings.Contains(request(a, "GET", "/api/v1/sync", tok["john"], nil).Body.String(), "Buyers") {
		t.Fatal("scope leak")
	}
}

func TestMailingMergeNoLossNoDuplicates(t *testing.T) {
	a := rexMergeSetup(t, "91", "92")
	for _, r := range [][3]string{{"a", "Shared", "active"}, {"a", "Only A", "active"}, {"b", "shared", "active"}, {"b", "Only B", "active"}, {"b", "Old", "unsubscribed"}} {
		if _, err := a.db.Exec(`INSERT INTO contact_mailing_lists(contact_id,name,status,updated_at) VALUES(?,?,?,?)`, r[0], r[1], r[2], now()); err != nil {
			t.Fatal(err)
		}
	}
	workerQueue(t, a, mailingAdd, "b:only b", `{"contactId":"b","name":"Only B"}`)
	plan := runMerge(t, a)
	if plan.PrimaryID != "a" {
		t.Fatal(plan.PrimaryID)
	}
	got := mailingRowsOf(t, a.db, "a")
	if len(got) != 4 || got["Shared"] != "active" || got["Only B"] != "active" || got["Old"] != "unsubscribed" || len(mailingRowsOf(t, a.db, "b")) != 0 {
		t.Fatal(got)
	}
	jobs := mailingJobs(t, a.db)
	if len(jobs) != 2 || !strings.HasSuffix(jobs[0], "|skipped") || jobs[1] != `add|a:only b|{"contactId":"a","name":"Only B"}|waiting_auth` {
		t.Fatal(jobs)
	}
	// The add waits for the Rex merge (combineRecords may or may not carry lists).
	if _, err := a.db.Exec(`UPDATE rex_outbox SET next_attempt_at='2999-01-01T00:00:00Z' WHERE entity='rex_merge'`); err != nil {
		t.Fatal(err)
	}
	w, calls := sandboxWorker(t, a, func(string, map[string]any) (any, int) { t.Error("write before merge"); return nil, 500 })
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	var reason string
	_ = a.db.QueryRow(`SELECT last_error FROM rex_outbox WHERE entity=? AND entity_id='a:only b'`, mailingAdd).Scan(&reason)
	if reason != "waiting for contact merge" || len(*calls) != 0 {
		t.Fatal(reason, *calls)
	}
}

func TestMailingPollReadKeepsPendingAdd(t *testing.T) {
	a, _ := setup(t)
	if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REX_POLL_SOFT_DELETE", "0")
	etag := "a"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch r.URL.Path {
		case "/v1/rex/properties/search":
			result = map[string]any{"rows": []any{}, "total": 0}
		case "/v1/rex/contacts/search":
			result = map[string]any{"rows": []any{map[string]any{"id": "456", "etag": etag}}, "total": 1}
		case "/v1/rex/contacts/read":
			result = map[string]any{"id": "456", "etag": etag, "related": map[string]any{"contact_names": []any{map[string]any{"name_first": "Teddie"}}, "contact_mailing_lists": []any{map[string]any{"id": 5, "mailing_list_name": "Buyers", "subscription_status": "active"}}}}
		default:
			result = map[string]any{"rows": []any{}, "total": 0}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly, c.AccountID = "mock", 0, true, "2057"
	lastRelationPoll = time.Now()
	poll := func() {
		t.Helper()
		singleConnGuard(t, a.db, "poll "+etag, func() error { return pollRex(context.Background(), a.db, c, "2057") })
	}
	poll()
	cid := "rex-contacts-2057-456"
	if got := mailingRowsOf(t, a.db, cid); len(got) != 1 || got["Buyers"] != "active" {
		t.Fatal(got)
	}
	mailingWrite(t, a.db, cid, map[string]any{"mailingLists": []any{"Buyers", "Sellers"}, "mailingListsSeen": []any{"Buyers"}})
	etag = "b"
	poll()
	if got := mailingRowsOf(t, a.db, cid); len(got) != 2 || got["Sellers"] != "" {
		t.Fatal("poll dropped the pending add", got)
	}
}
