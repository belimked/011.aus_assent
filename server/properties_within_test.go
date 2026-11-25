package main

import "testing"

func TestWithinPolygon(t *testing.T) {
	concave := []withinPoint{{0, 0}, {0, 4}, {4, 4}, {4, 3}, {1, 3}, {1, 1}, {4, 1}, {4, 0}}
	for _, tc := range []struct {
		pt     withinPoint
		inside bool
	}{{withinPoint{.5, .5}, true}, {withinPoint{3, 2}, false}, {withinPoint{3, 3.5}, true}, {withinPoint{5, 2}, false}} {
		if withinPolygon(tc.pt, concave) != tc.inside {
			t.Fatalf("%v: expected %v", tc.pt, tc.inside)
		}
	}
}
func TestPropertiesWithin(t *testing.T) {
	a, tokens := setup(t)
	var mary, john string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary)
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&john)
	for _, row := range []struct{ id, owner, ll, source string }{{"parcel", mary, `[-27.57,153.06]`, "qld"}, {"rex", mary, `[-27.571,153.061]`, "rex_import"}, {"private", john, `[-27.57,153.06]`, "manual"}, {"outside", mary, `[-27.59,153.06]`, "qld"}} {
		if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,owner_id,registered,created_at,updated_at) VALUES(?,'1','Test St','Sunnybank',?,?,0,?,?)`, row.id, `{"id":"`+row.id+`","ll":`+row.ll+`,"source":"`+row.source+`"}`, row.owner, now(), now()); err != nil {
			t.Fatal(err)
		}
	}
	// SQL NULL represents the common parcel; an empty owner is a private owner, not public.
	if _, err := a.db.Exec(`UPDATE properties SET owner_id=NULL WHERE id IN ('parcel','rex','outside')`); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('c',?,'{"id":"c","first":"Secret"}',?,?)`, mary, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO property_owners(property_id,contact_id) VALUES('parcel','c')`); err != nil {
		t.Fatal(err)
	}
	poly := [][]float64{{-27.575, 153.055}, {-27.575, 153.065}, {-27.565, 153.065}, {-27.565, 153.055}}
	call := func(token string, limit, offset int) map[string]any {
		t.Helper()
		w := request(a, "POST", "/api/v1/properties/within", token, map[string]any{"polygon": poly, "limit": limit, "offset": offset})
		if w.Code != 200 {
			t.Fatalf("%d %s", w.Code, w.Body)
		}
		return obj(t, w)
	}
	m := call(tokens["mary"], 1, 0)
	if m["total"] != float64(2) || m["linked"] != float64(1) || len(m["items"].([]any)) != 1 {
		t.Fatal(m)
	}
	next := call(tokens["mary"], 1, 1)
	if next["items"].([]any)[0].(map[string]any)["id"] != "rex" {
		t.Fatal(next)
	}
	if call(tokens["admi"], 10, 0)["total"] != float64(3) {
		t.Fatal("admin visibility")
	}
	for _, bad := range []any{[][]float64{{0, 0}, {0, 1}}, [][]float64{{91, 0}, {91, 1}, {90, 2}}, [][]float64{{-27, 153}, {-27, 154}, {-26, 154}}} {
		w := request(a, "POST", "/api/v1/properties/within", tokens["mary"], map[string]any{"polygon": bad})
		if w.Code != 400 {
			t.Fatalf("invalid: %d", w.Code)
		}
	}
	if w := request(a, "POST", "/api/v1/properties/within", "", map[string]any{"polygon": poly}); w.Code != 401 {
		t.Fatalf("unauthorized: %d", w.Code)
	}
}
