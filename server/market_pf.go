package main

import (
	"database/sql"
	"encoding/json"
	"regexp"
	"strings"
)

func hasHeader(head []string, name string) bool {
	for _, h := range head {
		if key(h) == key(name) {
			return true
		}
	}
	return false
}
func pfColumn(head, line []string, name string) string {
	for i, h := range head {
		if key(h) == key(name) && i < len(line) {
			return strings.TrimSpace(line[i])
		}
	}
	return ""
}

var pfLegal = regexp.MustCompile(`(?i)\bL(?:OT)?\s*(\d+)\s+([A-Z]{1,4}\s*\d+)`)

func pfLotDescription(s string) string {
	if m := pfLegal.FindStringSubmatch(s); m != nil {
		return m[1] + strings.ReplaceAll(m[2], " ", "")
	}
	return s
}
func pfAddress(head, line []string) string {
	street := pfColumn(head, line, "Street Name")
	number := pfColumn(head, line, "Number")
	unit := pfColumn(head, line, "Unit")
	if street == "" {
		return pfColumn(head, line, "Street")
	}
	if unit != "" {
		number = unit + "/" + number
	} else if number != "" && len(street) > 0 && street[0] >= '0' && street[0] <= '9' {
		if strings.Fields(street)[0] == number {
			return street
		}
		return number + "/" + street
	}
	return strings.TrimSpace(number + " " + street)
}

type marketEntry struct {
	id, unit, suburb, postcode, addr, source string
	parcelLike                               bool
}
type marketIndex struct {
	lots  map[string][]marketEntry
	addrs map[string][]marketEntry
}

func loadMarketIndex(db *sql.DB) (marketIndex, error) {
	idx := marketIndex{map[string][]marketEntry{}, map[string][]marketEntry{}}
	rows, err := db.Query(`SELECT id,no,street,suburb,data FROM properties WHERE deleted_at IS NULL`)
	if err != nil {
		return idx, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, no, street, sub, raw string
		if rows.Scan(&id, &no, &street, &sub, &raw) != nil {
			continue
		}
		var d map[string]any
		_ = json.Unmarshal([]byte(raw), &d)
		e := marketEntry{id: id, unit: str(d, "unit"), suburb: key(sub), postcode: str(d, "postcode"), addr: addressKey(no + " " + street), source: str(d, "source"), parcelLike: parcelLike(d)}
		lp := lotKey(str(d, "lotplan"))
		if lp == "" {
			lp = lotKey(str(d, "lot"))
		}
		if lp != "" {
			idx.lots[lp] = append(idx.lots[lp], e)
		}
		idx.addrs[e.suburb+"|"+e.addr] = append(idx.addrs[e.suburb+"|"+e.addr], e)
		if e.unit != "" {
			unitAddr := addressKey(e.unit + "/" + no + " " + street)
			idx.addrs[e.suburb+"|"+unitAddr] = append(idx.addrs[e.suburb+"|"+unitAddr], e)
		}
	}
	return idx, rows.Err()
}
func (idx marketIndex) match(e MarketEvent, hasUnit bool) (string, string) {
	candidates := idx.lots[lotKey(e.LotPlan)]
	// A historical parcel may share its lot with a linked Rex property.
	// Prefer exactly one Rex dwelling at the same unit-specific address.
	hasParcel := false
	for _, candidate := range candidates {
		if candidate.parcelLike || candidate.source == "qld_parcels" {
			hasParcel = true
		}
	}
	if hasParcel {
		var rex string
		ambiguous := false
		for _, p := range idx.addrs[key(e.Suburb)+"|"+addressKey(e.Address)] {
			if p.source != "rex_import" || e.Postcode != "" && p.postcode != "" && p.postcode != e.Postcode {
				continue
			}
			if !hasUnit && p.unit != "" || hasUnit && (p.unit == "" || addressKey(strings.SplitN(e.Address, "/", 2)[0]) != addressKey(p.unit)) {
				continue
			}
			if rex != "" && rex != p.id {
				ambiguous = true
				break
			}
			rex = p.id
		}
		if rex != "" && !ambiguous {
			return rex, "address"
		}
	}
	if len(candidates) == 1 && candidates[0].suburb == key(e.Suburb) && (e.Postcode == "" || candidates[0].postcode == "" || candidates[0].postcode == e.Postcode) {
		unit := strings.SplitN(e.Address, "/", 2)[0]
		if !hasUnit && candidates[0].unit == "" || hasUnit && candidates[0].unit != "" && addressKey(unit) == addressKey(candidates[0].unit) {
			return candidates[0].id, "lot"
		}
	}
	// A lot with multiple units is not a unique dwelling. Match unit-specific address only.
	addr := addressKey(e.Address)
	matches := idx.addrs[key(e.Suburb)+"|"+addr]
	var id string
	for _, p := range matches {
		if e.Postcode != "" && p.postcode != "" && p.postcode != e.Postcode {
			continue
		}
		if id != "" {
			return "", ""
		}
		id = p.id
	}
	if id != "" {
		return id, "address"
	}
	return "", ""
}
func storePFAttributes(tx *sql.Tx, id string, head, line []string) error {
	var raw string
	if err := tx.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw); err != nil {
		return err
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return err
	}
	attrs := map[string]string{}
	for _, field := range []string{"Bedrooms", "Bathrooms", "Car Parks", "Valuation Amount", "Valuation Date", "Land Use", "Zoning"} {
		if v := pfColumn(head, line, field); v != "" {
			attrs[field] = v
		}
	}
	if len(attrs) == 0 {
		return nil
	}
	p["pricefinderAttributes"] = attrs
	b, _ := json.Marshal(p)
	_, err := tx.Exec(`UPDATE properties SET data=?,updated_at=?,version=version+1 WHERE id=?`, string(b), now(), id)
	return err
}
