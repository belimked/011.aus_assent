package main

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func rpFixtureAt(t *testing.T, address, sale, owner string) []byte {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()
	sheet := f.GetSheetName(0)
	head := []string{"Property Photo", "Street Address", "Suburb", "State", "Postcode", "Council Area", "Property Type", "Bed", "Bath", "Car", "Land Size (m²)", "Floor Size (m²)", "Year Built", "Sale Price", "Sale Date", "Settlement Date", "Sale Type", "Agency", "Agent", "Land Use", "Development Zone", "Parcel Details", "Owner 1 Name", "Owner 2 Name", "Owner 3 Name", "Owner Type", "Vendor 1 Name", "Vendor 2 Name", "Vendor 3 Name", "Open in RPData"}
	values := []any{"", address, "Sunnybank", "QLD", 4109, "Brisbane", "House", 4, 2, 2, 607, 180, 1988, "$1,105,000", sale, "-", "Normal Sale", "Northwind Realty (Test)", "Avery Example (Test)", "Single Unit Dwelling", "Low Density Residential", "-", owner, "-", "-", "Owner Occupied", "Quinn Sample (Test)", "-", "-", ""}
	_ = f.SetSheetRow(sheet, "A1", &[]any{"Search String", "Sunnybank QLD 4109"})
	_ = f.SetSheetRow(sheet, "A3", &head)
	_ = f.SetSheetRow(sheet, "A4", &values)
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSyntheticRPDataFlagAndPurge(t *testing.T) {
	a, _ := setup(t)
	if err := seed(a.db, true); err != nil {
		t.Fatal(err)
	}
	var admin string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE role='admin' LIMIT 1`).Scan(&admin)
	p := principal{id: admin, role: "admin"}
	recent := time.Now().AddDate(0, 0, -2).Format("2 Jan 2006")
	fake := rpFixtureAt(t, "2 Gager St", recent, "Wren Placeholder (Test)")
	const fakeName = "SYNTHETIC_Sunnybank_RPData_propertyExport_20261125103259.xlsx"
	report, err := a.importMarket(fake, "/tmp/out/"+fakeName, "rpdata", "auto", false, p)
	if err != nil || !report.Synthetic || report.Success != 1 || report.SalesNotified != 1 {
		t.Fatalf("synthetic import %+v %v", report, err)
	}
	realReport, err := a.importMarket(rpFixtureAt(t, "4 Gager St", "10 Jul 2016", "Real Example"), "propertyExport_20261125103259 Sunnybank.xlsx", "rpdata", "auto", false, p)
	if err != nil || realReport.Synthetic || realReport.Success != 1 {
		t.Fatalf("real import %+v %v", realReport, err)
	}
	attrs := func(id string) map[string]any {
		var raw string
		_ = a.db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw)
		var prop map[string]any
		_ = json.Unmarshal([]byte(raw), &prop)
		m, _ := prop["rpdataAttributes"].(map[string]any)
		return m
	}
	if attrs("p1")["synthetic"] != "true" || attrs("p2") == nil || attrs("p2")["synthetic"] != nil {
		t.Fatal("synthetic flag", attrs("p1"), attrs("p2"))
	}
	var alerts int
	_ = a.db.QueryRow(`SELECT count(*) FROM market_items WHERE event_id IS NOT NULL AND deleted_at IS NULL`).Scan(&alerts)

	dry, err := purgeSyntheticMarket(a.db, true, p)
	if err != nil || dry.Imports != 1 || dry.Events != 1 || dry.Ownership != 1 || dry.Properties != 1 || dry.Alerts != alerts {
		t.Fatalf("dry purge %+v %v (alerts %d)", dry, err, alerts)
	}
	if attrs("p1")["synthetic"] != "true" {
		t.Fatal("dry run changed data")
	}
	var before int64
	_ = a.db.QueryRow(`SELECT COALESCE(max(seq),0) FROM sync_changes`).Scan(&before)
	done, err := purgeSyntheticMarket(a.db, false, p)
	if err != nil || done != dry {
		t.Fatalf("purge %+v %v", done, err)
	}
	if attrs("p1") != nil || attrs("p2") == nil {
		t.Fatal("attributes after purge", attrs("p1"), attrs("p2"))
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM property_ownership WHERE property_id='p1'`).Scan(&n)
	if n != 0 {
		t.Fatal("synthetic ownership kept")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM property_ownership WHERE property_id='p2'`).Scan(&n)
	if n != 1 {
		t.Fatal("real ownership removed")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM market_imports`).Scan(&n)
	if n != 1 {
		t.Fatal("imports left", n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM market_items WHERE event_id IS NOT NULL AND deleted_at IS NULL`).Scan(&n)
	if n != 0 {
		t.Fatal("synthetic alerts still active", n)
	}
	var floor int64
	_ = a.db.QueryRow(`SELECT CAST(value AS INTEGER) FROM settings WHERE key=?`, syncResetKey).Scan(&floor)
	if floor < before {
		t.Fatal("phones not forced to a full snapshot", floor, before)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity='market_imports' AND entity_id='synthetic'`).Scan(&n)
	if n != 1 {
		t.Fatal("purge not audited")
	}
	// The same generated file can be imported again after a purge.
	if _, err = a.importMarket(fake, fakeName, "rpdata", "auto", true, p); err != nil {
		t.Fatal("re-import after purge", err)
	}
	again, err := purgeSyntheticMarket(a.db, false, p)
	if err != nil || again != (syntheticPurge{}) {
		t.Fatalf("second purge %+v %v", again, err)
	}
}

func TestSyntheticMarketFile(t *testing.T) {
	for name, want := range map[string]bool{
		"SYNTHETIC_Sunnybank_RPData_propertyExport_20261125103259.xlsx": true,
		"/data/in/SYNTHETIC_Sunnybank Hills_RPData_x.xlsx":              true,
		"propertyExport_20261125103259 Sunnybank.xlsx":                  false,
		"synthetic_lowercase.xlsx":                                      false,
		"/SYNTHETIC_dir/real.xlsx":                                      false,
	} {
		if syntheticMarketFile(name) != want {
			t.Fatal(name)
		}
	}
}
