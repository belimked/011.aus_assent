package main

import (
	"encoding/base64"
	"sort"
	"strings"
	"testing"
)

// scopeFixture inserts public properties (owner_id NULL) covering every branch
// of propertySyncSQL and returns the ids phones must receive.
func scopeFixture(t *testing.T, a *app) (mary string, want []string) {
	t.Helper()
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := a.db.Exec(q, args...); err != nil {
			t.Fatal(q, err)
		}
	}
	add := func(id string, registered int, extra string) {
		t.Helper()
		exec(`INSERT INTO properties(id,no,street,suburb,data,owner_id,registered,created_at,updated_at) VALUES(?,'1','Scope St','Sunnybank',?,NULL,?,?,?)`,
			id, `{"id":"`+id+`","ll":[-27.57,153.06]`+extra+`}`, registered, now(), now())
	}
	qld := `,"source":"qld_parcels","lotplan":"1RP1"`
	// a) Rex
	add("rex", 1, qld)
	exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('properties','rex','r1','Properties',?, '1')`, now())
	// b) market data
	add("ownership", 1, qld)
	exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES('imp','file','pricefinder','x','x',?,'completed')`, now())
	exec(`INSERT INTO property_ownership(property_id,owner_names,source,import_id,updated_at) VALUES('ownership','A Owner','pricefinder','imp',?)`, now())
	add("pf", 1, qld+`,"pricefinderAttributes":{"beds":"3"}`)
	add("rp", 1, qld+`,"rpdataAttributes":{"beds":"3"}`)
	// c) app content
	exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c1',?,'{"id":"c1"}',?,?)`, mary, now(), now())
	add("owned", 1, qld)
	exec(`INSERT INTO property_owners(property_id,contact_id) VALUES('owned','c1')`)
	add("notes", 1, qld+`,"notes":[{"d":"2026-01-01","a":"Mary","t":"Knocked"}]`)
	add("todo", 1, qld)
	exec(`INSERT INTO todos(id,assignee_id,created_by,data,created_at,updated_at) VALUES('t1',?,?,'{"id":"t1","propId":"todo"}',?,?)`, mary, mary, now(), now())
	add("photo", 1, qld)
	exec(`INSERT INTO photos(id,property_id,created_by,mime,size,created_at) VALUES('ph1','photo',?,'image/jpeg',1,?)`, mary, now())
	add("app", 1, ``)                                    // created on the phone: no source, no lotplan
	add("parcel-9", 1, `,"lot":"1RP1","lotplan":"1RP1"`) // phone parcel record, even if enriched with a lotplan
	add("manual", 1, `,"source":"manual","lotplan":"1RP1"`)
	// Not sent.
	add("plain", 1, qld)
	add("seed", 1, `,"lotplan":"13RP135380"`) // legacy demo.json parcel
	add("emptynotes", 1, qld+`,"notes":[]`)
	add("nullpf", 1, qld+`,"pricefinderAttributes":null`)
	add("deltodo", 1, qld)
	exec(`INSERT INTO todos(id,assignee_id,created_by,data,created_at,updated_at,deleted_at) VALUES('t2',?,?,'{"id":"t2","propId":"deltodo"}',?,?,?)`, mary, mary, now(), now(), now())
	add("unregistered", 0, qld)
	exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('properties','unregistered','r2','Properties',?,'1')`, now())
	want = []string{"app", "manual", "notes", "owned", "ownership", "parcel-9", "pf", "photo", "rex", "rp", "todo"}
	return mary, want
}

func pulledIDs(t *testing.T, m map[string]any) []string {
	t.Helper()
	ids := []string{}
	for _, raw := range m["properties"].([]any) {
		ids = append(ids, raw.(map[string]any)["id"].(string))
	}
	sort.Strings(ids)
	return ids
}

func TestSyncPropertyScope(t *testing.T) {
	a, tokens := setup(t)
	_, want := scopeFixture(t, a)
	full := obj(t, request(a, "GET", "/api/v1/sync", tokens["mary"], nil))
	if got := pulledIDs(t, full); strings.Join(got, ",") != strings.Join(want, ",") || full["full"] != true {
		t.Fatalf("full pull %v, want %v", got, want)
	}
	// Admins use the same property rule.
	if got := pulledIDs(t, obj(t, request(a, "GET", "/api/v1/sync", tokens["admi"], nil))); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("admin pull %v", got)
	}
	cursor := full["cursor"].(string)
	if raw, _ := base64.RawURLEncoding.DecodeString(cursor); !strings.HasPrefix(string(raw), syncScopeVersion+":") {
		t.Fatalf("cursor %q lacks the scope version", raw)
	}
	inc := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, tokens["mary"], nil))
	if inc["full"] != false || len(inc["properties"].([]any)) != 0 {
		t.Fatal("current cursor should be incremental", inc["full"], inc["properties"])
	}
	// Cursors issued before the scope existed (bare sequence) or under another
	// version get a full snapshot, which the phone uses to replace its tables.
	seq := strings.TrimPrefix(string(mustDecode(t, cursor)), syncScopeVersion+":")
	for _, old := range []string{seq, "1:" + seq} {
		m := obj(t, request(a, "GET", "/api/v1/sync?since="+base64.RawURLEncoding.EncodeToString([]byte(old)), tokens["mary"], nil))
		if m["full"] != true || strings.Join(pulledIDs(t, m), ",") != strings.Join(want, ",") {
			t.Fatalf("legacy cursor %q: full=%v %v", old, m["full"], pulledIDs(t, m))
		}
	}
	for _, bad := range []string{"!!!", base64.RawURLEncoding.EncodeToString([]byte("2:x")), base64.RawURLEncoding.EncodeToString([]byte("-1"))} {
		if w := request(a, "GET", "/api/v1/sync?since="+bad, tokens["mary"], nil); w.Code != 400 {
			t.Fatalf("bad cursor %q: %d", bad, w.Code)
		}
	}
	// Excluded parcels remain readable on the server.
	if w := request(a, "GET", "/api/v1/properties/plain", tokens["mary"], nil); w.Code != 200 {
		t.Fatal("parcel lookup", w.Code)
	}
}

func mustDecode(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A parcel that later gains market data or app content reaches the phone in
// the next incremental pull, even when the properties row itself is untouched.
func TestSyncPropertyScopeIncremental(t *testing.T) {
	a, tokens := setup(t)
	mary, _ := scopeFixture(t, a)
	for _, id := range []string{"plain2", "plain3", "plain4", "plain5", "plain6"} {
		if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,owner_id,registered,created_at,updated_at) VALUES(?,'2','Scope St','Sunnybank',?,NULL,1,?,?)`, id, `{"id":"`+id+`","source":"qld_parcels","lotplan":"2RP2"}`, now(), now()); err != nil {
			t.Fatal(err)
		}
	}
	cursor := obj(t, request(a, "GET", "/api/v1/sync", tokens["mary"], nil))["cursor"].(string)
	// Market import path (Pricefinder / RP Data ownership).
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = applyMarketOwnership(tx, "plain", "New Owner", 500000, "2026-01-01", "rpdata", "imp", false); err != nil {
		t.Fatal(err)
	}
	if err = storeRPAttributes(tx, "seed", []string{"Bedrooms"}, []string{"3"}, "rp-1", "rp.csv", "", ""); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	// Related rows written without touching properties: covered by migration 016 triggers.
	for _, q := range []string{
		`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('properties','plain2','r3','Properties','x','1')`,
		`INSERT INTO property_owners(property_id,contact_id) VALUES('plain3','c1')`,
		`INSERT INTO photos(id,property_id,created_by,mime,size,created_at) VALUES('ph2','plain4','` + mary + `','image/jpeg',1,'x')`,
		`INSERT INTO property_ownership(property_id,owner_names,source,import_id,updated_at) VALUES('plain6','B','pricefinder','imp','x')`,
	} {
		if _, err := a.db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	// App path: a to-do created through the API on an unsynced parcel.
	if w := request(a, "POST", "/api/v1/todos", tokens["mary"], map[string]any{"id": "t9", "t": "Knock", "d": "2026-01-01", "propId": "plain5"}); w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	inc := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, tokens["mary"], nil))
	if inc["full"] != false {
		t.Fatal("expected incremental")
	}
	got := map[string]bool{}
	for _, raw := range inc["properties"].([]any) {
		got[raw.(map[string]any)["id"].(string)] = true
	}
	for _, id := range []string{"plain", "seed", "plain2", "plain3", "plain4", "plain5", "plain6"} {
		if !got[id] {
			t.Fatalf("%s missing from incremental pull: %v", id, got)
		}
	}
	if got["emptynotes"] || got["deltodo"] || got["unregistered"] {
		t.Fatal("ineligible property in incremental pull", got)
	}
}

// /properties/within applies the same rule; total and linked count the same rows.
func TestPropertiesWithinSyncScope(t *testing.T) {
	a, tokens := setup(t)
	_, want := scopeFixture(t, a)
	poly := [][]float64{{-27.575, 153.055}, {-27.575, 153.065}, {-27.565, 153.065}, {-27.565, 153.055}}
	w := request(a, "POST", "/api/v1/properties/within", tokens["mary"], map[string]any{"polygon": poly, "limit": 200})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body)
	}
	m := obj(t, w)
	ids := []string{}
	for _, raw := range m["items"].([]any) {
		ids = append(ids, raw.(map[string]any)["id"].(string))
	}
	sort.Strings(ids)
	if strings.Join(ids, ",") != strings.Join(want, ",") || m["total"] != float64(len(want)) || m["linked"] != float64(1) {
		t.Fatalf("within %v total=%v linked=%v, want %v", ids, m["total"], m["linked"], want)
	}
}
