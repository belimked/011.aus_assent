package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParcelAndPricefinderImport(t *testing.T) {
	a, _ := setup(t)
	if err := seed(a.db, true); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	addr := `{"features":[{"properties":{"lotplan":"999RP999999","street_number":"22","street_name":"Acorus","street_type":"Place","locality":"Sunnybank","latitude":-27.57,"longitude":153.06},"geometry":{"type":"Point","coordinates":[153.06,-27.57]}}]}`
	parcel := `{"features":[{"properties":{"lotplan":"999RP999999","lot_area":600},"geometry":{"type":"Polygon","coordinates":[[[153.06,-27.57],[153.061,-27.57],[153.06,-27.571],[153.06,-27.57]]]}}]}`
	for name, content := range map[string]string{"addr_test_0.json": addr, "parcel_test_0.json": parcel} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := importParcels(a.db, dir, true); err != nil || n.Added != 1 {
		t.Fatalf("dry %d %v", n, err)
	}
	if n, err := importParcels(a.db, dir, false); err != nil || n.Added != 1 {
		t.Fatalf("import %d %v", n, err)
	}
	if n, err := importParcels(a.db, dir, false); err != nil || n.Added != 0 {
		t.Fatalf("repeat %d %v", n, err)
	}
	csv := `Street,Unit,Number,Street Name,Locality,Postcode,Legal Description,Current Owners,Last Sale,Last Sale Date,Owner Occupier,Bedrooms
22 Acorus Place,,22,Acorus Place,Sunnybank,4109,L999 RP999999,Example Person,450000,2009-07-06,false,4
22 Acorus Place,,22,Acorus Place,WRONG,4109,L999 RP999999,Wrong Person,450000,2009-07-06,false,4
`
	report, err := a.importMarket([]byte(csv), "synthetic.csv", "pricefinder", "ownership", true, principal{role: "admin"})
	if err != nil || report.MatchedLot != 1 || report.Misaligned != 1 {
		t.Fatalf("%+v %v", report, err)
	}
	var id string
	if err := a.db.QueryRow(`SELECT id FROM properties WHERE data LIKE '%999RP999999%'`).Scan(&id); err != nil {
		t.Fatal(err)
	}
	report, err = a.importMarket([]byte(csv), "synthetic.csv", "pricefinder", "ownership", false, principal{id: "admin", role: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	m := a.getMarketTest(t, id, principal{role: "admin"})
	if _, seen := m["owners"]; seen || m["hasOwners"] != true || !strings.HasPrefix(m["lastSaleDate"].(string), "2009-") {
		t.Fatal(m)
	}
	var stored string
	if _ = a.db.QueryRow(`SELECT owner_names FROM property_ownership WHERE property_id=?`, id).Scan(&stored); stored != "Example Person" {
		t.Fatal("stored owner", stored)
	}
	var john string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&john); err != nil {
		t.Fatal(err)
	}
	agent := principal{id: john, role: "agent"}
	if _, visible := a.getMarketTest(t, id, agent)["owners"]; visible {
		t.Fatal("owner names on a property read")
	}
	if err := createMarketZone(a.db, "john@axleo.local"); err != nil {
		t.Fatal(err)
	}
	if _, visible := a.getMarketTest(t, id, agent)["owners"]; visible {
		t.Fatal("zone put owner names back on a property read")
	}
	if err := createMarketZone(a.db, "john@axleo.local"); err != nil {
		t.Fatal(err)
	}
}
