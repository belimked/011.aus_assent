package main

import (
	"testing"
)

func TestCompetitorListingFromMarketEvents(t *testing.T) {
	a, tokens := setup(t)
	var mary string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	const id = "rex-properties-2057-88001"
	if err := importSave(a.db, "properties", "2057", importRow{"id": "88001", "etag": "v1", "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank"}, mary, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := a.db.Exec(q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES('cl-import','file','other','listings.csv','cl-sha',?,'completed')`, now())
	n := 0
	event := func(provider, kind, date, agency string) {
		t.Helper()
		n++
		exec(`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,event_date,agency,import_id,property_id,raw) VALUES(?,?,?,?,'16 Gager St','16gagerst',?,?,'cl-import',?,'{}')`, ulid(), provider, "pid-"+string(rune('a'+n)), kind, date, agency, id)
	}
	competitor := func() map[string]any {
		t.Helper()
		got := obj(t, request(a, "GET", "/api/v1/properties/"+id, tokens["mary"], nil))
		c, _ := got["competitorListing"].(map[string]any)
		return c
	}
	if c := competitor(); c != nil {
		t.Fatalf("no events: %v", c)
	}
	full := obj(t, request(a, "GET", "/api/v1/sync", tokens["mary"], nil))
	cursor, _ := full["cursor"].(string)

	// A sale with the last sale's agency is not a current listing.
	event("other", "sale", "2019-05-01", "Ray White Sunnybank")
	if c := competitor(); c != nil {
		t.Fatalf("sale counted: %v", c)
	}
	// RP Data "listing" rows carry the last sale's agency: ignored.
	event("rpdata", "listing", "2024-01-01", "Harcourts")
	if c := competitor(); c != nil {
		t.Fatalf("rpdata listing counted: %v", c)
	}
	// A dated listing with an agency, newer than the sale: competitor.
	event("other", "listing", "2025-02-03", "  Harcourts Sunnybank ")
	c := competitor()
	if c == nil || c["agency"] != "Harcourts Sunnybank" || c["date"] != "2025-02-03" || c["provider"] != "other" {
		t.Fatalf("listing: %v", c)
	}
	// The new event reaches an incremental pull although the property row did not change.
	inc := obj(t, request(a, "GET", "/api/v1/sync?since="+cursor, tokens["mary"], nil))
	found := false
	for _, v := range inc["properties"].([]any) {
		if p, _ := v.(map[string]any); p["id"] == id {
			found = p["competitorListing"] != nil
		}
	}
	if !found {
		t.Fatal("incremental pull missed the competitor listing")
	}
	// Our own company (normalised) is not a competitor.
	exec(`INSERT INTO settings(key,value) VALUES('company_name','Harcourts - Sunnybank') ON CONFLICT(key) DO UPDATE SET value=excluded.value`)
	if c := competitor(); c != nil {
		t.Fatalf("own company counted: %v", c)
	}
	exec(`UPDATE settings SET value='Place Sunnybank' WHERE key='company_name'`)
	if c := competitor(); c == nil {
		t.Fatal("competitor lost")
	}
	// A recorded sale on or after the listing date ends it.
	exec(`INSERT INTO property_ownership(property_id,owner_names,last_sale_price,last_sale_date,source,import_id,updated_at) VALUES(?,'',1,'2025-02-03','other','cl-import',?)`, id, now())
	if c := competitor(); c != nil {
		t.Fatalf("listing not newer than last sale: %v", c)
	}
	exec(`UPDATE property_ownership SET last_sale_date='2019-05-01' WHERE property_id=?`, id)
	if c := competitor(); c == nil {
		t.Fatal("older ownership sale hid listing")
	}
	// A withdrawal event on the same day wins the tie.
	event("other", "withdrawn", "2025-02-03", "")
	if c := competitor(); c != nil {
		t.Fatalf("withdrawn listing counted: %v", c)
	}
	// A relisting later than the withdrawal counts again; an agency-less listing does not.
	event("other", "listing", "2025-06-01", "")
	if c := competitor(); c != nil {
		t.Fatalf("listing without agency counted: %v", c)
	}
	event("other", "listing", "2025-07-01", "LJ Hooker")
	if c := competitor(); c == nil || c["agency"] != "LJ Hooker" {
		t.Fatalf("relisting: %v", c)
	}
	// A phone echo of the derived field is not trusted.
	exec(`UPDATE market_events SET kind='withdrawn' WHERE agency='LJ Hooker'`)
	exec(`UPDATE properties SET data=json_set(data,'$.competitorListing',json('{"agency":"Stale"}')) WHERE id=?`, id)
	if c := competitor(); c != nil {
		t.Fatalf("stored field leaked: %v", c)
	}
}
