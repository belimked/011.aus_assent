package main

import (
	"encoding/json"
	"testing"
)

func TestRexLandAreaUnits(t *testing.T) {
	for _, tc := range []struct {
		row  importRow
		want string
	}{
		{importRow{"attr_landarea": float64(989), "attr_landarea_m2": float64(989), "attr_landarea_unit": map[string]any{"id": "m2", "text": "m2"}}, "989 m²"},
		{importRow{"attr_landarea": "1.25", "attr_landarea_unit": map[string]any{"id": "hectares", "text": "Hectares"}}, "1.25 ha"},
		{importRow{"attr_landarea": "2.5", "attr_landarea_unit_id": "acres"}, "2.5 acres"},
		{importRow{"attr_landarea": "1", "attr_landarea_unit": "acre"}, "1 acre"},
		{importRow{"attr_landarea": "40", "attr_landarea_unit": map[string]any{"id": "squares", "text": "Squares"}}, "40 Squares"},
		// No unit on attr_landarea: do not assume m²; use the explicit m² field.
		{importRow{"attr_landarea": "5", "attr_landarea_m2": "20234.3"}, "20234.3 m²"},
		{importRow{"attr_landarea": "5"}, ""},
		{importRow{"attr_landarea": nil, "attr_landarea_m2": "0"}, ""},
		{importRow{}, ""},
	} {
		if got := rexLandArea(tc.row); got != tc.want {
			t.Errorf("%v: got %q want %q", tc.row, got, tc.want)
		}
	}
}

func TestRexImportLandAreaAndMarketSale(t *testing.T) {
	a, tokens := setup(t)
	var mary string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	const id = "rex-properties-2057-77001"
	row := importRow{"id": "77001", "etag": "v1", "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank", "attr_bedrooms": "4", "title_number": "HL12345", "attr_landarea": float64(607), "attr_landarea_m2": float64(607), "attr_landarea_unit": map[string]any{"id": "m2", "text": "m2"}}
	if err := importSave(a.db, "properties", "2057", row, mary, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	data := func() map[string]any {
		var raw string
		if err := a.db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	m := data()
	if m["land"] != "607 m²" {
		t.Fatalf("land %v", m["land"])
	}
	if _, ok := m["lot"]; ok {
		t.Fatalf("title_number must not be mapped to lot: %v", m["lot"])
	}
	// A later Rex read without a land area keeps the stored value.
	row2 := importRow{"id": "77001", "etag": "v2", "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank", "attr_landarea": nil}
	if err := importSave(a.db, "properties", "2057", row2, mary, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	if m = data(); m["land"] != "607 m²" {
		t.Fatalf("empty Rex land overwrote local: %v", m["land"])
	}
	// A new Rex value replaces it, with Rex's unit.
	row3 := importRow{"id": "77001", "etag": "v3", "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank", "attr_landarea": "0.2", "attr_landarea_unit": map[string]any{"id": "ha", "text": "ha"}}
	if err := importSave(a.db, "properties", "2057", row3, mary, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	if m = data(); m["land"] != "0.2 ha" {
		t.Fatalf("land %v", m["land"])
	}

	// Without market data the phone has no purchase date to show.
	got := obj(t, request(a, "GET", "/api/v1/properties/"+id, tokens["mary"], nil))
	if _, ok := got["market"]; ok {
		t.Fatalf("unexpected market %v", got["market"])
	}
	// Market ownership data on a Rex-imported property reaches the phone as market.lastSaleDate.
	if _, err := a.db.Exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES('land-test','file','other','x.csv','land-sha',?,'completed')`, now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO property_ownership(property_id,owner_names,last_sale_price,last_sale_date,source,import_id,updated_at) VALUES(?,'',600000,'2015-03-04','other','land-test',?)`, id, now()); err != nil {
		t.Fatal(err)
	}
	got = obj(t, request(a, "GET", "/api/v1/properties/"+id, tokens["mary"], nil))
	market, _ := got["market"].(map[string]any)
	if market["lastSaleDate"] != "2015-03-04" || got["land"] != "0.2 ha" {
		t.Fatalf("property read %v", got)
	}

	// A phone PATCH echoing land:null does not erase the Rex land area.
	w := request(a, "PATCH", "/api/v1/properties/"+id, tokens["mary"], map[string]any{"version": got["version"], "land": nil})
	if w.Code != 200 {
		t.Fatalf("patch %d %s", w.Code, w.Body.String())
	}
	if m = data(); m["land"] != "0.2 ha" {
		t.Fatalf("null patch erased land: %v", m["land"])
	}
}
