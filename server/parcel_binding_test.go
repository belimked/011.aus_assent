package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAddressKeyVariants(t *testing.T) {
	for _, tc := range [][2]string{
		{"12 Lister St", "12 Lister Street"}, {"12 St Albans Rd", "12 St Albans Road"}, {"12 Mt Gravatt Rd", "12 Mount Gravatt Road"},
		{"U 3/12 Lister St", "3/12 Lister Street"}, {"Unit 3, 12 Lister St", "3/12 Lister Street"},
		{"1 Cl", "1 Close"}, {"1 Cct", "1 Circuit"}, {"1 Cir", "1 Circle"}, {"1 Gr", "1 Grove"}, {"1 Gve", "1 Grove"},
		{"1 Hwy", "1 Highway"}, {"1 La", "1 Lane"}, {"1 Ln", "1 Lane"}, {"1 Bvd", "1 Boulevard"}, {"1 Blvd", "1 Boulevard"},
		{"1 Esp", "1 Esplanade"}, {"1 Pkwy", "1 Parkway"}, {"1 Pwy", "1 Parkway"}, {"1 Sq", "1 Square"},
		{"1 Crt", "1 Court"}, {"1 Gdns", "1 Gardens"}, {"1 Cr", "1 Crescent"}, {"1 Crs", "1 Crescent"}, {"1 Wy", "1 Way"}, {"1 Rdg", "1 Ridge"},
	} {
		if addressKey(tc[0]) != addressKey(tc[1]) {
			t.Errorf("%q != %q: %q != %q", tc[0], tc[1], addressKey(tc[0]), addressKey(tc[1]))
		}
	}
	if addressKey("12 St Albans") == addressKey("12 Street Albans") {
		t.Fatal("St prefix became street type")
	}
}

func TestParcelRexBindingAndDedupe(t *testing.T) {
	a, _ := setup(t)
	raw := `{"id":"rex1","source":"rex_import","unit":"","rex":{"id":1}}`
	if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at,registered) VALUES('rex1','12','Lister St','Sunnybank',?,'now','now',1)`, raw); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	addr := `{"features":[{"properties":{"lotplan":"1RP123","street_number":"12","street_name":"Lister","street_type":"Street","locality":"Sunnybank","latitude":-27.57,"longitude":153.06}}]}`
	parcel := `{"features":[{"properties":{"lotplan":"1RP123","lot_area":600},"geometry":{"type":"Polygon","coordinates":[]}}]}`
	for name, body := range map[string]string{"addr_0.json": addr, "parcel_0.json": parcel} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, dry := range []bool{true, false} {
		c, err := importParcels(a.db, dir, dry)
		if err != nil || c.LinkedToExisting != 1 || c.Added != 0 {
			t.Fatalf("dry=%v: %+v %v", dry, c, err)
		}
	}
	var count, version int
	var data, geom string
	_ = a.db.QueryRow(`SELECT count(*) FROM properties`).Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	_ = a.db.QueryRow(`SELECT data,geometry,version FROM properties WHERE id='rex1'`).Scan(&data, &geom, &version)
	var d map[string]any
	_ = json.Unmarshal([]byte(data), &d)
	if d["lotplan"] != "1RP123" || d["land"] != float64(600) || geom == "" || version < 2 || d["source"] != "rex_import" {
		t.Fatal(data, geom, version)
	}
	// Pricefinder supplement must attach to Rex, not to a parcel duplicate.
	idx, err := loadMarketIndex(a.db)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := idx.match(MarketEvent{Address: "12 Lister Street", Suburb: "Sunnybank", LotPlan: "L1 RP123"}, false)
	if id != "rex1" {
		t.Fatal(id)
	}
	// Historical duplicate: keep the Rex id, ownership and PF attributes.
	dup := `{"source":"qld_parcels","lotplan":"1RP123","pricefinderAttributes":{"Bedrooms":"4"}}`
	_, err = a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at) VALUES('parcel','12','Lister Street','Sunnybank',?,'now','now')`, dup)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES('imp','file','pricefinder','x','x','now','complete')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO property_ownership(property_id,owner_names,last_sale_date,source,import_id,updated_at) VALUES('parcel','Owner','2020-01-01','pricefinder','imp','now')`)
	if err != nil {
		t.Fatal(err)
	}
	idx, err = loadMarketIndex(a.db)
	if err != nil {
		t.Fatal(err)
	}
	id, _ = idx.match(MarketEvent{Address: "12 Lister Street", Suburb: "Sunnybank", LotPlan: "L1 RP123"}, false)
	if id != "rex1" {
		t.Fatal(id)
	}
	var admin string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE role='admin' LIMIT 1`).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	csv := `Street,Unit,Number,Street Name,Locality,Postcode,Legal Description,Current Owners,Last Sale,Last Sale Date,Owner Occupier,Bedrooms
12 Lister Street,,12,Lister Street,Sunnybank,4109,L1 RP123,Pricefinder Owner,550000,2021-03-01,false,4
`
	report, err := a.importMarket([]byte(csv), "rex-binding.csv", "pricefinder", "ownership", false, principal{id: admin, role: "admin"})
	if err != nil || report.MatchedAddress+report.MatchedLot != 1 {
		t.Fatal(report, err)
	}
	var ownerName string
	if err := a.db.QueryRow(`SELECT owner_names FROM property_ownership WHERE property_id='rex1'`).Scan(&ownerName); err != nil || ownerName != "Pricefinder Owner" {
		t.Fatal(ownerName, err)
	}
	for _, dry := range []bool{true, false} {
		c, e := dedupeParcels(a.db, dry)
		if e != nil || c.Moved != 1 {
			t.Fatalf("%+v %v", c, e)
		}
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM property_ownership WHERE property_id='rex1'`).Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE id='parcel' AND deleted_at IS NOT NULL`).Scan(&count)
	if count != 1 {
		t.Fatal(count)
	}
	_ = a.db.QueryRow(`SELECT data FROM properties WHERE id='rex1'`).Scan(&data)
	_ = json.Unmarshal([]byte(data), &d)
	if d["pricefinderAttributes"] == nil {
		t.Fatal(data)
	}
	c, e := dedupeParcels(a.db, false)
	if e != nil || c.Moved != 0 {
		t.Fatal(c, e)
	}
}

func TestMarketUnitAndAmbiguity(t *testing.T) {
	idx := marketIndex{lots: map[string][]marketEntry{"1rp123": {{id: "parcel", suburb: "sunnybank", source: "qld_parcels"}}}, addrs: map[string][]marketEntry{"sunnybank|12 lister street": {{id: "parcel", suburb: "sunnybank", source: "qld_parcels"}, {id: "rex", suburb: "sunnybank", source: "rex_import"}}}}
	id, _ := idx.match(MarketEvent{Address: "12 Lister St", Suburb: "Sunnybank", LotPlan: "1RP123"}, false)
	if id != "rex" {
		t.Fatal(id)
	}
	idx.addrs["sunnybank|12 lister street"] = append(idx.addrs["sunnybank|12 lister street"], marketEntry{id: "rex2", suburb: "sunnybank", source: "rex_import"})
	id, _ = idx.match(MarketEvent{Address: "12 Lister St", Suburb: "Sunnybank", LotPlan: "1RP123"}, false)
	if id != "parcel" {
		t.Fatal(id)
	}
	idx.lots["1rp123"][0].unit = "3"
	id, _ = idx.match(MarketEvent{Address: "12 Lister St", Suburb: "Sunnybank", LotPlan: "1RP123"}, false)
	if id != "" {
		t.Fatal(id)
	}
}

// Two QLD address records at the same street address on different lots (e.g. a re-subdivided
// lot) must not treat the freshly inserted parcel as an existing agent property.
func TestParcelImportSameAddressTwoLots(t *testing.T) {
	a, _ := setup(t)
	dir := t.TempDir()
	addr := `{"features":[
{"properties":{"lotplan":"1RP900","street_number":"5","street_name":"Lister","street_type":"Street","locality":"Sunnybank","latitude":-27.57,"longitude":153.06}},
{"properties":{"lotplan":"2RP900","street_number":"5","street_name":"Lister","street_type":"Street","locality":"Sunnybank","latitude":-27.57,"longitude":153.06}}]}`
	if err := os.WriteFile(filepath.Join(dir, "addr_0.json"), []byte(addr), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dry := range []bool{true, false} {
		c, err := importParcels(a.db, dir, dry)
		if err != nil || c.Added != 2 || c.Skipped != 0 || c.LinkedToExisting != 0 {
			t.Fatalf("dry=%v: %+v %v", dry, c, err)
		}
	}
}
