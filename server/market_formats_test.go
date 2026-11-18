package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"
)

func TestMarketFormats(t *testing.T) {
	csvData := []byte("Street,Locality,Last Sale Date\n8 Fiction St,Sunnybank,1/2/2025\n")
	rows, err := marketRows(csvData)
	if err != nil || len(rows) != 2 || rows[1][2] != "1/2/2025" {
		t.Fatal(rows, err)
	}
	f := excelize.NewFile()
	defer f.Close()
	_ = f.SetCellValue("Sheet1", "A1", "Street")
	_ = f.SetCellValue("Sheet1", "A2", "8 Fiction St")
	var b bytes.Buffer
	if err = f.Write(&b); err != nil {
		t.Fatal(err)
	}
	rows, err = marketRows(b.Bytes())
	if err != nil || rows[1][0] != "8 Fiction St" {
		t.Fatal(rows, err)
	}
	if date, err := marketDate("1/2/2025"); err != nil || date != "2025-02-01" {
		t.Fatal(date, err)
	}
	for _, s := range []string{"L7 RP111842", "Lot 7 RP 111842", "7RP111842", "7/RP111842"} {
		if lotKey(s) != "7rp111842" {
			t.Fatal(s, lotKey(s))
		}
	}
	if !ignoredMarketRow([]string{"Copyright: test provider"}) || !ignoredMarketRow(nil) {
		t.Fatal("footer")
	}
	for _, h := range []string{"Current Owners Address", "Vendor Names", "Purchaser Names", "Other Vendor"} {
		if safeMarketHeader(h) {
			t.Fatal(h)
		}
	}
	if !absenteeMarket("99 Imaginary Ave, Sunnybank QLD 4109", "8 Fiction St") || absenteeMarket("8 FICTION ST, SUNNYBANK QLD 4109", "8 Fiction St") {
		t.Fatal("absentee")
	}
}
func TestPricefinderSnapshot(t *testing.T) {
	a, tokens := setup(t)
	if err := seed(a.db); err != nil {
		t.Fatal(err)
	}
	var admin string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE name='Mary'`).Scan(&admin)
	p := principal{id: admin, role: "admin"}
	today := time.Now().In(func() *time.Location { l, _ := time.LoadLocation("Australia/Brisbane"); return l }()).Format("02/01/2006")
	input := func(date, ownerAddress string) []byte {
		return []byte(fmt.Sprintf("Street,Locality,Legal Description,Current Owners,Current Owners Address,Vendor Names,Purchaser Names,Last Sale,Last Sale Date,PDS ID\n2 Gager St,Sunnybank,L12 RP123456,Fictional Owner,%s,Private Vendor,Private Purchaser,$510000,%s,1234\nCopyright disclaimer,,,,,,,,,\n", ownerAddress, date))
	}
	b := input(today, "99 Imaginary Ave")
	report, err := a.importMarket(b, "fiction.csv", "pricefinder", "auto", false, p)
	if err != nil || report.Errors != 0 || report.Ignored != 1 || report.OwnershipUpdated != 1 || report.SalesRecorded != 1 || report.SalesNotified != 1 {
		t.Fatalf("report %+v %v", report, err)
	}
	var raw string
	var storedPrice int64
	var storedDate string
	_ = a.db.QueryRow(`SELECT last_sale_price,last_sale_date FROM property_ownership WHERE property_id='p1'`).Scan(&storedPrice, &storedDate)
	if storedPrice != 510000 || storedDate != time.Now().In(func() *time.Location { l, _ := time.LoadLocation("Australia/Brisbane"); return l }()).Format("2006-01-02") {
		t.Fatal("sale price/date mapping")
	}
	_ = a.db.QueryRow(`SELECT raw FROM market_events WHERE provider_id LIKE '1234:%'`).Scan(&raw)
	var fields map[string]string
	if err = json.Unmarshal([]byte(raw), &fields); err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"Current Owners", "Current Owners Address", "Vendor Names", "Purchaser Names"} {
		if _, ok := fields[h]; ok {
			t.Fatal("private raw header", h)
		}
	}
	for _, tc := range []struct {
		token   string
		visible bool
	}{{tokens["mary"], true}, {tokens["john"], false}} {
		v := obj(t, request(a, "GET", "/api/v1/properties/p1", tc.token, nil))
		_, ok := v["market"].(map[string]any)["absenteeOwner"]
		if ok != tc.visible {
			t.Fatal("absentee visibility", ok)
		}
	}
	older := time.Now().AddDate(-1, 0, 0).Format("02/01/2006")
	report, err = a.importMarket(input(older, "N/A"), "older.csv", "pricefinder", "auto", false, p)
	if err != nil || report.SalesNotified != 0 || report.SalesRecorded != 1 {
		t.Fatalf("older %+v %v", report, err)
	}
	var last string
	_ = a.db.QueryRow(`SELECT last_sale_date FROM property_ownership WHERE property_id='p1'`).Scan(&last)
	if last != time.Now().In(func() *time.Location { l, _ := time.LoadLocation("Australia/Brisbane"); return l }()).Format("2006-01-02") {
		t.Fatal("older sale overwrote newer")
	}
	changed := bytes.Replace(b, []byte("$510000"), []byte("$520000"), 1)
	report, err = a.importMarket(changed, "retry.csv", "pricefinder", "auto", false, p)
	if err != nil || report.SalesNotified != 0 {
		t.Fatal(report, err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM market_events WHERE provider_id LIKE '1234:%'`).Scan(&n)
	if n != 2 {
		t.Fatal("event identity", n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM market_items WHERE event_id IS NOT NULL AND owner_id=?`, admin).Scan(&n)
	if n != 1 {
		t.Fatal("duplicate notification", n)
	}
	if strings.Contains(raw, "Imaginary") {
		t.Fatal("address in raw")
	}
}
func TestMarketMatchesUnregisteredParcelByLotplan(t *testing.T) {
	a, _ := setup(t)
	if err := seed(a.db); err != nil {
		t.Fatal(err)
	}
	var admin string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE role='admin' LIMIT 1`).Scan(&admin)
	// Parcel reference rows (DCDB) are unregistered and keep the lot in data.lotplan.
	_, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at,registered) VALUES('parcel1','77','Imaginary St','Sunnybank','{"lotplan":"9SP999999"}','now','now',0)`)
	if err != nil {
		t.Fatal(err)
	}
	csv := []byte("Street,Locality,Legal Description,Current Owners,Current Owners Address,Last Sale,Last Sale Date,PDS ID\n77 IMAGINARY ST,SUNNYBANK,L9 SP999999,Fictional Owner,N/A,$700000,01/02/2003,999\n")
	report, err := a.importMarket(csv, "parcel.csv", "pricefinder", "auto", false, principal{id: admin, role: "admin"})
	if err != nil || report.Unmatched != 0 || report.OwnershipUpdated != 1 {
		t.Fatalf("report %+v %v", report, err)
	}
	var registered int
	_ = a.db.QueryRow(`SELECT registered FROM properties WHERE id='parcel1'`).Scan(&registered)
	if registered != 1 {
		t.Fatal("matched parcel must become registered so it syncs")
	}
}
