package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type qldFeature struct {
	Properties struct {
		Lotplan      string  `json:"lotplan"`
		StreetNumber string  `json:"street_number"`
		StreetName   string  `json:"street_name"`
		StreetType   string  `json:"street_type"`
		Locality     string  `json:"locality"`
		UnitNumber   string  `json:"unit_number"`
		Latitude     float64 `json:"latitude"`
		Longitude    float64 `json:"longitude"`
		LotArea      float64 `json:"lot_area"`
	} `json:"properties"`
	Geometry json.RawMessage `json:"geometry"`
}

// GeoJSON field names use snake_case; keep explicit tags for the QLD API.
func importParcels(db *sql.DB, dir string, dry bool) (int, int, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return 0, 0, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return 0, 0, fmt.Errorf("no GeoJSON pages in %s", dir)
	}
	type parcel struct {
		geometry json.RawMessage
		area     float64
	}
	lots := map[string]parcel{}
	addresses := []qldFeature{}
	for _, file := range files {
		var page struct {
			Features []qldFeature `json:"features"`
		}
		b, e := os.ReadFile(file)
		if e != nil {
			return 0, 0, e
		}
		if e = json.Unmarshal(b, &page); e != nil {
			return 0, 0, fmt.Errorf("%s: %w", file, e)
		}
		for _, f := range page.Features {
			p := f.Properties
			lp := lotKey(p.Lotplan)
			if lp == "" {
				continue
			}
			if strings.HasPrefix(filepath.Base(file), "parcel_") {
				lots[lp] = parcel{f.Geometry, p.LotArea}
			} else if strings.HasPrefix(filepath.Base(file), "addr_") {
				addresses = append(addresses, f)
			}
		}
	}
	// Promote only previously unregistered non-Rex parcels. Never alter Rex or agent records.
	type prior struct {
		id         string
		data       map[string]any
		registered bool
	}
	existing := map[string]prior{}
	rows, err := db.Query(`SELECT id,data,registered FROM properties WHERE deleted_at IS NULL`)
	if err != nil {
		return 0, 0, err
	}
	for rows.Next() {
		var raw, id string
		var registered bool
		if rows.Scan(&id, &raw, &registered) == nil {
			var p map[string]any
			_ = json.Unmarshal([]byte(raw), &p)
			if lp := lotKey(str(p, "lotplan")); lp != "" {
				existing[lp] = prior{id, p, registered}
			} else if lp := lotKey(str(p, "lot")); lp != "" {
				existing[lp] = prior{id, p, registered}
			}
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, 0, err
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()
	added, skipped := 0, 0
	for _, f := range addresses {
		p := f.Properties
		lp := lotKey(p.Lotplan)
		if lp == "" || p.StreetNumber == "" || p.StreetName == "" || p.Latitude == 0 || p.Longitude == 0 {
			skipped++
			continue
		}
		previous, found := existing[lp]
		if found && (previous.registered || str(previous.data, "source") == "rex_import") {
			skipped++
			continue
		}
		existing[lp] = prior{registered: true}
		street := strings.TrimSpace(p.StreetName + " " + p.StreetType)
		geometry := lots[lp].geometry
		data := map[string]any{"id": ulid(), "no": p.StreetNumber, "street": street, "suburb": p.Locality, "lotplan": strings.ToUpper(lp), "lot": strings.ToUpper(lp), "unit": p.UnitNumber, "ll": []float64{p.Latitude, p.Longitude}, "source": "qld_parcels", "ownerIds": []string{}, "pfOwner": "", "lastSale": "", "flags": map[string]any{}}
		if lots[lp].area > 0 {
			data["land"] = lots[lp].area
		}
		b, _ := json.Marshal(data)
		stamp := now()
		if !dry {
			if found {
				previous.data["ll"] = data["ll"]
				previous.data["lotplan"] = data["lotplan"]
				previous.data["source"] = "qld_parcels"
				b, _ = json.Marshal(previous.data)
				_, err = tx.Exec(`UPDATE properties SET data=?,geometry=?,registered=1,updated_at=?,version=version+1 WHERE id=?`, string(b), string(geometry), stamp, previous.id)
			} else {
				_, err = tx.Exec(`INSERT INTO properties(id,no,street,suburb,geometry,pf_owner,last_sale,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,?,?,?,?,?,1)`, data["id"], p.StreetNumber, street, p.Locality, string(geometry), "", "", string(b), stamp, stamp)
			}
			if err != nil {
				return added, skipped, err
			}
		}
		added++
	}
	if !dry {
		err = tx.Commit()
	}
	return added, skipped, err
}
