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

type parcelCounts struct{ Added, Promoted, LinkedToExisting, Ambiguous, Skipped int }
type parcelPrior struct {
	id, geometry string
	addr         string // propertyAddressKey of the row, used to pick one row of a multi-address lot
	data         map[string]any
	registered   bool
}

// Legacy seed parcels have a lot but no source. They must not masquerade as
// owner records merely because they share an address with a Rex property.
func parcelLike(data map[string]any) bool {
	source := str(data, "source")
	return source == "qld_parcels" || source == "" && (lotKey(str(data, "lotplan")) != "" || lotKey(str(data, "lot")) != "")
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
	} else if ensurePropertyLL(d) {
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
		p.addr = propertyAddressKey(sub, no, street, str(p.data, "unit"))
		lp := lotKey(str(p.data, "lotplan"))
		if lp == "" {
			lp = lotKey(str(p.data, "lot"))
		}
		if lp != "" {
			byLot[lp] = append(byLot[lp], p)
		}
		byAddress[p.addr] = append(byAddress[p.addr], p)
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
		street := strings.TrimSpace(p.StreetName + " " + p.StreetType)
		addr := propertyAddressKey(p.Locality, p.StreetNumber, street, p.UnitNumber)
		var owners []parcelPrior
		for _, candidate := range byAddress[addr] {
			if !parcelLike(candidate.data) {
				owners = append(owners, candidate)
			}
		}
		if len(owners) > 1 {
			counts.Ambiguous++
			continue
		}
		if len(owners) == 1 {
			prior := owners[0]
			ownedLot := lotKey(str(prior.data, "lotplan"))
			if ownedLot == "" {
				ownedLot = lotKey(str(prior.data, "lot"))
			}
			if ownedLot != "" && ownedLot != lp {
				counts.Skipped++
				continue
			}
			if err = enrichParcel(tx, prior, lp, p.Latitude, p.Longitude, lots[lp].area, lots[lp].geometry, dry); err != nil {
				return counts, err
			}
			// Keep the in-memory lot index in sync for later address records.
			if ownedLot == "" {
				byLot[lp] = append(byLot[lp], prior)
			}
			counts.LinkedToExisting++
			continue
		}
		var parcelMatches []parcelPrior
		for _, candidate := range byLot[lp] {
			if parcelLike(candidate.data) {
				parcelMatches = append(parcelMatches, candidate)
			}
		}
		// A corner lot carries one row per street address (e.g. 690 Beenleigh Rd and 1 Ardargie St).
		// Pick the row at this record's address; only skip when that is still not unique.
		if len(parcelMatches) > 1 {
			var same []parcelPrior
			for _, candidate := range parcelMatches {
				if candidate.addr == addr {
					same = append(same, candidate)
				}
			}
			parcelMatches = same
			if len(parcelMatches) != 1 {
				counts.Skipped++
				continue
			}
		}
		if len(parcelMatches) == 1 {
			prior := parcelMatches[0]
			if prior.registered {
				counts.Skipped++
				continue
			}
			if err = enrichParcel(tx, prior, lp, p.Latitude, p.Longitude, lots[lp].area, lots[lp].geometry, dry); err != nil {
				return counts, err
			}
			if !dry {
				if _, err = tx.Exec(`UPDATE properties SET registered=1,updated_at=?,version=version+1 WHERE id=?`, now(), prior.id); err != nil {
					return counts, err
				}
			}
			prior.registered = true
			for i := range byLot[lp] { // update in place: other rows on the same lot stay promotable
				if byLot[lp][i].id == prior.id {
					byLot[lp][i].registered = true
				}
			}
			counts.Promoted++
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
		// Index the new parcel with its real data so later records at the same lot/address
		// see a qld_parcels row (skipped), not an empty placeholder that looks like an agent property.
		added := parcelPrior{id: str(data, "id"), addr: addr, geometry: string(geometry), data: data, registered: true}
		byLot[lp] = append(byLot[lp], added)
		byAddress[addr] = append(byAddress[addr], added)
		counts.Added++
	}
	if !dry {
		err = tx.Commit()
	}
	return counts, err
}
