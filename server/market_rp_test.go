package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/xuri/excelize/v2"
)

func rpFixture(t *testing.T, sale, owner string) []byte {
	t.Helper()
	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	head := []string{"Property Photo", "Street Address", "Suburb", "State", "Postcode", "Parcel Details", "Property Type", "Bed", "Bath", "Car", "Land Size (m²)", "Floor Size (m²)", "Year Built", "Sale Price", "Sale Date", "Settlement Date", "Sale Type", "Agency", "Agent", "Land Use", "Development Zone", "Owner 1 Name", "Owner 2 Name", "Owner 3 Name", "Owner Type", "Vendor 1 Name", "Vendor 2 Name", "Vendor 3 Name", "Extra Field", "Open in RPData"}
	_ = f.SetCellValue(sheet, "A1", "Search String")
	_ = f.SetCellValue(sheet, "B1", "Fictional QLD")
	for i, v := range head {
		c, _ := excelize.CoordinatesToCellName(i+1, 4)
		_ = f.SetCellValue(sheet, c, v)
	}
	values := []string{"", "2 Gager St", "Sunnybank", "QLD", "4109", "-", "House", "-", "2", "1", "600", "130", "1999", "$1,250,000", sale, "11 Aug 2020", "Normal", "Fiction Agency", "Example Agent", "Residential", "A", owner, "Second Example", "Third Example", "Person", "Seller Example", "-", "-", "", ""}
	for i, v := range values {
		c, _ := excelize.CoordinatesToCellName(i+1, 5)
		_ = f.SetCellValue(sheet, c, v)
	}
	_ = f.SetCellFormula(sheet, "AD5", `HYPERLINK("https://rpp.corelogic.com.au/property/2-gager-st-sunnybank-qld-4109/1234567")`)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	return buf.Bytes()
}
func TestRPDataLinksAndDates(t *testing.T) {
	link := `https://rpp.corelogic.com.au/property/fictional-road/7654321`
	if corelogicID(link) != "7654321" || corelogicURL(`HYPERLINK("`+link+`")`) != link {
		t.Fatal("CoreLogic link")
	}
	b := rpFixture(t, "10 Jul 2016", "Synthetic Owner")
	if date, err := marketDate(normalMarketExcelDate("42561", b)); err != nil || date == "" {
		t.Fatal("Excel serial", date, err)
	}
}
func TestRPDataSnapshot(t *testing.T) {
	a, tokens := setup(t)
	if err := seed(a.db); err != nil {
		t.Fatal(err)
	}
	var admin string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE role='admin' LIMIT 1`).Scan(&admin)
	p := principal{id: admin, role: "admin"}
	b := rpFixture(t, "10 Jul 2016", "First Example")
	report, err := a.importMarket(b, "propertyExport_20261125103259.xlsx", "rpdata", "auto", true, p)
	if err != nil || report.Ignored != 3 || report.Success != 1 || report.MatchedAddress != 1 {
		t.Fatalf("dry %+v %v", report, err)
	}
	report, err = a.importMarket(b, "propertyExport_20261125103259.xlsx", "rpdata", "auto", false, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.importMarket(b, "again.xlsx", "rpdata", "auto", false, p); err == nil {
		t.Fatal("duplicate accepted")
	}
	var id, owners, raw string
	_ = a.db.QueryRow(`SELECT provider_id FROM market_events WHERE provider='rpdata'`).Scan(&id)
	if id != "1234567:2016-07-10" {
		t.Fatal("id", id)
	}
	_ = a.db.QueryRow(`SELECT owner_names FROM property_ownership WHERE property_id='p1'`).Scan(&owners)
	if owners != "First Example & Second Example & Third Example" {
		t.Fatal("owners")
	}
	_ = a.db.QueryRow(`SELECT data FROM properties WHERE id='p1'`).Scan(&raw)
	var prop map[string]any
	_ = json.Unmarshal([]byte(raw), &prop)
	attrs := prop["rpdataAttributes"].(map[string]any)
	if attrs["beds"] != nil || attrs["exportedAt"] == nil || attrs["corelogicId"] != "1234567" {
		t.Fatal("attrs", attrs)
	}
	john := obj(t, request(a, "GET", "/api/v1/properties/p1", tokens["john"], nil))
	if _, ok := john["rpdataAttributes"].(map[string]any)["vendors"]; ok {
		t.Fatal("vendor leaked")
	}
	if _, ok := john["rpdataAttributes"].(map[string]any)["owners"]; ok {
		t.Fatal("owner leaked")
	}
	pulled := obj(t, request(a, "GET", "/api/v1/sync", tokens["john"], nil))
	for _, item := range pulled["properties"].([]any) {
		property := item.(map[string]any)
		if property["id"] == "p1" {
			private := property["rpdataAttributes"].(map[string]any)
			if private["owners"] != nil || private["vendors"] != nil {
				t.Fatal("sync leaked names")
			}
		}
	}
	newer := rpFixture(t, "11 Jul 2017", "New Example")
	if _, err = a.importMarket(newer, "newer.xlsx", "rpdata", "auto", false, p); err != nil {
		t.Fatal(err)
	}
	older := rpFixture(t, "10 Jul 2015", "Old Example")
	if _, err = a.importMarket(older, "older.xlsx", "rpdata", "auto", false, p); err != nil {
		t.Fatal(err)
	}
	_ = a.db.QueryRow(`SELECT owner_names FROM property_ownership WHERE property_id='p1'`).Scan(&owners)
	if owners != "New Example & Second Example & Third Example" {
		t.Fatal("older overwrote ownership", owners)
	}
}
