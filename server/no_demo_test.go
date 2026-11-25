package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestSeedDefaultsToParcelsOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seed.db")
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(db, path); err != nil {
		t.Fatal(err)
	}
	if err = seed(db, false); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"contacts", "todos", "listings", "referrals", "ofi_attendees", "zones", "tags"} {
		var n int
		if err = db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: %d %v", table, n, err)
		}
	}
	var count int
	if err = db.QueryRow("SELECT count(*) FROM properties").Scan(&count); err != nil || count != 4449 {
		t.Fatalf("parcels: %d %v", count, err)
	}
	var raw string
	if err = db.QueryRow("SELECT data FROM properties LIMIT 1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var parcel map[string]any
	if err = json.Unmarshal([]byte(raw), &parcel); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ownerIds", "flags", "beds", "baths", "land", "pfOwner"} {
		if _, ok := parcel[key]; ok {
			t.Fatalf("parcel has demo field %s", key)
		}
	}
	for _, key := range []string{"lotplan", "geometry", "area", "lat", "lng"} {
		if _, ok := parcel[key]; !ok {
			t.Fatalf("parcel missing %s", key)
		}
	}
	if err = seed(db, true); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT count(*) FROM contacts").Scan(&count); err != nil || count == 0 {
		t.Fatalf("explicit demo: %d %v", count, err)
	}
}

func TestResetDefaultsToNoDemo(t *testing.T) {
	a, path, _ := resetFixture(t)
	if err := resetCommand(a.db, path, []string{"--yes"}); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"contacts", "todos", "listings", "referrals", "ofi_attendees", "tags"} {
		var n int
		if err := a.db.QueryRow("SELECT count(*) FROM " + table).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s: %d %v", table, n, err)
		}
	}
	var n int
	if err := a.db.QueryRow("SELECT count(*) FROM zones WHERE id='z1'").Scan(&n); err != nil || n != 0 {
		t.Fatalf("demo zones: %d %v", n, err)
	}
	if err := a.db.QueryRow("SELECT count(*) FROM properties WHERE json_extract(data,'$.lotplan') IS NOT NULL").Scan(&n); err != nil || n != 4449 {
		t.Fatalf("parcels: %d %v", n, err)
	}
	var raw string
	if err := a.db.QueryRow("SELECT data FROM properties WHERE id='p1' AND deleted_at IS NOT NULL").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pfOwner", "beds", "baths", "land", "ownerIds"} {
		if _, ok := p[key]; ok {
			t.Fatalf("retained Pricefinder-linked home has demo field %s", key)
		}
	}
}
