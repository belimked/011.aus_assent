package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func appContact(t *testing.T, a *app, id, mobile, email string) {
	t.Helper()
	data := map[string]any{"id": id, "first": "Teddie", "last": "Bear", "mobile": mobile, "email": email, "mobileNorm": normAUMobile(mobile)}
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,email,data,created_at,updated_at) VALUES(?,'u','Teddie','Bear',?,?,?,?,?)`, id, mobile, email, ijson(data), now(), now()); err != nil {
		t.Fatal(err)
	}
}
func createJob(t *testing.T, a *app, id string) {
	t.Helper()
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts',?,'POST','{}','now')`, id); err != nil {
		t.Fatal(err)
	}
}
func linkOf(a *app, id string) string {
	var rex string
	_ = a.db.QueryRow(`SELECT rex_id FROM rex_links WHERE entity='contacts' AND local_id=?`, id).Scan(&rex)
	return rex
}
func dupOf(t *testing.T, a *app, id string) []string {
	t.Helper()
	var raw string
	if err := a.db.QueryRow(`SELECT data FROM contacts WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(raw), &m)
	return stringsFrom(m["possibleDuplicateOf"])
}

func TestWorkerAdoptsUniqueRexMatch(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401 999 088", "teddie@example.com")
	createJob(t, a, "c")
	var updates []map[string]any
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/find-matching-contacts":
			if b["contact_type"] != "person" || fmt.Sprint(b["phones"]) != "[0401 999 088]" {
				t.Errorf("find body %v", b)
			}
			return rexFindResult(map[string]any{"id": 68313642, "name": "Teddie Bear", "phone_number": "0401 999 088"}), 200
		case "/v1/rex/contacts/read":
			return map[string]any{"id": 68313642, "etag": "e2", "related": map[string]any{"contact_names": []any{map[string]any{"id": 11}}, "contact_phones": []any{map[string]any{"id": 22}}}}, 200
		case "/v1/rex/contacts/update":
			updates = append(updates, b)
			return map[string]any{"id": 68313642, "etag": "e3"}, 200
		case "/v1/rex/contacts/create":
			t.Error("must not create a second Rex contact")
		}
		return nil, 500
	})
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 1) != "sent" || linkOf(a, "c") != "68313642" {
		t.Fatal(workerStatus(t, a, 1), linkOf(a, "c"))
	}
	var payload, status string
	_ = a.db.QueryRow(`SELECT payload,status FROM rex_outbox WHERE id=2`).Scan(&payload, &status)
	if payload != `{"rexAdopt":true}` || status != "pending" {
		t.Fatal(payload, status)
	}
	// Follow-up update writes the app's non-empty fields; a missing email row is added.
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 2) != "sent" || len(updates) != 1 {
		t.Fatal(workerStatus(t, a, 2), updates)
	}
	related := updates[0]["data"].(map[string]any)["related"].(map[string]any)
	email := related["contact_emails"].([]any)[0].(map[string]any)
	phone := related["contact_phones"].([]any)[0].(map[string]any)
	if email["email_address"] != "teddie@example.com" || email["id"] != nil || phone["id"] != float64(22) {
		t.Fatalf("adopt update: %v", related)
	}
}

func TestWorkerCreatesWhenRexHasNoMatch(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "+61 401 999 088", "")
	createJob(t, a, "c")
	creates := 0
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/find-matching-contacts":
			return rexFindResult(), 200
		case "/v1/rex/contacts/create":
			creates++
			return map[string]any{"id": 456}, 200
		}
		return nil, 500
	})
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if creates != 1 || linkOf(a, "c") != "456" {
		t.Fatal(creates, linkOf(a, "c"))
	}
}

func TestWorkerRetriesWhenRexCheckFails(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401999088", "")
	createJob(t, a, "c")
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		if path == "/v1/rex/contacts/create" {
			t.Error("must not create blindly")
		}
		return nil, 503
	})
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	var tries int
	var reason, next string
	_ = a.db.QueryRow(`SELECT tries,last_error,COALESCE(next_attempt_at,'') FROM rex_outbox WHERE id=1`).Scan(&tries, &reason, &next)
	if workerStatus(t, a, 1) != "pending" || tries != 1 || !strings.Contains(reason, "duplicate check failed") || next == "" || linkOf(a, "c") != "" {
		t.Fatal(workerStatus(t, a, 1), tries, reason, next)
	}
}

func TestWorkerRexMatchLinkedToOtherContact(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401 999 088", "")
	appContact(t, a, "other", "0499 000 000", "") // different local mobile; Rex holds both numbers
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','other','555','Contacts','2057','now')`); err != nil {
		t.Fatal(err)
	}
	createJob(t, a, "c")
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		if path == "/v1/rex/contacts/find-matching-contacts" {
			return rexFindResult(map[string]any{"id": 555, "name": "Teddie Bear"}), 200
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	})
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 1) != "skipped" || linkOf(a, "c") != "" {
		t.Fatal(workerStatus(t, a, 1), linkOf(a, "c"))
	}
	if d := dupOf(t, a, "c"); len(d) != 1 || d[0] != "other" {
		t.Fatal(d)
	}
	if d := dupOf(t, a, "other"); len(d) != 1 || d[0] != "c" {
		t.Fatal(d)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_links WHERE rex_id='555'`).Scan(&n)
	if n != 1 {
		t.Fatal("second link to one Rex record")
	}
}

func TestWorkerLocalDuplicateIsFlaggedNotDoubleLinked(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401 999 088", "")
	appContact(t, a, "other", "+61401999088", "")
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','other','555','Contacts','2057','now')`); err != nil {
		t.Fatal(err)
	}
	createJob(t, a, "c")
	w, calls := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) { t.Errorf("unexpected %s", path); return nil, 500 })
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	var rexID string
	_ = a.db.QueryRow(`SELECT COALESCE(json_extract(data,'$.rexId'),'') FROM contacts WHERE id='c'`).Scan(&rexID)
	if workerStatus(t, a, 1) != "skipped" || linkOf(a, "c") != "" || rexID != "" || len(*calls) != 0 {
		t.Fatal(workerStatus(t, a, 1), linkOf(a, "c"), rexID, *calls)
	}
	if d := dupOf(t, a, "c"); len(d) != 1 || d[0] != "other" {
		t.Fatal(d)
	}
}

func TestWorkerSeveralRexMatchesFail(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401 999 088", "")
	createJob(t, a, "c")
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		if path == "/v1/rex/contacts/find-matching-contacts" {
			return rexFindResult(map[string]any{"id": 1}, map[string]any{"id": 2}), 200
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	})
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if workerStatus(t, a, 1) != "failed" {
		t.Fatal(workerStatus(t, a, 1))
	}
}

func rexDetail(id, phone string, mod time.Time) importRow {
	r := importRow{"id": id, "etag": "e-" + id, "system_modtime": float64(mod.Unix()), "related": map[string]any{
		"contact_names":  []any{map[string]any{"name_first": "Rex", "name_last": "Name"}},
		"contact_phones": []any{map[string]any{"phone_number": phone, "phone_primary": true}},
		"contact_emails": []any{map[string]any{"email_address": "rex@example.com"}}}}
	flattenRexRead(r)
	return r
}

func TestPollerAdoptsUnlinkedLocalContact(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rexNews bool
	}{{"rex newer wins", true}, {"app newer wins", false}} {
		t.Run(tc.name, func(t *testing.T) {
			a := workerDB(t)
			appContact(t, a, "c", "0401999088", "")
			createJob(t, a, "c")
			mod := time.Now().Add(-time.Hour)
			if tc.rexNews {
				mod = time.Now().Add(time.Hour)
			}
			local, created, err := rexAdoptOrImport(a.db, "2057", rexDetail("68313642", "0401 999 088", mod), "u")
			if err != nil || created || local != "c" {
				t.Fatal(local, created, err)
			}
			if linkOf(a, "c") != "68313642" || workerStatus(t, a, 1) != "sent" {
				t.Fatal(linkOf(a, "c"), workerStatus(t, a, 1))
			}
			var first, email string
			var n, adopt int
			_ = a.db.QueryRow(`SELECT json_extract(data,'$.first'),json_extract(data,'$.email') FROM contacts WHERE id='c'`).Scan(&first, &email)
			_ = a.db.QueryRow(`SELECT count(*) FROM contacts`).Scan(&n)
			_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE payload='{"rexAdopt":true}' AND status='pending'`).Scan(&adopt)
			if n != 1 {
				t.Fatal("second local contact created")
			}
			if tc.rexNews && (first != "Rex" || email != "rex@example.com" || adopt != 0) {
				t.Fatal(first, email, adopt)
			}
			if !tc.rexNews && (first != "Teddie" || email != "" || adopt != 1) {
				t.Fatal(first, email, adopt)
			}
		})
	}
}

func TestPollerEmptyRexValuesDoNotOverwrite(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401999088", "keep@example.com")
	r := importRow{"id": "9", "system_modtime": float64(time.Now().Add(time.Hour).Unix()), "phone_number": "0401 999 088", "name_first": "", "email_address": ""}
	if _, _, err := rexAdoptOrImport(a.db, "2057", r, "u"); err != nil {
		t.Fatal(err)
	}
	var first, email string
	_ = a.db.QueryRow(`SELECT json_extract(data,'$.first'),json_extract(data,'$.email') FROM contacts WHERE id='c'`).Scan(&first, &email)
	if first != "Teddie" || email != "keep@example.com" || linkOf(a, "c") != "9" {
		t.Fatal(first, email)
	}
}

func TestPollerImportsAndFlagsWhenLocalLinkedElsewhere(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401 999 088", "")
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','c','111','Contacts','2057','now')`); err != nil {
		t.Fatal(err)
	}
	local, created, err := rexAdoptOrImport(a.db, "2057", rexDetail("222", "+61 401 999 088", time.Now()), "u")
	if err != nil || !created || local != "rex-contacts-2057-222" {
		t.Fatal(local, created, err)
	}
	if linkOf(a, "c") != "111" || linkOf(a, local) != "222" {
		t.Fatal("links changed")
	}
	if d := dupOf(t, a, "c"); len(d) != 1 || d[0] != local {
		t.Fatal(d)
	}
	if d := dupOf(t, a, local); len(d) != 1 || d[0] != "c" {
		t.Fatal(d)
	}
	// Re-import keeps the flag (app-side data field).
	if err := importSave(a.db, "contacts", "2057", rexDetail("222", "0401 999 088", time.Now()), "u", false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	if d := dupOf(t, a, local); len(d) != 1 {
		t.Fatal("flag lost on re-import")
	}
}

func TestPollerDoesNotAdoptSentCreate(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401999088", "")
	createJob(t, a, "c")
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='sent' WHERE id=1`); err != nil {
		t.Fatal(err)
	}
	local, created, err := rexAdoptOrImport(a.db, "2057", rexDetail("5", "0401999088", time.Now()), "u")
	if err != nil || !created || linkOf(a, "c") != "" || len(dupOf(t, a, local)) != 1 {
		t.Fatal(local, created, err)
	}
}

// End to end through pollRex on the single connection: a new Rex contact with an
// app contact's mobile is adopted, not imported twice.
func TestPollRexAdoptSingleConnection(t *testing.T) {
	a, _ := setup(t)
	if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	mary := userID(t, a, "mary@axleo.local")
	data := `{"id":"app-1","first":"Teddie","last":"Bear","mobile":"0401999088","mobileNorm":"0401999088"}`
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,data,created_at,updated_at) VALUES('app-1',?,'Teddie','Bear','0401999088',?,?,?)`, mary, data, now(), now()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REX_POLL_SOFT_DELETE", "0")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any = map[string]any{"rows": []any{}, "total": 0}
		switch r.URL.Path {
		case "/v1/rex/contacts/search":
			result = map[string]any{"rows": []any{map[string]any{"_id": "68313642", "etag": "e1"}}, "total": 1}
		case "/v1/rex/contacts/read":
			result = rexDetail("68313642", "0401 999 088", time.Now().Add(-time.Hour))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly, c.AccountID, c.Log = "mock", 0, true, "2057", rexDBLogger(a.db)
	lastRelationPoll = time.Now()
	appConnGuard(t, a, "poll adopt", func() error { return pollRex(context.Background(), a.db, c, "2057") })
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts`).Scan(&n)
	if n != 1 || linkOf(&app{db: a.db}, "app-1") != "68313642" {
		t.Fatal(n, linkOf(&app{db: a.db}, "app-1"))
	}
	var owner string
	_ = a.db.QueryRow(`SELECT owner_id FROM contacts WHERE id='app-1'`).Scan(&owner)
	if owner != mary {
		t.Fatal("adopted contact must keep its owner")
	}
}

func TestDuplicateRexLinksReport(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "a1", "", "")
	appContact(t, a, "a2", "", "")
	appContact(t, a, "a3", "", "")
	for _, q := range []string{
		`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','a1','9','Contacts','2057','now')`,
		`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','a2','9','Contacts','','now')`,
		`UPDATE contacts SET data=json_set(data,'$.rexId','9') WHERE id='a3'`,
	} {
		if _, err := a.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	var before string
	_ = a.db.QueryRow(`SELECT group_concat(version) FROM contacts`).Scan(&before)
	var dups []dupRexLink
	appConnGuard(t, a, "duplicate rex links", func() (err error) { dups, err = duplicateRexLinks(a.db); return })
	if len(dups) != 2 || dups[0].Via != "rex_links" || len(dups[0].LocalIDs) != 2 || dups[1].Via != "data.rexId" {
		t.Fatalf("%+v", dups)
	}
	var after string
	_ = a.db.QueryRow(`SELECT group_concat(version) FROM contacts`).Scan(&after)
	if before != after {
		t.Fatal("report must be read-only")
	}
}

func TestWorkerPreCreateSingleConnection(t *testing.T) {
	a := workerDB(t)
	appContact(t, a, "c", "0401 999 088", "")
	appContact(t, a, "d", "0402 000 000", "")
	createJob(t, a, "c")
	createJob(t, a, "d")
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/contacts/find-matching-contacts":
			if fmt.Sprint(b["phones"]) == "[0401 999 088]" {
				return rexFindResult(map[string]any{"id": 7}), 200
			}
			return rexFindResult(), 200
		case "/v1/rex/contacts/create":
			return map[string]any{"id": 8}, 200
		}
		return nil, 500
	})
	w.client.Log = rexDBLogger(a.db)
	appConnGuard(t, a, "worker pre-create", func() error {
		for i := 0; i < 2; i++ {
			if err := w.step(context.Background()); err != nil {
				return err
			}
		}
		return nil
	})
	if linkOf(a, "c") != "7" || linkOf(a, "d") != "8" {
		t.Fatal(linkOf(a, "c"), linkOf(a, "d"))
	}
}
