package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportParcelsAmbiguousAddressAndUnit(t *testing.T) {
	a, _ := setup(t)
	for _, id := range []string{"rex-a", "rex-b"} {
		if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at,registered) VALUES(?,'12','Lister St','Sunnybank',?,'now','now',1)`, id, `{"source":"rex_import","unit":"3"}`); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	addr := `{"features":[{"properties":{"lotplan":"2RP123","street_number":"12","street_name":"Lister","street_type":"Street","locality":"Sunnybank","unit_number":"3","latitude":-27.57,"longitude":153.06}}]}`
	if err := os.WriteFile(filepath.Join(dir, "addr_0.json"), []byte(addr), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dry := range []bool{true, false} {
		counts, err := importParcels(a.db, dir, dry)
		if err != nil || counts.Ambiguous != 1 || counts.Added != 0 || counts.LinkedToExisting != 0 {
			t.Fatal(counts, err)
		}
	}
	var count int
	_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE deleted_at IS NULL`).Scan(&count)
	if count != 2 {
		t.Fatal(count)
	}
}
