package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

type dedupeCounts struct{ Moved, Ambiguous, Skipped int }

// dedupeParcels repairs historical QLD duplicates without changing Rex-owned fields.
func dedupeParcels(db *sql.DB, dry bool) (dedupeCounts, error) {
	var counts dedupeCounts
	type row struct {
		parcelPrior
		address string
	}
	groups := map[string][]row{}
	rows, err := db.Query(`SELECT id,no,street,suburb,data,COALESCE(geometry,'') FROM properties WHERE deleted_at IS NULL`)
	if err != nil {
		return counts, err
	}
	for rows.Next() {
		var r row
		var no, street, sub, raw string
		if err = rows.Scan(&r.id, &no, &street, &sub, &raw, &r.geometry); err != nil {
			break
		}
		_ = json.Unmarshal([]byte(raw), &r.data)
		if str(r.data, "source") != "rex_import" && !parcelLike(r.data) {
			continue
		}
		r.address = propertyAddressKey(sub, no, street, str(r.data, "unit"))
		groups[r.address] = append(groups[r.address], r)
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
	for _, group := range groups {
		var rex, parcel []row
		for _, r := range group {
			if str(r.data, "source") == "rex_import" {
				rex = append(rex, r)
			} else if parcelLike(r.data) {
				parcel = append(parcel, r)
			}
		}
		if len(parcel) == 0 {
			continue
		}
		if len(rex) == 0 {
			counts.Skipped += len(parcel)
			continue
		}
		if len(rex) != 1 || len(parcel) != 1 {
			counts.Ambiguous += len(parcel)
			continue
		}
		from, to := parcel[0], rex[0]
		if dry {
			counts.Moved++
			continue
		}
		lat, lng, _ := propertyLL(from.data)
		area, _ := from.data["land"].(float64)
		lp := lotKey(str(from.data, "lotplan"))
		if lp == "" {
			lp = lotKey(str(from.data, "lot"))
		}
		if err = enrichParcel(tx, to.parcelPrior, lp, lat, lng, area, json.RawMessage(from.geometry), false); err != nil {
			return counts, err
		}
		var oldDate, newDate sql.NullString
		e := tx.QueryRow(`SELECT last_sale_date FROM property_ownership WHERE property_id=?`, from.id).Scan(&oldDate)
		if e != nil && e != sql.ErrNoRows {
			return counts, e
		}
		e = tx.QueryRow(`SELECT last_sale_date FROM property_ownership WHERE property_id=?`, to.id).Scan(&newDate)
		if e != nil && e != sql.ErrNoRows {
			return counts, e
		}
		if oldDate.Valid && (!newDate.Valid || oldDate.String > newDate.String) {
			if _, err = tx.Exec(`DELETE FROM property_ownership WHERE property_id=?`, to.id); err != nil {
				return counts, err
			}
			if _, err = tx.Exec(`UPDATE property_ownership SET property_id=? WHERE property_id=?`, to.id, from.id); err != nil {
				return counts, err
			}
		} else if _, err = tx.Exec(`DELETE FROM property_ownership WHERE property_id=?`, from.id); err != nil {
			return counts, err
		}
		if to.data["pricefinderAttributes"] == nil && from.data["pricefinderAttributes"] != nil || oldDate.Valid && (!newDate.Valid || oldDate.String > newDate.String) && from.data["pricefinderAttributes"] != nil {
			to.data["pricefinderAttributes"] = from.data["pricefinderAttributes"]
			ensurePropertyLL(to.data)
			b, _ := json.Marshal(to.data)
			if _, err = tx.Exec(`UPDATE properties SET data=?,updated_at=?,version=version+1 WHERE id=?`, string(b), now(), to.id); err != nil {
				return counts, err
			}
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO property_owners(property_id,contact_id,relation,updated_at) SELECT ?,contact_id,relation,updated_at FROM property_owners WHERE property_id=?`, to.id, from.id); err != nil {
			return counts, err
		}
		if _, err = tx.Exec(`DELETE FROM property_owners WHERE property_id=?`, from.id); err != nil {
			return counts, err
		}
		// All property_id columns in the schema (including optional photos and market rows).
		for _, table := range []string{"market_events", "market_items", "todos", "captures", "enquiries", "leads", "listings", "sales", "contracts", "referrals", "photos", "ofi_events"} {
			var n int
			if err = tx.QueryRow(`SELECT count(*) FROM pragma_table_info(?) WHERE name='property_id'`, table).Scan(&n); err != nil {
				return counts, err
			}
			if n == 0 {
				continue
			}
			if _, err = tx.Exec(fmt.Sprintf(`UPDATE %s SET property_id=? WHERE property_id=?`, table), to.id, from.id); err != nil {
				return counts, err
			}
		}
		// Activity has JSON-only property references; migrate them before retiring
		// the legacy parcel. The property_owners/contact links were moved above.
		if _, err = tx.Exec(`UPDATE activity SET data=json_set(data,
			'$.propId',CASE WHEN json_extract(data,'$.propId')=? THEN ? ELSE json_extract(data,'$.propId') END,
			'$.propertyId',CASE WHEN json_extract(data,'$.propertyId')=? THEN ? ELSE json_extract(data,'$.propertyId') END),
			updated_at=?,version=version+1
			WHERE json_valid(data) AND (json_extract(data,'$.propId')=? OR json_extract(data,'$.propertyId')=?)`,
			from.id, to.id, from.id, to.id, now(), from.id, from.id); err != nil {
			return counts, err
		}
		// Market alerts also embed the property id in their JSON payload.
		if _, err = tx.Exec(`UPDATE market_items SET data=json_set(data,'$.propId',?) WHERE json_valid(data) AND json_extract(data,'$.propId')=?`, to.id, from.id); err != nil {
			return counts, err
		}
		stamp := now()
		if _, err = tx.Exec(`UPDATE properties SET deleted_at=?,updated_at=?,version=version+1 WHERE id=?`, stamp, stamp, from.id); err != nil {
			return counts, err
		}
		counts.Moved++
	}
	if !dry {
		err = tx.Commit()
	}
	return counts, err
}
