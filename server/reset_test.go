package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// resetFixture builds a migrated, seeded database with the data reset must keep
// (accounts, settings, a team, an admin zone, a Pricefinder import linked to
// both a demo parcel and an app-created property).
func resetFixture(t *testing.T) (*app, string, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "axleo.db")
	t.Setenv("AXLEO_DB", path)
	t.Setenv("AXLEO_BACKUP_DIR", filepath.Join(dir, "backups"))
	t.Setenv("AXLEO_PHOTO_DIR", filepath.Join(dir, "photos"))
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = migrate(db, path); err != nil {
		t.Fatal(err)
	}
	for _, u := range [][3]string{{"admin@axleo.local", "Admin", "admin"}, {"mary@axleo.local", "Mary", "agent"}, {"john@axleo.local", "John", "agent"}} {
		if err = createUser(db, u[0], u[1], u[2], "password123"); err != nil {
			t.Fatal(err)
		}
	}
	if err = seedData(db, true); err != nil {
		t.Fatal(err)
	}
	a := &app{db: db, rdb: testReaders(t, path), failures: map[string][]time.Time{}}
	tokens := map[string]string{}
	for _, email := range []string{"admin@axleo.local", "mary@axleo.local"} {
		tokens[email] = resetLogin(t, a, email)
	}
	var mary string
	if err = db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	// App-created property that Pricefinder later matched: must survive for the FK.
	if w := request(a, "POST", "/api/v1/properties", tokens["mary@axleo.local"], map[string]any{"id": "parcel-pf", "no": "9", "street": "Pf St", "lot": "9RP1"}); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	if w := request(a, "POST", "/api/v1/admin/zones", tokens["admin@axleo.local"], map[string]any{"ownerId": mary, "name": "Admin drawn", "pts": []any{[]any{1, 1}, []any{1, 2}, []any{2, 2}}}); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		`INSERT INTO teams(id,name) VALUES('east','East')`,
		`INSERT INTO settings(key,value) VALUES('reset-test','preserved')`,
		`INSERT INTO market_imports(id,source,provider,filename,sha256,imported_by,created_at,status) VALUES('imp','file','pricefinder','test.csv','hash','` + mary + `','now','completed')`,
		`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,import_id,property_id,raw) VALUES('ev','pricefinder','ev','sale','2 Gager St','2gagerst','imp','p1','{}')`,
		`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,import_id,property_id,raw) VALUES('ev2','pricefinder','ev2','sale','9 Pf St','9pfst','imp','parcel-pf','{}')`,
	} {
		if _, err = tx.Exec(s); err != nil {
			t.Fatal(s, err)
		}
	}
	var parcel string
	if err = tx.QueryRow(`SELECT id FROM properties WHERE json_extract(data,'$.lotplan') IS NOT NULL ORDER BY id LIMIT 1`).Scan(&parcel); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{parcel, "parcel-pf"} {
		if err = applyMarketOwnership(tx, id, "PF Owner", 900000, "2020-01-01", "pricefinder", "imp", false); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	return a, path, tokens
}

func resetLogin(t *testing.T, a *app, email string) string {
	t.Helper()
	w := request(a, "POST", "/api/v1/auth/login", "", map[string]any{"email": email, "password": "password123", "device": "reset-test"})
	if w.Code != 200 {
		t.Fatal(email, w.Code, w.Body)
	}
	return obj(t, w)["token"].(string)
}

func tableCounts(t *testing.T, db *sql.DB) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for rows.Next() {
		var n string
		_ = rows.Scan(&n)
		names = append(names, n)
	}
	rows.Close()
	for _, n := range names {
		var c int64
		if err = db.QueryRow(`SELECT count(*) FROM ` + n).Scan(&c); err != nil {
			t.Fatal(err)
		}
		out[n] = c
	}
	return out
}

func dump(t *testing.T, db *sql.DB, q string) []string {
	t.Helper()
	rows, err := db.Query(q)
	if err != nil {
		t.Fatal(q, err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []string{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err = rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(vals)
		out = append(out, string(b))
	}
	return out
}

// pullState reduces a sync pull to comparable content, dropping volatile fields.
func pullState(t *testing.T, pull map[string]any) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, kind := range syncKinds {
		items, _ := pull[kind].([]any)
		list := []string{}
		for _, it := range items {
			m := it.(map[string]any)
			c := map[string]any{}
			for k, v := range m {
				switch k {
				case "version", "updated_at", "created_at", "updatedAt", "createdAt":
				default:
					c[k] = v
				}
			}
			b, _ := json.Marshal(c)
			list = append(list, string(b))
		}
		sort.Strings(list)
		out[kind] = list
	}
	return out
}

func TestResetData(t *testing.T) {
	a, path, tokens := resetFixture(t)
	db := a.db
	mary, admin := tokens["mary@axleo.local"], tokens["admin@axleo.local"]
	// The expected post-reset state: seed plus the data that must be kept.
	expected := tableCounts(t, db)
	keptRows := map[string][]string{}
	for table, q := range map[string]string{
		"users":              `SELECT id,email,name,role,password_hash,team_id,disabled,version FROM users ORDER BY id`,
		"teams":              `SELECT * FROM teams ORDER BY id`,
		"settings":           `SELECT * FROM settings ORDER BY key`,
		"zones":              `SELECT id,owner_id,data,version,deleted_at FROM zones ORDER BY id`,
		"market_imports":     `SELECT * FROM market_imports ORDER BY id`,
		"market_events":      `SELECT * FROM market_events ORDER BY id`,
		"property_ownership": `SELECT * FROM property_ownership ORDER BY property_id`,
		"properties":         `SELECT id,no,street,suburb,geometry,pf_owner,last_sale,flags,data,registered,deleted_at FROM properties WHERE id<>'parcel-pf' ORDER BY id`,
	} {
		keptRows[table] = dump(t, db, q)
	}
	freshMary := pullState(t, obj(t, request(a, "GET", "/api/v1/sync", mary, nil)))
	freshAdmin := pullState(t, obj(t, request(a, "GET", "/api/v1/sync", admin, nil)))
	oldPull := obj(t, request(a, "GET", "/api/v1/sync", mary, nil))
	oldCursor := oldPull["cursor"].(string)

	// Phone/test activity that reset must undo.
	var c1 map[string]any
	for _, it := range oldPull["contacts"].([]any) {
		if m := it.(map[string]any); m["id"] == "c1" {
			c1 = m
		}
	}
	if c1 == nil {
		t.Fatal("seed contact c1 not visible to Mary")
	}
	var p1 map[string]any
	for _, it := range oldPull["properties"].([]any) {
		if m := it.(map[string]any); m["id"] == "p1" {
			p1 = m
		}
	}
	var pfVersion int
	if err := db.QueryRow(`SELECT version FROM properties WHERE id='parcel-pf'`).Scan(&pfVersion); err != nil {
		t.Fatal(err)
	}
	steps := []struct {
		method, path string
		body         map[string]any
		code         int
	}{
		{"POST", "/api/v1/contacts", map[string]any{"id": "app-contact", "first": "Test"}, 201},
		{"POST", "/api/v1/todos", map[string]any{"id": "app-todo", "t": "Test"}, 201},
		{"POST", "/api/v1/properties", map[string]any{"id": "parcel-123", "no": "1", "street": "Phone St", "lot": "1RP2", "ownerIds": []any{"app-contact"}}, 201},
		{"POST", "/api/v1/captures", map[string]any{"id": "app-capture", "t": "x"}, 201},
		{"POST", "/api/v1/tags", map[string]any{"id": "app-tag", "name": "Mine"}, 201},
		{"PATCH", "/api/v1/properties/p1", map[string]any{"version": p1["version"], "no": "2", "street": "Gager St", "suburb": "Sunnybank", "visits": []any{"knocked"}, "pfOwner": "x"}, 200},
		{"PATCH", "/api/v1/properties/parcel-pf", map[string]any{"version": pfVersion, "no": "9", "street": "Pf St", "lot": "9RP1", "visits": []any{"x"}}, 200},
		{"DELETE", "/api/v1/contacts/c1", map[string]any{"version": c1["version"]}, 200},
	}
	for _, s := range steps {
		if w := request(a, s.method, s.path, mary, s.body); w.Code != s.code {
			t.Fatal(s.method, s.path, w.Code, w.Body)
		}
	}
	if w := request(a, "POST", "/api/v1/sync/push", mary, map[string]any{"ops": []any{map[string]any{"opId": "op1", "kind": "contacts", "op": "upsert", "id": "app-contact-2", "data": map[string]any{"first": "Pushed"}}}}); w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	for _, s := range []string{
		`INSERT INTO contact_notes(id,contact_id,t,created_at) VALUES('app-note','app-contact','note','now')`,
		`INSERT INTO photos(id,property_id,created_by,mime,size,created_at) VALUES('photo1','p1',(SELECT id FROM users WHERE email='mary@axleo.local'),'image/png',1,'now')`,
		`INSERT INTO ofi_events(id,listing_id) VALUES('app-ofi','l1')`,
		`INSERT INTO approvals(id,kind,status) VALUES('app-approval','merge','pending')`,
		`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts','app-contact','create','{}','now')`,
		`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at) VALUES('contacts','app-contact','1','Contacts','now')`,
		`INSERT INTO activity(id,owner_id,data) VALUES('app-activity',(SELECT id FROM users WHERE email='mary@axleo.local'),'{}')`,
		`INSERT INTO owner_name_views(id,team_id,user_id,property_id,source,at) VALUES('app-view','default',(SELECT id FROM users WHERE email='mary@axleo.local'),'p1','rpdata','now')`,
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatal(s, err)
		}
	}
	if err := os.MkdirAll(photoDir(), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(photoDir(), "photo1.png"), []byte("png"), 0600); err != nil {
		t.Fatal(err)
	}
	var stale int
	if err := db.QueryRow(`SELECT version FROM contacts WHERE id='app-contact'`).Scan(&stale); err != nil {
		t.Fatal(err)
	}

	// Dry run changes nothing, including photo files and backups.
	dirty := tableCounts(t, db)
	dirtyProps := dump(t, db, `SELECT * FROM properties ORDER BY id`)
	if err := resetData(db, path, false, true); err != nil {
		t.Fatal(err)
	}
	if got := tableCounts(t, db); !reflect.DeepEqual(got, dirty) {
		t.Fatalf("dry run changed counts:\n%v\n%v", dirty, got)
	}
	if !reflect.DeepEqual(dump(t, db, `SELECT * FROM properties ORDER BY id`), dirtyProps) {
		t.Fatal("dry run changed properties")
	}
	if _, err := os.Stat(filepath.Join(photoDir(), "photo1.png")); err != nil {
		t.Fatal("dry run removed photo", err)
	}
	if _, err := os.Stat(os.Getenv("AXLEO_BACKUP_DIR")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote a backup", err)
	}

	if err := resetData(db, path, true, true); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(os.Getenv("AXLEO_BACKUP_DIR"), "axleo-*.db"))
	if len(backups) != 1 {
		t.Fatal("expected one verified backup", backups)
	}
	if counts, err := verify(backups[0]); err != nil || counts["contacts"] != dirty["contacts"] {
		t.Fatal("backup is not the pre-reset state", err)
	}
	after := tableCounts(t, db)
	for table, n := range expected {
		switch table {
		case "sessions", "audit_log", "sync_changes", "sync_ops", "schema_migrations":
			continue
		case "settings":
			n++ // sync_reset_seq
		case "properties":
			// parcel-pf (Pricefinder-linked) is kept, parcel-123 deleted.
		}
		if after[table] != n {
			t.Errorf("%s: want %d, got %d", table, n, after[table])
		}
	}
	for _, table := range []string{"sessions", "audit_log", "sync_ops", "photos", "ofi_events", "approvals", "rex_outbox", "rex_links", "activity", "owner_name_views"} {
		if after[table] != 0 {
			t.Errorf("%s not cleared: %d", table, after[table])
		}
	}
	for table, rows := range keptRows {
		q := map[string]string{
			"users":              `SELECT id,email,name,role,password_hash,team_id,disabled,version FROM users ORDER BY id`,
			"teams":              `SELECT * FROM teams ORDER BY id`,
			"settings":           `SELECT * FROM settings WHERE key<>'` + syncResetKey + `' ORDER BY key`,
			"zones":              `SELECT id,owner_id,data,version,deleted_at FROM zones ORDER BY id`,
			"market_imports":     `SELECT * FROM market_imports ORDER BY id`,
			"market_events":      `SELECT * FROM market_events ORDER BY id`,
			"property_ownership": `SELECT * FROM property_ownership ORDER BY property_id`,
			"properties":         `SELECT id,no,street,suburb,geometry,pf_owner,last_sale,flags,data,registered,deleted_at FROM properties WHERE id<>'parcel-pf' ORDER BY id`,
		}[table]
		if got := dump(t, db, q); !reflect.DeepEqual(got, rows) {
			t.Errorf("%s differs from the seed/kept state", table)
		}
	}
	var pf string
	var registered int
	if err := db.QueryRow(`SELECT data,registered FROM properties WHERE id='parcel-pf'`).Scan(&pf, &registered); err != nil || registered != 1 {
		t.Fatal("Pricefinder-linked app property", err, registered)
	}
	var data map[string]any
	_ = json.Unmarshal([]byte(pf), &data)
	if data["visits"] != nil {
		t.Fatal("phone fields kept on Pricefinder-linked property", pf)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM properties WHERE id='parcel-123'`).Scan(&n); err != nil || n != 0 {
		t.Fatal("app-created property survived", n, err)
	}
	if err := db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&n); err != nil || n != 0 {
		t.Fatal("foreign keys", n, err)
	}
	if _, err := os.Stat(filepath.Join(photoDir(), "photo1.png")); !os.IsNotExist(err) {
		t.Fatal("photo file survived", err)
	}

	// Old tokens are revoked; a fresh login pulls exactly the seed state.
	if w := request(a, "GET", "/api/v1/sync", mary, nil); w.Code != 401 {
		t.Fatal("old session", w.Code)
	}
	mary, admin = resetLogin(t, a, "mary@axleo.local"), resetLogin(t, a, "admin@axleo.local")
	pull := obj(t, request(a, "GET", "/api/v1/sync", mary, nil))
	if pull["full"] != true || !reflect.DeepEqual(pullState(t, pull), freshMary) {
		t.Fatalf("Mary's fresh pull differs from seed:\n%v\n%v", pullState(t, pull), freshMary)
	}
	if got := pullState(t, obj(t, request(a, "GET", "/api/v1/sync", admin, nil))); !reflect.DeepEqual(got, freshAdmin) {
		t.Fatal("admin's fresh pull differs from seed")
	}
	// A pre-reset cursor gets a full snapshot rather than a partial delta.
	old := obj(t, request(a, "GET", "/api/v1/sync?since="+oldCursor, mary, nil))
	if old["full"] != true || !reflect.DeepEqual(pullState(t, old), freshMary) {
		t.Fatal("pre-reset cursor did not get a full resync", old["full"])
	}
	// A post-reset cursor stays incremental.
	inc := obj(t, request(a, "GET", "/api/v1/sync?since="+pull["cursor"].(string), mary, nil))
	if inc["full"] != false || len(inc["contacts"].([]any)) != 0 {
		t.Fatal("post-reset cursor", inc["full"])
	}
	if _, err := base64.RawURLEncoding.DecodeString(oldCursor); err != nil {
		t.Fatal(err)
	}
	// Stale queued writes are rejected by version checks instead of resurrecting data.
	push := obj(t, request(a, "POST", "/api/v1/sync/push", mary, map[string]any{"ops": []any{
		map[string]any{"opId": "s1", "kind": "contacts", "op": "upsert", "id": "app-contact", "baseVersion": stale, "data": map[string]any{"first": "Stale"}},
		map[string]any{"opId": "s2", "kind": "properties", "op": "upsert", "id": "p1", "baseVersion": p1["version"], "data": map[string]any{"visits": []any{"stale"}}},
		map[string]any{"opId": "s3", "kind": "contacts", "op": "delete", "id": "app-contact", "baseVersion": stale},
	}}))
	for i, want := range []string{"notfound", "conflict", "notfound"} {
		if got := push["results"].([]any)[i].(map[string]any)["status"]; got != want {
			t.Errorf("stale op %d: want %s, got %v", i, want, got)
		}
	}
}

func TestResetDataRequiresMigratedSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.db")
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = resetData(db, path, true, true); err == nil {
		t.Fatal("reset of an unmigrated database must fail")
	}
}
