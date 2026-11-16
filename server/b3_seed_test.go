package main

import "testing"

func TestB3SeedExistingDatabaseIdempotent(t *testing.T) {
	a, tokens := setup(t)
	mary := tokens["mary"]
	if w := request(a, "POST", "/api/v1/contacts", mary, map[string]any{"id": "c1", "first": "Production"}); w.Code != 201 {
		t.Fatal(w.Body)
	}
	for i := 0; i < 2; i++ {
		if err := seed(a.db); err != nil {
			t.Fatal(err)
		}
	}
	if got := obj(t, request(a, "GET", "/api/v1/contacts/c1", mary, nil))["first"]; got != "Production" {
		t.Fatalf("seed overwrote contact: %v", got)
	}
	for kind, want := range map[string]int{"captures": 3, "enquiries": 5, "listings": 3, "ofi_attendees": 2, "sales": 7, "contracts": 1, "referrals": 1, "market_items": 2, "zones": 1, "tags": 4} {
		var n int
		if err := a.db.QueryRow(`SELECT count(*) FROM ` + kind).Scan(&n); err != nil || n != want {
			t.Fatalf("%s count %d want %d: %v", kind, n, want, err)
		}
	}
}
