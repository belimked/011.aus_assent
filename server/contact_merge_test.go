package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func userID(t *testing.T, a *app, email string) string {
	t.Helper()
	var id string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email=?`, email).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

// mergeContactRow inserts a contact with controlled created/updated times.
func mergeContactRow(t *testing.T, a *app, id, owner, created, updated string, data map[string]any) {
	t.Helper()
	data["id"] = id
	b, _ := json.Marshal(data)
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,email,data,created_at,updated_at,roles,tags,flags,consent) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		append([]any{id, owner, str(data, "first"), str(data, "last"), str(data, "mobile"), str(data, "email"), string(b), created, updated}, contactColumns(data)...)...); err != nil {
		t.Fatal(err)
	}
}
func mergeProperty(t *testing.T, a *app, id, no, street string) {
	t.Helper()
	if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at) VALUES(?,?,?,'Sunnybank',?,?,?)`, id, no, street, fmt.Sprintf(`{"id":%q}`, id), now(), now()); err != nil {
		t.Fatal(err)
	}
}
func mergeOwner(t *testing.T, a *app, pid, cid, relation, at string) {
	t.Helper()
	if _, err := a.db.Exec(`INSERT INTO property_owners(property_id,contact_id,relation,updated_at) VALUES(?,?,?,?)`, pid, cid, relation, at); err != nil {
		t.Fatal(err)
	}
}
func previewMerge(t *testing.T, a *app, token, x, y string) map[string]any {
	t.Helper()
	w := request(a, "POST", "/api/v1/contacts/merge-preview", token, map[string]any{"aId": x, "bId": y})
	if w.Code != 200 {
		t.Fatalf("preview %d %s", w.Code, w.Body)
	}
	return obj(t, w)
}
func doMerge(t *testing.T, a *app, token string, preview map[string]any) map[string]any {
	t.Helper()
	w := request(a, "POST", "/api/v1/contacts/"+preview["primaryId"].(string)+"/merge", token, map[string]any{"secondaryId": preview["secondaryId"], "previewHash": preview["previewHash"]})
	if w.Code != 200 {
		t.Fatalf("merge %d %s", w.Code, w.Body)
	}
	return obj(t, w)
}
func contactData(t *testing.T, a *app, id string) map[string]any {
	t.Helper()
	var raw string
	if err := a.db.QueryRow(`SELECT data FROM contacts WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

const (
	t1 = "2026-01-01T00:00:00Z"
	t2 = "2026-02-01T00:00:00Z"
	t3 = "2026-03-01T00:00:00Z"
	t4 = "2026-04-01T00:00:00Z"
)

func TestMergeTimelineFieldsAndCompliance(t *testing.T) {
	a, tok := setup(t)
	mary := userID(t, a, "mary@axleo.local")
	// "old" was created first (primary); "new" was edited last (its values win).
	mergeContactRow(t, a, "old", mary, t1, t2, map[string]any{"first": "Jane", "last": "Doe", "mobile": "0400 000 001", "email": "jane@old.example", "address": "1 Old St",
		"flags": map[string]any{"dnc": true}, "consent": map[string]any{"sms": 0, "email": 1, "letter": 1}, "tags": []any{"Warm", " seller lead "}, "roles": []any{"owner"}})
	mergeContactRow(t, a, "new", mary, t2, t3, map[string]any{"first": "Janet", "last": "", "mobile": "0400 000 002", "email": "", "address": "2 New St", "reason": "Downsizing",
		"flags": map[string]any{"dnc": false}, "consent": map[string]any{"sms": 1, "email": 1, "letter": 0, "phone": 1}, "tags": []any{"warm", "Seller Lead", "Investor"}, "roles": []any{"Owner", "buyer"},
		"fieldTimes": map[string]any{"address": t1}})
	p := previewMerge(t, a, tok["mary"], "new", "old")
	if p["primaryId"] != "old" || p["primaryReason"] != "older" {
		t.Fatalf("primary %v %v", p["primaryId"], p["primaryReason"])
	}
	if !strings.HasPrefix(p["logNote"].(string), "Merged with Janet (0400 000 002) by Mary on ") {
		t.Fatalf("log note %q", p["logNote"])
	}
	doMerge(t, a, tok["mary"], p)
	c := contactData(t, a, "old")
	want := map[string]any{
		"first":   "Janet",            // newer wins
		"last":    "Doe",              // newer is empty: never overwrites
		"mobile":  "0400 000 002",     // newer wins
		"email":   "jane@old.example", // newer is empty
		"address": "1 Old St",         // field time (t1) is older than the other record's t2
		"reason":  "Downsizing",       // only one side
	}
	for k, v := range want {
		if c[k] != v {
			t.Errorf("%s=%v want %v", k, c[k], v)
		}
	}
	if imap(c["flags"])["dnc"] != true {
		t.Errorf("DNC lost: %v", c["flags"])
	}
	consent := imap(c["consent"])
	if consent["sms"] != float64(0) || consent["letter"] != float64(0) || consent["email"] != float64(1) || consent["phone"] != float64(1) || c["optedOut"] != true {
		t.Errorf("consent %v optedOut %v", consent, c["optedOut"])
	}
	if got := ijson(c["tags"]); got != `["warm","Seller Lead","Investor"]` {
		t.Errorf("tags %s", got)
	}
	if got := ijson(c["roles"]); got != `["owner","buyer"]` {
		t.Errorf("roles %s", got)
	}
	var first, mobile, flags string
	_ = a.db.QueryRow(`SELECT first,mobile,flags FROM contacts WHERE id='old'`).Scan(&first, &mobile, &flags)
	if first != "Janet" || mobile != "0400 000 002" || !strings.Contains(flags, `"dnc":true`) {
		t.Errorf("columns %s %s %s", first, mobile, flags)
	}
	var deleted string
	_ = a.db.QueryRow(`SELECT COALESCE(deleted_at,'') FROM contacts WHERE id='new'`).Scan(&deleted)
	if deleted == "" {
		t.Fatal("secondary not soft-deleted")
	}
	// Audit keeps both full snapshots; no undo endpoint exists.
	var diff string
	_ = a.db.QueryRow(`SELECT diff FROM audit_log WHERE action='merge' AND entity_id='old'`).Scan(&diff)
	if !strings.Contains(diff, `"before"`) || !strings.Contains(diff, "jane@old.example") || !strings.Contains(diff, "Downsizing") {
		t.Fatalf("audit %s", diff)
	}
}

func TestMergeNotesOrderedDedupedWithLog(t *testing.T) {
	a, tok := setup(t)
	mary := userID(t, a, "mary@axleo.local")
	mergeContactRow(t, a, "a", mary, t1, t2, map[string]any{"first": "Ann", "notes": []any{
		map[string]any{"d": "2026-03-05", "a": "Mary", "t": "Third"},
		map[string]any{"d": "2026-01-10", "a": "Mary", "t": "First"},
	}})
	mergeContactRow(t, a, "b", mary, t2, t3, map[string]any{"first": "Ann", "mobile": "0411 111 111", "notes": []any{
		map[string]any{"d": "2026-03-05", "a": "Mary", "t": "Third"}, // exact duplicate
		map[string]any{"d": "2026-02-01", "a": "John", "t": "Second"},
	}})
	if _, err := a.db.Exec(`INSERT INTO contact_notes(id,contact_id,user_id,d,a,t,created_at) VALUES('n1','b',?,'2026-02-01','John','Second','2026-02-01T09:00:00Z'),('n2','b',?,'2026-04-02','Mary','Fourth','2026-04-02T09:00:00Z')`, mary, mary); err != nil {
		t.Fatal(err)
	}
	p := previewMerge(t, a, tok["mary"], "a", "b")
	doMerge(t, a, tok["mary"], p)
	notes, _ := contactData(t, a, "a")["notes"].([]any)
	got := []string{}
	for _, n := range notes {
		got = append(got, str(imap(n), "t"))
	}
	if len(got) != 5 || !strings.HasPrefix(got[0], "Merged with Ann (0411 111 111) by Mary on ") || !strings.Contains(got[0], "由 Mary 于") || strings.Join(got[1:], ",") != "Fourth,Third,Second,First" {
		t.Fatalf("notes %q", got)
	}
	var moved, logs int
	_ = a.db.QueryRow(`SELECT count(*) FROM contact_notes WHERE contact_id='a'`).Scan(&moved)
	_ = a.db.QueryRow(`SELECT count(*) FROM contact_notes WHERE contact_id='a' AND t LIKE 'Merged with%'`).Scan(&logs)
	if moved != 3 || logs != 1 {
		t.Fatalf("contact_notes moved=%d logs=%d", moved, logs)
	}
}

func TestMergePropertyRelationsLatestWins(t *testing.T) {
	a, tok := setup(t)
	mary := userID(t, a, "mary@axleo.local")
	mergeContactRow(t, a, "a", mary, t1, t1, map[string]any{"first": "Ann", "rels": []any{map[string]any{"type": "Partner of", "name": "Bob"}}})
	mergeContactRow(t, a, "b", mary, t2, t2, map[string]any{"first": "Ann", "rels": []any{map[string]any{"type": "Partner of", "name": "Bob"}, map[string]any{"type": "Tenant of", "propertyId": "p4"}}})
	mergeContactRow(t, a, "friend", mary, t1, t1, map[string]any{"first": "Fred", "rels": []any{map[string]any{"type": "Partner of", "contactId": "b"}}})
	for _, x := range [][3]string{{"p1", "3", "Feltwell St"}, {"p2", "5", "Main Rd"}, {"p3", "7", "Side St"}, {"p4", "9", "Last Ave"}} {
		mergeProperty(t, a, x[0], x[1], x[2])
	}
	mergeOwner(t, a, "p1", "a", "tenant", t1) // conflict: B's owner (t3) is newer
	mergeOwner(t, a, "p1", "b", "owner", t3)
	mergeOwner(t, a, "p2", "a", "owner", t4) // conflict: A's owner (t4) is newer than B's tenant
	mergeOwner(t, a, "p2", "b", "tenant", t2)
	mergeOwner(t, a, "p3", "b", "owner", t2) // not conflicting: appended
	mergeOwner(t, a, "p4", "a", "owner", t1) // B's rels entry "Tenant of" p4 at B's updated_at (t2) wins
	p := previewMerge(t, a, tok["mary"], "a", "b")
	log := p["logNote"].(string)
	for _, s := range []string{"Relation to 3 Feltwell St, Sunnybank changed from Tenant to Owner by merge.", "Relation to 5 Main Rd, Sunnybank changed from Tenant to Owner by merge.", "Relation to 9 Last Ave, Sunnybank changed from Owner to Tenant by merge.", "因合并由 Tenant 改为 Owner"} {
		if !strings.Contains(log, s) {
			t.Errorf("log note missing %q in %q", s, log)
		}
	}
	doMerge(t, a, tok["mary"], p)
	rows, err := a.db.Query(`SELECT property_id,contact_id,relation,updated_at FROM property_owners ORDER BY property_id`)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for rows.Next() {
		var pid, cid, rel, at string
		_ = rows.Scan(&pid, &cid, &rel, &at)
		got = append(got, pid+":"+cid+":"+rel+":"+at[:10])
	}
	rows.Close()
	if strings.Join(got, " ") != "p1:a:owner:2026-03-01 p2:a:owner:2026-04-01 p3:a:owner:2026-02-01 p4:a:tenant:2026-02-01" {
		t.Fatalf("property_owners %v", got)
	}
	c := contactData(t, a, "a")
	if got := ijson(c["rels"]); got != `[{"name":"Bob","type":"Partner of"},{"propertyId":"p4","type":"Tenant of"}]` {
		t.Fatalf("rels %s", got)
	}
	if got := ijson(contactData(t, a, "friend")["rels"]); got != `[{"contactId":"a","type":"Partner of"}]` {
		t.Fatalf("friend rels %s", got)
	}
	// The phone reads ownerIds from property_owners.
	if ids := obj(t, request(a, "GET", "/api/v1/properties/p3", tok["mary"], nil))["ownerIds"].([]any); len(ids) != 1 || ids[0] != "a" {
		t.Fatal(ids)
	}
}

func TestMergePreviewHashAndPermissions(t *testing.T) {
	a, tok := setup(t)
	mary, john := userID(t, a, "mary@axleo.local"), userID(t, a, "john@axleo.local")
	mergeContactRow(t, a, "m1", mary, t1, t1, map[string]any{"first": "M1"})
	mergeContactRow(t, a, "m2", mary, t2, t2, map[string]any{"first": "M2"})
	mergeContactRow(t, a, "j1", john, t1, t1, map[string]any{"first": "J1"})
	// Agents may not merge (or preview) a record they cannot see.
	if w := request(a, "POST", "/api/v1/contacts/merge-preview", tok["mary"], map[string]any{"aId": "m1", "bId": "j1"}); w.Code != 404 {
		t.Fatalf("preview of another agent's contact: %d", w.Code)
	}
	if w := request(a, "POST", "/api/v1/contacts/m1/merge", tok["mary"], map[string]any{"secondaryId": "j1", "previewHash": "x"}); w.Code != 404 {
		t.Fatalf("merge of another agent's contact: %d", w.Code)
	}
	p := previewMerge(t, a, tok["mary"], "m1", "m2")
	// Missing hash: the user must have seen a preview.
	if w := request(a, "POST", "/api/v1/contacts/m1/merge", tok["mary"], map[string]any{"secondaryId": "m2"}); w.Code != 409 {
		t.Fatalf("merge without preview: %d", w.Code)
	}
	// The data changes after the preview: 409 with a fresh preview.
	if w := request(a, "PATCH", "/api/v1/contacts/m2", tok["mary"], map[string]any{"version": 1, "email": "m2@example.com"}); w.Code != 200 {
		t.Fatal(w.Body)
	}
	w := request(a, "POST", "/api/v1/contacts/m1/merge", tok["mary"], map[string]any{"secondaryId": "m2", "previewHash": p["previewHash"]})
	if w.Code != 409 {
		t.Fatalf("stale preview accepted: %d %s", w.Code, w.Body)
	}
	fresh := obj(t, w)["preview"].(map[string]any)
	if fresh["previewHash"] == p["previewHash"] {
		t.Fatal("409 did not return a new preview")
	}
	var alive int
	_ = a.db.QueryRow(`SELECT count(*) FROM contacts WHERE id IN ('m1','m2') AND deleted_at IS NULL`).Scan(&alive)
	if alive != 2 {
		t.Fatal("stale merge wrote")
	}
	// The agent merges directly: no admin step.
	out := doMerge(t, a, tok["mary"], fresh)
	if out["primaryId"] != "m1" || imap(out["contact"])["email"] != "m2@example.com" {
		t.Fatal(out)
	}
	var approvals int
	_ = a.db.QueryRow(`SELECT count(*) FROM approvals`).Scan(&approvals)
	if approvals != 0 {
		t.Fatal("merge created an approval")
	}
	// Admin may merge any two contacts.
	doMerge(t, a, tok["admi"], previewMerge(t, a, tok["admi"], "j1", "m1"))
	// Preview with the same data is stable (the date-bearing log note is not hashed).
	mergeContactRow(t, a, "m3", mary, t1, t1, map[string]any{"first": "M3"})
	mergeContactRow(t, a, "m4", mary, t2, t2, map[string]any{"first": "M4"})
	if previewMerge(t, a, tok["mary"], "m3", "m4")["previewHash"] != previewMerge(t, a, tok["mary"], "m4", "m3")["previewHash"] {
		t.Fatal("preview hash not stable")
	}
}

func TestMergeRelationMigrationBackfill(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(`CREATE TABLE schema_migrations(version TEXT PRIMARY KEY,applied_at TEXT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	entries, _ := assets.ReadDir("migrations")
	for _, e := range entries {
		if e.Name() >= "019" {
			continue
		}
		b, _ := assets.ReadFile("migrations/" + e.Name())
		if name := e.Name(); name == "010_console.sql" {
			_, _ = db.Exec(`PRAGMA foreign_keys=OFF`)
		}
		if _, err = db.Exec(string(b)); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		_, _ = db.Exec(`PRAGMA foreign_keys=ON`)
		_, _ = db.Exec(`INSERT INTO schema_migrations VALUES(?,?)`, e.Name(), now())
	}
	if err = createUser(db, "legacy@example.com", "Legacy", "agent", "password123"); err != nil {
		t.Fatal(err)
	}
	var uid string
	_ = db.QueryRow(`SELECT id FROM users WHERE email='legacy@example.com'`).Scan(&uid)
	for _, q := range []string{
		`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c1','` + uid + `','{}','2025-01-01T00:00:00Z','2025-06-01T00:00:00Z')`,
		`INSERT INTO properties(id,data,created_at,updated_at) VALUES('p1','{}','2025-01-01T00:00:00Z','2025-07-01T00:00:00Z')`,
		`INSERT INTO property_owners(property_id,contact_id,relation) VALUES('p1','c1','tenant')`,
	} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err = migrate(db, path); err != nil {
		t.Fatal(err)
	}
	var at string
	_ = db.QueryRow(`SELECT updated_at FROM property_owners WHERE property_id='p1'`).Scan(&at)
	if at != "2025-06-01T00:00:00Z" {
		t.Fatalf("backfill %q", at)
	}
	// New rows without a time are stamped; a relation change refreshes the time.
	if _, err = db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c2',?,'{}',?,?)`, uid, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO property_owners(property_id,contact_id) VALUES('p1','c2')`); err != nil {
		t.Fatal(err)
	}
	_ = db.QueryRow(`SELECT updated_at FROM property_owners WHERE contact_id='c2'`).Scan(&at)
	if !strings.HasPrefix(at, time.Now().UTC().Format("2006-01-02")) {
		t.Fatalf("insert stamp %q", at)
	}
	if _, err = db.Exec(`UPDATE property_owners SET relation='owner' WHERE contact_id='c1'`); err != nil {
		t.Fatal(err)
	}
	_ = db.QueryRow(`SELECT updated_at FROM property_owners WHERE contact_id='c1'`).Scan(&at)
	if !strings.HasPrefix(at, time.Now().UTC().Format("2006-01-02")) {
		t.Fatalf("update stamp %q", at)
	}
}

// A property PATCH from the phone (ownerIds only) keeps relation types and times.
func TestPropertyPatchKeepsRelationAndTime(t *testing.T) {
	a, tok := setup(t)
	mary := userID(t, a, "mary@axleo.local")
	mergeContactRow(t, a, "c1", mary, t1, t1, map[string]any{"first": "T"})
	mergeContactRow(t, a, "c2", mary, t1, t1, map[string]any{"first": "U"})
	if w := request(a, "POST", "/api/v1/properties", tok["mary"], map[string]any{"id": "pp", "ownerIds": []string{"c1"}}); w.Code != 201 {
		t.Fatal(w.Body)
	}
	if _, err := a.db.Exec(`UPDATE property_owners SET relation='tenant',updated_at=? WHERE contact_id='c1'`, t2); err != nil {
		t.Fatal(err)
	}
	v := obj(t, request(a, "GET", "/api/v1/properties/pp", tok["mary"], nil))["version"]
	if w := request(a, "PATCH", "/api/v1/properties/pp", tok["mary"], map[string]any{"version": v, "ownerIds": []string{"c1", "c2"}}); w.Code != 200 {
		t.Fatal(w.Body)
	}
	var rel, at string
	_ = a.db.QueryRow(`SELECT relation,updated_at FROM property_owners WHERE contact_id='c1'`).Scan(&rel, &at)
	if rel != "tenant" || at != t2 {
		t.Fatalf("relation reset: %s %s", rel, at)
	}
}

// --- Rex ---

func rexMergeSetup(t *testing.T, aRex, bRex string) *app {
	t.Helper()
	a := workerDB(t)
	for _, c := range []struct{ id, created, rex, mobile string }{{"a", t1, aRex, "0400000001"}, {"b", t2, bRex, "0400000002"}} {
		mergeContactRow(t, a, c.id, "u", c.created, c.created, map[string]any{"first": "Pat", "last": strings.ToUpper(c.id), "mobile": c.mobile, "source": "rex_import", "consent": map[string]any{"sms": 1}})
		if c.rex != "" {
			if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts',?,?,'Contacts','2057',?)`, c.id, c.rex, now()); err != nil {
				t.Fatal(err)
			}
		}
	}
	// b is DNC and refused SMS; a tenant at p1 later became b's owner relation.
	if _, err := a.db.Exec(`UPDATE contacts SET data=json_set(data,'$.flags',json('{"dnc":true}'),'$.consent',json('{"sms":0}')) WHERE id='b'`); err != nil {
		t.Fatal(err)
	}
	mergeProperty(t, a, "p1", "3", "Feltwell St")
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('properties','p1','501','Properties','2057',?)`, now()); err != nil {
		t.Fatal(err)
	}
	mergeOwner(t, a, "p1", "a", "tenant", t1)
	mergeOwner(t, a, "p1", "b", "owner", t3)
	return a
}
func runMerge(t *testing.T, a *app) *mergePlan {
	t.Helper()
	var plan *mergePlan
	singleConnGuard(t, a.db, "merge execute", func() error {
		tx, err := a.db.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		plan, err = a.executeMerge(tx, principal{id: "u", name: "Mary", role: "admin", device: "test"}, "a", "b", "")
		if err != nil {
			return err
		}
		return tx.Commit()
	})
	return plan
}
func outboxJobs(t *testing.T, a *app) []string {
	t.Helper()
	rows, err := a.db.Query(`SELECT entity,entity_id,status FROM rex_outbox ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var e, id, s string
		_ = rows.Scan(&e, &id, &s)
		out = append(out, e+"|"+id+"|"+s)
	}
	return out
}

// mergeWorker runs the worker against mergeRex (rexmerge_fix_test.go) with the
// given contact_reln_property rows on property 501.
func mergeWorker(t *testing.T, a *app, relations []any) (*rexWorker, *mergeRex) {
	t.Helper()
	f := newMergeRex(t)
	for _, r := range relations {
		f.props["501"] = append(f.props["501"], imap(r))
	}
	return mergeRexWorker(t, a, f), f
}
func drain(t *testing.T, a *app, w *rexWorker, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		singleConnGuard(t, a.db, "worker step", func() error { return w.step(context.Background()) })
	}
}

func TestRexMergeBothLinked(t *testing.T) {
	a := rexMergeSetup(t, "91", "92")
	plan := runMerge(t, a)
	if plan.PrimaryID != "a" || plan.Rex != "both" {
		t.Fatal(plan.PrimaryID, plan.Rex)
	}
	// Only the merge job: no contact update can reach Rex before combineRecords.
	if jobs := outboxJobs(t, a); len(jobs) != 1 || jobs[0] != "rex_merge|merge:a:b|waiting_auth" {
		t.Fatal(jobs)
	}
	w, m := mergeWorker(t, a, []any{
		map[string]any{"id": 801, "contact": map[string]any{"id": 91}, "reln_type": map[string]any{"id": "tenant"}},
		map[string]any{"id": 802, "contact": map[string]any{"id": 92}, "reln_type": map[string]any{"id": "owner"}},
		map[string]any{"id": 803, "contact": map[string]any{"id": 55}, "reln_type": map[string]any{"id": "tenant"}},
	})
	drain(t, a, w, 6)
	combine := m.bodies["/v1/rex/dedupe/combine-records"]
	if len(combine) != 1 || ijson(combine[0]) != `{"losing_ids":["92"],"service_name":"Contacts","winning_id":"91"}` {
		t.Fatalf("combine %v", combine)
	}
	jobs := outboxJobs(t, a)
	want := []string{"rex_merge|merge:a:b|sent", "contacts|a|sent", "rex_contact_dnd|a|sent", "rex_merge_relation|a:p1|sent"}
	for i, j := range want {
		if i >= len(jobs) || jobs[i] != j {
			t.Fatalf("jobs %v", jobs)
		}
	}
	if len(jobs) != 5 || !strings.HasPrefix(jobs[4], "contacts|") || !strings.HasSuffix(jobs[4], "|sent") {
		t.Fatalf("note job %v", jobs)
	}
	upd := imap(m.bodies["/v1/rex/contacts/update"][0]["data"])
	if upd["id"] != "91" || upd["contact_by_sms"] != float64(0) || ijson(imap(upd["related"])["contact_phones"]) != `[{"phone_number":"0400000002","phone_primary":true,"phone_type":"mobile"}]` {
		t.Fatalf("contact update %v", upd)
	}
	var dnd map[string]any
	for _, b := range m.bodies["/v1/rex/contacts/update"] {
		if _, ok := imap(b["data"])["is_dnd"]; ok {
			dnd = imap(b["data"])
		}
	}
	if dnd == nil || dnd["is_dnd"] != true || dnd["id"] != "91" {
		t.Fatalf("dnd %v", m.bodies["/v1/rex/contacts/update"])
	}
	// combineRecords left the winner's row 801 with the loser's type (owner), which
	// is the merge result: the relation job only reads. The other contact's row stays.
	if prop := m.bodies["/v1/rex/properties/update"]; len(prop) != 0 {
		t.Fatalf("property update %v", prop)
	}
	if r := m.rows("501"); len(r) != 2 || r["801"] != "91:owner" || r["803"] != "55:tenant" {
		t.Fatalf("rows %v", r)
	}
	note := m.bodies["/v1/rex/notes/create"]
	if len(note) != 1 || !strings.Contains(ijson(note[0]), "Merged with Pat B (0400000002) by Mary on") || !strings.Contains(ijson(note[0]), `"note_contacts":[{"contact":{"id":"91"}}]`) {
		t.Fatalf("note %v", note)
	}
}

func TestRexMergeOnlyPrimaryLinked(t *testing.T) {
	a := rexMergeSetup(t, "", "92") // b is in Rex, so b becomes the primary
	plan := runMerge(t, a)
	if plan.PrimaryID != "b" || plan.PrimaryReason != "rex" || plan.Rex != "primary" {
		t.Fatal(plan.PrimaryID, plan.PrimaryReason, plan.Rex)
	}
	w, m := mergeWorker(t, a, []any{map[string]any{"id": 802, "contact": map[string]any{"id": 92}, "reln_type": map[string]any{"id": "owner"}}})
	drain(t, a, w, 6)
	if len(m.bodies["/v1/rex/dedupe/combine-records"]) != 0 {
		t.Fatal("combine called without a losing Rex record")
	}
	if len(m.bodies["/v1/rex/contacts/update"]) == 0 || imap(m.bodies["/v1/rex/contacts/update"][0]["data"])["id"] != "92" {
		t.Fatalf("update %v", m.bodies)
	}
	// b already owned p1 (newest): the relation job only cleans up and writes nothing.
	if len(m.bodies["/v1/rex/properties/update"]) != 0 {
		t.Fatal(m.bodies["/v1/rex/properties/update"])
	}
}

func TestRexMergeOnlySecondaryLinkedAdopts(t *testing.T) {
	a := rexMergeSetup(t, "", "")
	// b's Rex create is still in flight at merge time.
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at,next_attempt_at) VALUES('contacts','b','POST','{}','pending',?,?)`, now(), "2999-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	plan := runMerge(t, a)
	if plan.PrimaryID != "a" || plan.Rex != "pending" {
		t.Fatal(plan.PrimaryID, plan.Rex)
	}
	// The create completes for the (now merged-away) secondary.
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','b','93','Contacts','2057',?)`, now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='sent' WHERE entity='contacts' AND entity_id='b'`); err != nil {
		t.Fatal(err)
	}
	w, m := mergeWorker(t, a, []any{map[string]any{"id": 900, "contact": map[string]any{"id": 93}, "reln_type": map[string]any{"id": "owner"}}})
	drain(t, a, w, 6)
	if len(m.bodies["/v1/rex/dedupe/combine-records"]) != 0 {
		t.Fatal("combine called")
	}
	var local string
	_ = a.db.QueryRow(`SELECT local_id FROM rex_links WHERE entity='contacts' AND rex_id='93'`).Scan(&local)
	if local != "a" || str(contactData(t, a, "a"), "rexId") != "93" {
		t.Fatalf("link not transferred: %s %v", local, contactData(t, a, "a")["rexId"])
	}
	if len(m.bodies["/v1/rex/contacts/update"]) == 0 || imap(m.bodies["/v1/rex/contacts/update"][0]["data"])["id"] != "93" {
		t.Fatalf("update %v", m.bodies)
	}
}

func TestRexMergeNeitherLinkedNoJob(t *testing.T) {
	a := rexMergeSetup(t, "", "")
	if plan := runMerge(t, a); plan.Rex != "none" {
		t.Fatal(plan.Rex)
	}
	if jobs := outboxJobs(t, a); len(jobs) != 0 {
		t.Fatal(jobs)
	}
}

func TestRexMergeWaitIsBounded(t *testing.T) {
	a := rexMergeSetup(t, "", "")
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,status,created_at,next_attempt_at) VALUES('contacts','b','POST','{}','pending',?,?)`, now(), "2999-01-01T00:00:00Z"); err != nil {
		t.Fatal(err)
	}
	runMerge(t, a)
	w, m := mergeWorker(t, a, nil)
	drain(t, a, w, 1)
	var status, reason string
	_ = a.db.QueryRow(`SELECT status,COALESCE(last_error,'') FROM rex_outbox WHERE entity='rex_merge'`).Scan(&status, &reason)
	if status != "pending" || reason != "waiting for contact link" {
		t.Fatal(status, reason)
	}
	// Deadline passes with the create still pending: finish, never wait forever.
	if _, err := a.db.Exec(`UPDATE rex_outbox SET payload=json_set(payload,'$.deadline','2000-01-01T00:00:00Z'),next_attempt_at=NULL WHERE entity='rex_merge'`); err != nil {
		t.Fatal(err)
	}
	drain(t, a, w, 1)
	_ = a.db.QueryRow(`SELECT status,COALESCE(last_error,'') FROM rex_outbox WHERE entity='rex_merge'`).Scan(&status, &reason)
	if status != "skipped" || reason != "timed out waiting for the Rex contact link" {
		t.Fatal(status, reason)
	}
	// A legacy job (no deadline) uses its queue time.
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_merge','legacy','POST','{"primaryId":"a","secondaryId":"zz"}','2000-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	drain(t, a, w, 1)
	_ = a.db.QueryRow(`SELECT status FROM rex_outbox WHERE entity_id='legacy'`).Scan(&status)
	if status != "skipped" {
		t.Fatal(status)
	}
	if len(m.bodies) != 0 {
		t.Fatal(m.bodies)
	}
}

func TestMergeSingleConnection(t *testing.T) {
	a, tok := setup(t)
	mary := userID(t, a, "mary@axleo.local")
	mergeContactRow(t, a, "a", mary, t1, t1, map[string]any{"first": "A", "notes": []any{map[string]any{"d": "2026-01-01", "a": "Mary", "t": "n"}}})
	mergeContactRow(t, a, "b", mary, t2, t2, map[string]any{"first": "B"})
	mergeProperty(t, a, "p", "1", "St")
	mergeOwner(t, a, "p", "b", "owner", t2)
	_ = request(a, "POST", "/api/v1/todos", tok["mary"], map[string]any{"id": "td", "cid": "b", "t": "Call"})
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,account_id,created_at) VALUES('contacts','a','1','Contacts','2057',?)`, now()); err != nil {
		t.Fatal(err)
	}
	singleConnGuard(t, a.db, "merge preview+execute", func() error {
		w := request(a, "POST", "/api/v1/contacts/merge-preview", tok["mary"], map[string]any{"aId": "a", "bId": "b"})
		if w.Code != 200 {
			return fmt.Errorf("preview %d %s", w.Code, w.Body)
		}
		var p map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &p)
		w = request(a, "POST", "/api/v1/contacts/a/merge", tok["mary"], map[string]any{"secondaryId": "b", "previewHash": p["previewHash"]})
		if w.Code != 200 {
			return fmt.Errorf("merge %d %s", w.Code, w.Body)
		}
		w = request(a, "POST", "/api/v1/contacts/a/merge", tok["mary"], map[string]any{"secondaryId": "b", "previewHash": p["previewHash"]})
		if w.Code != 404 {
			return fmt.Errorf("second merge %d", w.Code)
		}
		return nil
	})
	var cid string
	_ = a.db.QueryRow(`SELECT json_extract(data,'$.cid') FROM todos WHERE id='td'`).Scan(&cid)
	if cid != "a" {
		t.Fatal(cid)
	}
}
