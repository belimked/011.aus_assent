package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAdminParcelQueriesAndZones(t *testing.T) {
	a, tok := setup(t)
	admin := tok["admi"]
	var agent string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&agent)
	for _, p := range []struct {
		id, suburb, lot string
		lng             float64
	}{{"parcel-a", "Fictionville", "1RP123", 153.06}, {"parcel-b", "Elsewhere", "2RP123", 153.18}} {
		data := map[string]any{"id": p.id, "no": "1", "street": "Imaginary St", "suburb": p.suburb, "lotplan": p.lot, "lng": p.lng, "lat": -27.57, "ll": []float64{-27.57, p.lng}, "geometry": []any{map[string]any{"lat": -27.57, "lng": p.lng}}}
		raw, _ := json.Marshal(data)
		_, e := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,geometry,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,?,?,?,?)`, p.id, "1", "Imaginary St", p.suburb, "[]", string(raw), now(), now(), 1)
		if e != nil {
			t.Fatal(e)
		}
	}
	list := func(query string) []map[string]any {
		t.Helper()
		w := request(a, "GET", "/api/v1/admin/properties?"+query, admin, nil)
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		var rows []map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &rows)
		return rows
	}
	if rows := list("q=1RP123&suburb=Fictionville&registered=true&hasOwner=false&limit=1"); len(rows) != 1 || rows[0]["id"] != "parcel-a" {
		t.Fatal(rows)
	}
	if rows := list("cursor=parcel-a&limit=1"); len(rows) != 1 || rows[0]["id"] != "parcel-b" {
		t.Fatal(rows)
	}
	if len(list("minYearsHeld=2")) != 0 {
		t.Fatal("years filter")
	}
	_, err := a.db.Exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES('fiction-import','file','other','fiction.csv','fiction-sha',?,'completed')`, now())
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO property_ownership(property_id,owner_names,last_sale_price,last_sale_date,source,import_id,updated_at,absentee_owner) VALUES('parcel-a','Fictional Owner',420000,'2010-01-02','other','fiction-import',?,1)`, now())
	if err != nil {
		t.Fatal(err)
	}
	if rows := list("hasOwner=true&minYearsHeld=2"); len(rows) != 1 || rows[0]["ownerNames"] != "Fictional Owner" || rows[0]["absenteeOwner"] != true {
		t.Fatal(rows)
	}
	before := map[string]any{"id": "parcel-a"}
	a.marketProperty(before, principal{id: agent, role: "agent"})
	if _, ok := before["market"].(map[string]any)["owners"]; ok || before["market"].(map[string]any)["hasOwners"] != true {
		t.Fatal("owner exposed on property read")
	}
	geo := obj(t, request(a, "GET", "/api/v1/admin/properties?bbox=153.05,-27.58,153.07,-27.56", admin, nil))
	if len(geo["features"].([]any)) != 1 {
		t.Fatal(geo)
	}
	feature := geo["features"].([]any)[0].(map[string]any)
	if feature["geometry"] != nil || feature["properties"].(map[string]any)["lotplan"] != "1RP123" {
		t.Fatal("bbox should return status and lot/plan, not fabricated geometry", feature)
	}
	stats := request(a, "GET", "/api/v1/admin/parcel-stats", admin, nil)
	if !strings.Contains(stats.Body.String(), `"total":1`) {
		t.Fatal(stats.Body)
	}
	zone := map[string]any{"id": "zone-fiction", "ownerId": agent, "name": "Fictionville", "suburbs": []string{"Fictionville"}}
	if w := request(a, "POST", "/api/v1/admin/zones", tok["john"], zone); w.Code != 404 {
		t.Fatal(w.Code)
	}
	w := request(a, "POST", "/api/v1/admin/zones", admin, zone)
	if w.Code != 201 {
		t.Fatalf("create %d %s", w.Code, w.Body)
	}
	var changes int
	_ = a.db.QueryRow(`SELECT count(*) FROM sync_changes WHERE kind='zones' AND id='zone-fiction'`).Scan(&changes)
	if changes == 0 {
		t.Fatal("missing sync change")
	}
	if !strings.Contains(request(a, "GET", "/api/v1/sync", tok["mary"], nil).Body.String(), "zone-fiction") {
		t.Fatal("zone absent from sync")
	}
	if !a.covered("parcel-a", agent) || a.covered("parcel-b", agent) {
		t.Fatal("coverage")
	}
	after := map[string]any{"id": "parcel-a"}
	a.marketProperty(after, principal{id: agent, role: "agent"})
	if _, ok := after["market"].(map[string]any)["owners"]; ok || after["market"].(map[string]any)["hasOwners"] != true {
		t.Fatal("coverage put owner names back on a property read")
	}
	if len(list("q=parcel-a&hasOwner=true")) != 1 {
		t.Fatal("ownership filter")
	}
	patch := map[string]any{"ownerId": agent, "name": "New name", "suburbs": []string{"Fictionville"}, "version": 1}
	if w := request(a, "PATCH", "/api/v1/admin/zones/zone-fiction", admin, patch); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if w := request(a, "DELETE", "/api/v1/admin/zones/zone-fiction", admin, nil); w.Code != 200 {
		t.Fatal(w.Body)
	}
	if a.covered("parcel-a", agent) {
		t.Fatal("deleted zone still covers")
	}
}
