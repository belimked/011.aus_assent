package main

import (
	"encoding/json"
	"testing"
)

func TestRexRepollKeepsParcelSupplement(t *testing.T) {
	a, _ := setup(t)
	var owner string
	if err := a.db.QueryRow(`SELECT id FROM users LIMIT 1`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	row := importRow{"id": "123", "etag": "first", "adr_street_number": "12", "adr_street_name": "Lister St", "adr_suburb_or_town": "Sunnybank"}
	if err := importSave(a.db, "properties", "2057", row, owner, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	id := "rex-properties-2057-123"
	var raw string
	if err := a.db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var d map[string]any
	_ = json.Unmarshal([]byte(raw), &d)
	d["lotplan"] = "1RP123"
	d["lot"] = "1RP123"
	d["ll"] = []float64{-27.5, 153.1}
	d["land"] = 600.0
	b, _ := json.Marshal(d)
	if _, err := a.db.Exec(`UPDATE properties SET data=? WHERE id=?`, string(b), id); err != nil {
		t.Fatal(err)
	}
	row["etag"] = "second"
	row["attr_bedrooms"] = 4
	if err := importSave(a.db, "properties", "2057", row, owner, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	if err := a.db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(raw), &d)
	if d["lotplan"] != "1RP123" || d["land"] != float64(600) || d["ll"] == nil {
		t.Fatal(raw)
	}
}
