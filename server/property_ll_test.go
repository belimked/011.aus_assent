package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEnsurePropertyLL(t *testing.T) {
	for _, tc := range []struct {
		in      string
		changed bool
		want    []float64
	}{
		{`{"lat":-27.57868,"lng":153.05715}`, true, []float64{-27.57868, 153.05715}},
		{`{"lat":"-27.5","lng":"153.1"}`, true, []float64{-27.5, 153.1}},
		{`{"ll":[-27.1,153.2],"lat":-27.57868,"lng":153.05715}`, false, []float64{-27.1, 153.2}},
		{`{"ll":[],"lat":-27.57868,"lng":153.05715}`, true, []float64{-27.57868, 153.05715}},
		{`{"lat":0,"lng":0}`, false, nil},
		{`{"lat":"","lng":""}`, false, nil},
		{`{"lat":153.05,"lng":-27.5}`, false, nil}, // swapped: not plausible in Australia
		{`{}`, false, nil},
	} {
		var d map[string]any
		if err := json.Unmarshal([]byte(tc.in), &d); err != nil {
			t.Fatal(err)
		}
		if got := ensurePropertyLL(d); got != tc.changed {
			t.Fatalf("%s: changed=%v", tc.in, got)
		}
		lat, lng, ok := propertyLL(d)
		if tc.want == nil {
			if d["ll"] != nil {
				t.Fatalf("%s: unexpected ll %v", tc.in, d["ll"])
			}
			continue
		}
		if !ok || lat != tc.want[0] || lng != tc.want[1] {
			t.Fatalf("%s: got %v,%v,%v", tc.in, lat, lng, ok)
		}
	}
}

func propertyData(t *testing.T, a *app, id string) map[string]any {
	t.Helper()
	var raw string
	if err := a.db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	if err := json.Unmarshal([]byte(raw), &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func llOf(t *testing.T, d map[string]any) []any {
	t.Helper()
	ll, _ := d["ll"].([]any)
	if len(ll) != 2 {
		t.Fatalf("missing ll: %v", d)
	}
	return ll
}

// The bundled DCDB parcels carry lat/lng only; seeding must store ll too.
func TestSeedParcelsHaveLL(t *testing.T) {
	a, _ := setup(t)
	if err := seed(a.db, false); err != nil {
		t.Fatal(err)
	}
	var missing, total int
	if err := a.db.QueryRow(`SELECT count(*), sum(json_type(data,'$.ll') IS NULL) FROM properties WHERE json_type(data,'$.lat') IN ('integer','real')`).Scan(&total, &missing); err != nil {
		t.Fatal(err)
	}
	if total == 0 || missing != 0 {
		t.Fatalf("seed parcels: total=%d without ll=%d", total, missing)
	}
	d := propertyData(t, a, "9-Boorman St-4RP68219")
	ll := llOf(t, d)
	if ll[0] != d["lat"] || ll[1] != d["lng"] {
		t.Fatalf("ll %v vs lat/lng %v/%v", ll, d["lat"], d["lng"])
	}
}

// Pricefinder attribute writes and API writes normalise legacy lat/lng rows.
func TestPropertyWritesAddLL(t *testing.T) {
	a, tok := setup(t)
	stamp := now()
	if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,registered,created_at,updated_at) VALUES('pf','3','Feltwell St','Sunnybank','{"id":"pf","lotplan":"1SP332105","lat":-27.57868,"lng":153.05715}',1,?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err = storePFAttributes(tx, "pf", []string{"Bedrooms"}, []string{"3"}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if ll := llOf(t, propertyData(t, a, "pf")); ll[0] != -27.57868 || ll[1] != 153.05715 {
		t.Fatal(ll)
	}
	w := request(a, "POST", "/api/v1/properties", tok["mary"], map[string]any{"id": "app-prop", "no": "1", "street": "Test St", "suburb": "Sunnybank", "lat": -27.5, "lng": 153.1})
	if w.Code != 200 && w.Code != 201 {
		t.Fatalf("%d %s", w.Code, w.Body)
	}
	if ll := llOf(t, propertyData(t, a, "app-prop")); ll[0] != -27.5 || ll[1] != 153.1 {
		t.Fatal(ll)
	}
}

// Migration 017 backfills ll and logs sync changes so phones pull the fix.
func TestMigrationBackfillsLLWithSyncChange(t *testing.T) {
	a, tok := setup(t)
	var mary string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary)
	stamp := now()
	for _, row := range []struct{ id, data string }{
		{"legacy", `{"id":"legacy","lat":-27.57868,"lng":153.05715,"pricefinderAttributes":{"Bedrooms":"3"}}`},
		{"has-ll", `{"id":"has-ll","ll":[-27.1,153.2],"lat":-27.57868,"lng":153.05715}`},
		{"no-coords", `{"id":"no-coords"}`},
		{"text-coords", `{"id":"text-coords","lat":"-27.5","lng":"153.1"}`},
	} {
		if _, err := a.db.Exec(`INSERT INTO properties(id,owner_id,no,street,suburb,data,registered,created_at,updated_at) VALUES(?,?,'9','Boorman St','Sunnybank',?,1,?,?)`, row.id, mary, row.data, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	cursor := obj(t, request(a, "GET", "/api/v1/sync", tok["mary"], nil))["cursor"].(string)
	var seq, version int
	_ = a.db.QueryRow(`SELECT max(seq) FROM sync_changes`).Scan(&seq)
	_ = a.db.QueryRow(`SELECT version FROM properties WHERE id='legacy'`).Scan(&version)
	if _, err := a.db.Exec(`DELETE FROM schema_migrations WHERE version='017_property_ll_backfill.sql'`); err != nil {
		t.Fatal(err)
	}
	if err := migrate(a.db, t.TempDir()+"/x.db"); err != nil {
		t.Fatal(err)
	}
	d := propertyData(t, a, "legacy")
	if ll := llOf(t, d); ll[0] != -27.57868 || ll[1] != 153.05715 {
		t.Fatal(ll)
	}
	if d["pricefinderAttributes"] == nil {
		t.Fatal("backfill must keep other data")
	}
	if ll := llOf(t, propertyData(t, a, "has-ll")); ll[0] != -27.1 {
		t.Fatal("existing ll must not change", ll)
	}
	for _, id := range []string{"no-coords", "text-coords"} {
		if propertyData(t, a, id)["ll"] != nil {
			t.Fatal(id, "should not get ll")
		}
	}
	var n, v2 int
	_ = a.db.QueryRow(`SELECT count(*) FROM sync_changes WHERE seq>? AND kind='properties' AND id='legacy'`, seq).Scan(&n)
	_ = a.db.QueryRow(`SELECT version FROM properties WHERE id='legacy'`).Scan(&v2)
	if n == 0 || v2 != version+1 {
		t.Fatalf("sync changes %d, version %d->%d", n, version, v2)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM sync_changes WHERE seq>? AND id IN ('has-ll','no-coords')`, seq).Scan(&n)
	if n != 0 {
		t.Fatal("untouched rows must not be re-synced")
	}
	body := request(a, "GET", "/api/v1/sync?since="+cursor, tok["mary"], nil).Body.String()
	if !strings.Contains(body, `"legacy"`) || !strings.Contains(body, `"ll":[-27.57868,153.05715]`) {
		t.Fatalf("incremental pull: %s", body)
	}
}

// Drawn-area lookups must find legacy rows that only carry lat/lng.
func TestPropertiesWithinLatLngFallback(t *testing.T) {
	a, tok := setup(t)
	var mary string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary)
	if _, err := a.db.Exec(`INSERT INTO properties(id,owner_id,no,street,suburb,data,registered,created_at,updated_at) VALUES('legacy',?,'1','Test St','Sunnybank','{"id":"legacy","lat":-27.57,"lng":153.06,"source":"manual"}',1,?,?)`, mary, now(), now()); err != nil {
		t.Fatal(err)
	}
	poly := [][]float64{{-27.575, 153.055}, {-27.575, 153.065}, {-27.565, 153.065}, {-27.565, 153.055}}
	w := request(a, "POST", "/api/v1/properties/within", tok["mary"], map[string]any{"polygon": poly})
	m := obj(t, w)
	if m["total"] != float64(1) {
		t.Fatal(m)
	}
	ll := m["items"].([]any)[0].(map[string]any)["ll"].([]any)
	if ll[0] != -27.57 || ll[1] != 153.06 {
		t.Fatal(ll)
	}
}
