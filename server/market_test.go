package main

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAddressKeys(t *testing.T) {
	cases := [][2]string{{"2 Gager St", "2 gager street"}, {"Unit 1 12 Gager St", "1/12 Gager Street"}, {"UNIT 20 12 Gager RD", "20/12 gager road"}, {"1/12 TEST AVE", "Unit 1 12 Test Avenue"}, {"1 Main Dr.", "1 MAIN DRIVE"}, {"3 Foo Ct", "3 Foo Court"}, {"4 Bar Cres.", "4 bar crescent"}, {"5 First Pl", "5 first PLACE"}, {"6 Alpha Pde", "6 alpha parade"}, {"7 Beta Tce", "7 beta terrace"}, {"  8,  Other St. ", "8 Other Street"}, {"12/4 a Rd", "Unit 12 4 A Road"}, {"1 First Av", "1 first Avenue"}, {"10  High Street", "10 High St"}, {"42 Foo ROAD", "42 foo rd"}}
	for _, c := range cases {
		if addressKey(c[0]) != addressKey(c[1]) {
			t.Errorf("%q != %q", c[0], c[1])
		}
	}
	if lotKey("Lot 12 RP 123456") != lotKey("12 RP-123456") {
		t.Fatal("lot normalization")
	}
}
func TestMarketImport(t *testing.T) {
	a, tokens := setup(t)
	if err := seed(a.db, true); err != nil {
		t.Fatal(err)
	}
	var mary, john string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE name='Mary'`).Scan(&mary)
	_ = a.db.QueryRow(`SELECT id FROM users WHERE name='John'`).Scan(&john)
	p := principal{id: mary, role: "admin", device: "test"}
	b, err := os.ReadFile("testdata/market/pricefinder-sample.csv")
	if err != nil {
		t.Fatal(err)
	}
	before := obj(t, request(a, "GET", "/api/v1/sync", tokens["mary"], nil))
	report, err := a.importMarket(b, "sample.csv", "pricefinder", "auto", true, p)
	if err != nil || report.Rows != 4 || report.Errors != 1 || report.Unmatched != 1 {
		t.Fatalf("dry %v %+v", err, report)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM market_imports`).Scan(&n)
	if n != 0 {
		t.Fatal("dry-run wrote")
	}
	report, err = a.importMarket(b, "sample.csv", "pricefinder", "auto", false, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.importMarket(b, "sample.csv", "pricefinder", "auto", false, p); err == nil {
		t.Fatal("duplicate accepted")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM market_items WHERE event_id IS NOT NULL AND owner_id=?`, mary).Scan(&n)
	if n != 2 {
		t.Fatalf("mary alerts %d", n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM market_items WHERE event_id IS NOT NULL AND owner_id=?`, john).Scan(&n)
	if n != 0 {
		t.Fatalf("john alerts %d", n)
	}
	sync := obj(t, request(a, "GET", "/api/v1/sync?since="+before["cursor"].(string), tokens["mary"], nil))
	if len(sync["market_items"].([]any)) != 2 {
		t.Fatal("incremental market alerts", sync["market_items"])
	}
	if len(sync["properties"].([]any)) != 2 {
		t.Fatalf("incremental property updates %v", sync["properties"])
	}
	// Owner names never ride on a property read, covered or not; only hasOwners does.
	for _, token := range []string{tokens["mary"], tokens["john"]} {
		v := obj(t, request(a, "GET", "/api/v1/properties/p1", token, nil))
		market := v["market"].(map[string]any)
		if _, seen := market["owners"]; seen || market["hasOwners"] != true {
			t.Fatalf("market %v", market)
		}
	}
	admin := a.getMarketTest(t, "p1", principal{role: "admin"})
	if _, seen := admin["owners"]; seen || admin["hasOwners"] != true {
		t.Fatal("admin market", admin)
	}
	if names := obj(t, request(a, "GET", "/api/v1/properties/p1/owner-names", tokens["john"], nil)); names["owners"] == "" {
		t.Fatal("uncovered agent cannot read owner names", names)
	}
	other := append([]byte{}, b...)
	other = bytes.Replace(other, []byte("1250000"), []byte("1350000"), 1)
	if _, err = a.importMarket(other, "updated.csv", "pricefinder", "auto", false, p); err != nil {
		t.Fatal(err)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM market_events WHERE provider_id='pf-sale-001'`).Scan(&n)
	if n != 1 {
		t.Fatal("duplicate row inserted")
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM market_items WHERE event_id IS NOT NULL AND owner_id=?`, mary).Scan(&n)
	if n != 2 {
		t.Fatal("duplicate alert")
	}
	_ = a.db.QueryRow(`SELECT last_sale_price FROM property_ownership WHERE property_id='p1'`).Scan(&n)
	if n != 1350000 {
		t.Fatal("price not updated", n)
	}
}
func (a *app) getMarketTest(t *testing.T, id string, p principal) map[string]any {
	t.Helper()
	m, err := a.get("properties", id, p, false)
	if err != nil {
		t.Fatal(err)
	}
	return m["market"].(map[string]any)
}
func TestZones(t *testing.T) {
	p := map[string]any{"suburb": "Sunnybank", "ll": []any{-27.5, 153.0}}
	for _, test := range []struct {
		zone map[string]any
		want bool
	}{{map[string]any{"suburb": "SUNNYBANK"}, true}, {map[string]any{"suburbs": []any{"Sunnybank"}}, true}, {map[string]any{"center": []any{-27.5, 153.0}, "radius": float64(300)}, true}, {map[string]any{"center": []any{-28.0, 153.0}, "radius": float64(300)}, false}, {map[string]any{"pts": []any{[]any{-27.6, 152.9}, []any{-27.6, 153.1}, []any{-27.4, 153.1}, []any{-27.4, 152.9}}}, true}, {map[string]any{"pts": []any{[]any{-28.0, 152.9}, []any{-28.0, 153.1}, []any{-27.9, 153.1}}}, false}} {
		if zoneCovers(test.zone, p) != test.want {
			t.Errorf("zone %v", test.zone)
		}
	}
}
func TestEmbeddedMarketMappingsOutsideServerDirectory(t *testing.T) {
	original, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Errorf("restore cwd: %v", err)
		}
	})

	if _, err := builtInMarketMappings.ReadFile("market-mappings/pricefinder.json"); err != nil {
		t.Fatal("Pricefinder mapping not embedded:", err)
	}
	for _, tc := range []struct{ provider, field, column string }{
		{"rpdata", "lot_plan", "Parcel Details"},
		{"rpdata", "address", "Street Address"},
		{"pricefinder", "address", "Street Address"},
	} {
		m, err := mapping(tc.provider)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(strings.Join(m[tc.field], ","), tc.column) {
			t.Errorf("%s %s: expected %q, got %v", tc.provider, tc.field, tc.column, m[tc.field])
		}
	}
}

func TestMappingAndBOM(t *testing.T) {
	m, err := mapping("pricefinder")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(m["address"], ","), "Street Address") {
		t.Fatal(m)
	}
	a, _ := setup(t)
	if err := seed(a.db, true); err != nil {
		t.Fatal(err)
	}
	p := principal{id: "test", role: "admin"}
	report, err := a.importMarket([]byte("\xef\xbb\xbfStreet_Address,Locality,Kind\n2 Gager St,Sunnybank,listing\n"), "bom.csv", "pricefinder", "auto", true, p)
	if err != nil || report.Success != 1 || report.Unmatched != 0 {
		t.Fatalf("bom %+v %v", report, err)
	}
}

func TestMarketAdminUpload(t *testing.T) {
	a, tokens := setup(t)
	if w := request(a, "GET", "/api/v1/admin/market/imports", tokens["mary"], nil); w.Code != 404 {
		t.Fatalf("agent list: %d", w.Code)
	}
	data := []byte("Address,Suburb,Kind,Record ID\n1 Missing St,Sunnybank,listing,pf-api-test\n")
	post := func(dry bool) int {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("provider", "pricefinder")
		f, _ := mw.CreateFormFile("file", "upload.csv")
		_, _ = f.Write(data)
		if dry {
			_ = mw.WriteField("dryRun", "true")
		}
		_ = mw.Close()
		req := httptest.NewRequest("POST", "/api/v1/admin/market/imports", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+tokens["admi"])
		rec := httptest.NewRecorder()
		a.routes().ServeHTTP(rec, req)
		return rec.Code
	}
	if code := post(true); code != 201 {
		t.Fatal("dry upload", code)
	}
	if code := post(false); code != 201 {
		t.Fatal("upload", code)
	}
	if code := post(false); code != 409 {
		t.Fatal("duplicate", code)
	}
	var n int
	_ = a.db.QueryRow("SELECT count(*) FROM market_imports").Scan(&n)
	if n != 1 {
		t.Fatal("imports", n)
	}
}
