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

type parcelCounts struct{ Added, LinkedToExisting, Ambiguous, Skipped int }
type parcelPrior struct {
	id, geometry string
	data         map[string]any
	registered   bool
}

// propertyAddressKey keeps units separate from whole-building addresses.
func propertyAddressKey(suburb, no, street, unit string) string {
	return key(suburb) + "|" + addressKey(no+" "+street) + "|" + addressKey(unit)
}

func enrichParcel(tx *sql.Tx, p parcelPrior, lp string, lat, lng, area float64, geometry json.RawMessage, dry bool) error {
	d := p.data
	changed := false
	for _, field := range []string{"lotplan", "lot"} {
		if str(d, field) == "" {
			d[field] = strings.ToUpper(lp)
			changed = true
		}
	}
	if d["ll"] == nil && lat != 0 && lng != 0 {
		d["ll"] = []float64{lat, lng}
		changed = true
	}
	if d["land"] == nil && area > 0 {
		d["land"] = area
		changed = true
	}
	geo := p.geometry
	if (geo == "" || geo == "null") && len(geometry) > 0 && string(geometry) != "null" {
		geo = string(geometry)
		changed = true
	}
	if changed && !dry {
		b, _ := json.Marshal(d)
		_, err := tx.Exec(`UPDATE properties SET data=?,geometry=?,updated_at=?,version=version+1 WHERE id=?`, string(b), geo, now(), p.id)
		return err
	}
	return nil
}

// GeoJSON field names use snake_case; keep explicit tags for the QLD API.
func importParcels(db *sql.DB, dir string, dry bool) (parcelCounts, error) {
	var counts parcelCounts
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return counts, err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return counts, fmt.Errorf("no GeoJSON pages in %s", dir)
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
			return counts, e
		}
		if e = json.Unmarshal(b, &page); e != nil {
			return counts, fmt.Errorf("%s: %w", file, e)
		}
		for _, f := range page.Features {
			lp := lotKey(f.Properties.Lotplan)
			if lp == "" {
				continue
			}
			if strings.HasPrefix(filepath.Base(file), "parcel_") {
				lots[lp] = parcel{f.Geometry, f.Properties.LotArea}
			} else if strings.HasPrefix(filepath.Base(file), "addr_") {
				addresses = append(addresses, f)
			}
		}
	}
	byLot := map[string][]parcelPrior{}
	byAddress := map[string][]parcelPrior{}
	rows, err := db.Query(`SELECT id,no,street,suburb,data,registered,COALESCE(geometry,'') FROM properties WHERE deleted_at IS NULL`)
	if err != nil {
		return counts, err
	}
	for rows.Next() {
		var p parcelPrior
		var no, street, sub, raw string
		if err = rows.Scan(&p.id, &no, &street, &sub, &raw, &p.registered, &p.geometry); err != nil {
			break
		}
		_ = json.Unmarshal([]byte(raw), &p.data)
		if p.data == nil {
			p.data = map[string]any{}
		}
		lp := lotKey(str(p.data, "lotplan"))
		if lp == "" {
			lp = lotKey(str(p.data, "lot"))
		}
		if lp != "" {
			byLot[lp] = append(byLot[lp], p)
		}
		byAddress[propertyAddressKey(sub, no, street, str(p.data, "unit"))] = append(byAddress[propertyAddressKey(sub, no, street, str(p.data, "unit"))], p)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		return counts, err
	}
	tx, err := db.Begin()
	if err != nil {
		return counts, err
	}
	defer tx.Rollback()
	for _, f := range addresses {
		p := f.Properties
		lp := lotKey(p.Lotplan)
		if lp == "" || p.StreetNumber == "" || p.StreetName == "" || p.Latitude == 0 || p.Longitude == 0 {
			counts.Skipped++
			continue
		}
		matches := byLot[lp]
		if len(matches) > 1 {
			counts.Ambiguous++
			continue
		}
		if len(matches) == 1 && (matches[0].registered || str(matches[0].data, "source") == "rex_import") {
			counts.Skipped++
			continue
		}
		street := strings.TrimSpace(p.StreetName + " " + p.StreetType)
		addr := propertyAddressKey(p.Locality, p.StreetNumber, street, p.UnitNumber)
		candidates := byAddress[addr]
		if len(candidates) > 1 {
			counts.Ambiguous++
			continue
		}
		if len(candidates) == 1 && (str(candidates[0].data, "source") == "rex_import" || candidates[0].registered && str(candidates[0].data, "source") != "qld_parcels" || candidates[0].data["source"] == "agent") {
			prior := candidates[0]
			if err = enrichParcel(tx, prior, lp, p.Latitude, p.Longitude, lots[lp].area, lots[lp].geometry, dry); err != nil {
				return counts, err
			}
			byLot[lp] = append(byLot[lp], prior)
			counts.LinkedToExisting++
			continue
		}
		if len(matches) == 1 { // Legacy unregistered parcel: retain its identity.
			prior := matches[0]
			if err = enrichParcel(tx, prior, lp, p.Latitude, p.Longitude, lots[lp].area, lots[lp].geometry, dry); err != nil {
				return counts, err
			}
			if !dry {
				if _, err = tx.Exec(`UPDATE properties SET registered=1,updated_at=?,version=version+1 WHERE id=?`, now(), prior.id); err != nil {
					return counts, err
				}
			}
			counts.Added++
			byLot[lp] = []parcelPrior{{id: prior.id, registered: true, data: prior.data}}
			continue
		}
		if len(candidates) == 1 {
			counts.Skipped++
			continue
		}
		geometry := lots[lp].geometry
		data := map[string]any{"id": ulid(), "no": p.StreetNumber, "street": street, "suburb": p.Locality, "lotplan": strings.ToUpper(lp), "lot": strings.ToUpper(lp), "unit": p.UnitNumber, "ll": []float64{p.Latitude, p.Longitude}, "source": "qld_parcels", "ownerIds": []string{}, "pfOwner": "", "lastSale": "", "flags": map[string]any{}}
		if lots[lp].area > 0 {
			data["land"] = lots[lp].area
		}
		b, _ := json.Marshal(data)
		stamp := now()
		if !dry {
			_, err = tx.Exec(`INSERT INTO properties(id,no,street,suburb,geometry,pf_owner,last_sale,data,created_at,updated_at,registered) VALUES(?,?,?,?,?,?,?,?,?,?,1)`, data["id"], p.StreetNumber, street, p.Locality, string(geometry), "", "", string(b), stamp, stamp)
			if err != nil {
				return counts, err
			}
		}
		byLot[lp] = append(byLot[lp], parcelPrior{registered: true})
		byAddress[addr] = append(byAddress[addr], parcelPrior{registered: true})
		counts.Added++
	}
	if !dry {
		err = tx.Commit()
	}
	return counts, err
}
