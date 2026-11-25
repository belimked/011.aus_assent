package main

import (
	"database/sql"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var rpIDPattern = regexp.MustCompile(`(?i)rpp\.corelogic\.com\.au/property/[^/"')\s]+/([0-9]+)\b`)
var rpExportPattern = regexp.MustCompile(`_([0-9]{14})`)

func corelogicID(s string) string {
	if m := rpIDPattern.FindStringSubmatch(s); m != nil {
		return m[1]
	}
	return ""
}
func corelogicURL(s string) string {
	if m := rpIDPattern.FindString(s); m != "" {
		return "https://" + strings.TrimPrefix(strings.TrimPrefix(m, "https://"), "http://")
	}
	return ""
}
func rpColumn(head, row []string, name string) string {
	v := pfColumn(head, row, name)
	if v == "-" {
		return ""
	}
	return v
}
func rpNames(head, row []string, prefix string) string {
	names := []string{}
	for i := 1; i <= 3; i++ {
		if v := rpColumn(head, row, prefix+" "+string(rune('0'+i))+" Name"); v != "" {
			names = append(names, v)
		}
	}
	return strings.Join(names, " & ")
}
func storeRPAttributes(tx *sql.Tx, id string, head, row []string, providerID, filename, saleDate, previousDate string) error {
	var raw string
	if err := tx.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw); err != nil {
		return err
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return err
	}
	fields := map[string]string{"propertyType": "Property Type", "councilArea": "Council Area", "floorSize": "Floor Size (m²)", "yearBuilt": "Year Built", "settlementDate": "Settlement Date", "saleType": "Sale Type", "landUse": "Land Use", "developmentZone": "Development Zone", "ownerType": "Owner Type", "beds": "Bed", "baths": "Bath", "cars": "Car", "land": "Land Size (m²)", "parcel": "Parcel Details", "agency": "Agency", "agent": "Agent", "salePrice": "Sale Price"}
	attrs := map[string]string{}
	for k, col := range fields {
		if v := rpColumn(head, row, col); v != "" {
			attrs[k] = v
		}
	}
	attrs["saleDate"] = saleDate
	attrs["updatedAt"] = now()
	if m := rpExportPattern.FindStringSubmatch(filepath.Base(filename)); m != nil {
		if d, e := time.Parse("20060102150405", m[1]); e == nil {
			attrs["exportedAt"] = d.Format(time.RFC3339)
		}
	}
	if syntheticMarketFile(filename) {
		attrs["synthetic"] = "true"
	}
	if providerID != "" {
		attrs["corelogicId"] = providerID
	}
	if u := corelogicURL(rpColumn(head, row, "Open in RPData")); u != "" {
		attrs["corelogicUrl"] = u
	}
	if v := rpNames(head, row, "Owner"); v != "" {
		attrs["owners"] = v
	}
	if v := rpNames(head, row, "Vendor"); v != "" {
		attrs["vendors"] = v
	}
	if saleDate != "" && previousDate != "" && saleDate < previousDate {
		for _, field := range []string{"saleDate", "salePrice", "saleType", "owners", "vendors"} {
			delete(attrs, field)
			if old, ok := p["rpdataAttributes"].(map[string]any); ok {
				if value, ok := old[field].(string); ok {
					attrs[field] = value
				}
			}
		}
	}
	p["rpdataAttributes"] = attrs
	ensurePropertyLL(p)
	b, _ := json.Marshal(p)
	_, err := tx.Exec(`UPDATE properties SET data=?,updated_at=?,version=version+1 WHERE id=?`, string(b), now(), id)
	return err
}
