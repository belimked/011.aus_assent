package main

import (
	"os"
	"path/filepath"
	"testing"
)

func legacyAddresses(t *testing.T, records string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "addr_0.json"), []byte(`{"features":[`+records+`]}`), 0600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPromoteMultipleLegacyLotsSameAddress(t *testing.T) {
	a, _ := setup(t)
	lots := []string{"9999BUP106706", "9999SP281546", "400SP281555"}
	for _, lp := range lots {
		if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at) VALUES(?,'245','McCullough St','Sunnybank',?,'now','now')`, lp, `{"lotplan":"`+lp+`"}`); err != nil {
			t.Fatal(err)
		}
	}
	records := ""
	for i, lp := range lots {
		if i > 0 {
			records += ","
		}
		records += `{"properties":{"lotplan":"` + lp + `","street_number":"245","street_name":"McCullough","street_type":"Street","locality":"Sunnybank","latitude":-27.57,"longitude":153.06}}`
	}
	dir := legacyAddresses(t, records)
	for _, dry := range []bool{true, false} {
		c, err := importParcels(a.db, dir, dry)
		if err != nil || c.Promoted != 3 || c.Added != 0 || c.Ambiguous != 0 {
			t.Fatalf("dry=%v %+v %v", dry, c, err)
		}
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE registered=1 AND no='245' AND street='McCullough St'`).Scan(&n)
	if n != 3 {
		t.Fatal(n)
	}
	c, err := importParcels(a.db, dir, false)
	if err != nil || c.Skipped != 3 || c.Promoted != 0 {
		t.Fatal(c, err)
	}
}

func TestLegacyParcelRexBindingAndDedupeReferences(t *testing.T) {
	a, _ := setup(t)
	_, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at,registered) VALUES('legacy','6','Sunnybrae Street','Sunnybank',?,'now','now',0),('rex','6','Sunnybrae St','Sunnybank',?,'now','now',1)`, `{"lotplan":"1RP999","ll":[-27.57,153.06],"land":620,"pricefinderAttributes":{"Bedrooms":"4"}}`, `{"source":"rex_import","rex":{"id":1}}`)
	if err != nil {
		t.Fatal(err)
	}
	dir := legacyAddresses(t, `{"properties":{"lotplan":"1RP999","street_number":"6","street_name":"Sunnybrae","street_type":"Street","locality":"Sunnybank","latitude":-27.57,"longitude":153.06}}`)
	for _, dry := range []bool{true, false} {
		c, e := importParcels(a.db, dir, dry)
		if e != nil || c.LinkedToExisting != 1 || c.Promoted != 0 || c.Ambiguous != 0 {
			t.Fatal(c, e)
		}
	}
	var rexLot string
	_ = a.db.QueryRow(`SELECT json_extract(data,'$.lotplan') FROM properties WHERE id='rex'`).Scan(&rexLot)
	if rexLot != "1RP999" {
		t.Fatal(rexLot)
	}
	idx, e := loadMarketIndex(a.db)
	if e != nil {
		t.Fatal(e)
	}
	id, _ := idx.match(MarketEvent{Address: "6 Sunnybrae Street", Suburb: "Sunnybank", LotPlan: "L1 RP999"}, false)
	if id != "rex" {
		t.Fatalf("market matched %q", id)
	}
	_, err = a.db.Exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES('imp','file','pricefinder','legacy','legacy','now','complete')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO property_ownership(property_id,owner_names,last_sale_date,source,import_id,updated_at) VALUES('legacy','Original','2020-01-02','pricefinder','imp','now')`)
	if err != nil {
		t.Fatal(err)
	}
	var user string
	if err = a.db.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&user); err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('legacy-contact',?,'{}','now','now')`, user)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO property_owners(property_id,contact_id) VALUES('legacy','legacy-contact')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO activity(id,data,created_at,updated_at) VALUES('legacy-activity','{"propId":"legacy","propertyId":"legacy"}','now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = a.db.Exec(`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,import_id,property_id,raw) VALUES('legacy-event','pricefinder','legacy-event','sale','6 Sunnybrae St','6 sunnybrae street','imp','legacy','{}')`)
	if err != nil {
		t.Fatal(err)
	}
	for _, dry := range []bool{true, false} {
		c, e := dedupeParcels(a.db, dry)
		if e != nil || c.Moved != 1 {
			t.Fatal(c, e)
		}
	}
	var n int
	for _, query := range []string{
		`SELECT count(*) FROM property_owners WHERE property_id='rex' AND contact_id='legacy-contact'`,
		`SELECT count(*) FROM property_ownership WHERE property_id='rex' AND owner_names='Original'`,
		`SELECT count(*) FROM market_events WHERE id='legacy-event' AND property_id='rex'`,
		`SELECT count(*) FROM activity WHERE id='legacy-activity' AND json_extract(data,'$.propId')='rex' AND json_extract(data,'$.propertyId')='rex'`,
		`SELECT count(*) FROM properties WHERE id='legacy' AND deleted_at IS NOT NULL`,
	} {
		if err := a.db.QueryRow(query).Scan(&n); err != nil || n != 1 {
			t.Fatalf("%s: %d %v", query, n, err)
		}
	}
	c, e := dedupeParcels(a.db, false)
	if e != nil || c.Moved != 0 {
		t.Fatal(c, e)
	}
}
