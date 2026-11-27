package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// resetTables hold test/business data only; they are emptied and optional demo
// rows are then re-inserted by seedData. Order respects foreign keys (children first).
var resetTables = []string{
	"sessions", "contact_notes", "property_owners", "todos", "activity", "captures", "ofi_attendees", "ofi_events",
	"enquiries", "leads", "sales", "contracts", "referrals", "listings", "market_items", "contact_mailing_lists", "contacts", "tags", "photos",
	"approvals", "rex_outbox", "rex_links", "audit_log", "sync_changes", "sync_ops", "owner_name_views",
}

// keptTables are never cleared (properties and zones are partly reset; see below).
var keptTables = []string{"properties", "zones", "users", "teams", "settings", "market_imports", "market_events", "property_ownership", "rex_request_log"}

// syncResetKey stores the last change-log sequence that existed before a reset.
// Pull treats any cursor at or below it as stale and answers with a full snapshot.
const syncResetKey = "sync_reset_seq"

// Property rule (4,449 DCDB parcels plus 12 optional demo homes in demo.json):
//  1. id in the selected seed: kept and restored in place to the selected values
//     (address, geometry, pf_owner, last_sale, flags, data), undeleting it.
//     registered = the seed rule (short p-ids) OR a Pricefinder ownership row
//     exists, because the market import registers the parcels it matches.
//  2. id not in the selected seed but referenced by property_ownership/market_events
//     (Pricefinder data must survive and keeps its FK), or carrying data.lotplan
//     (a parcel imported outside demo.json): kept, phone-only fields stripped,
//     registered only if Pricefinder ownership exists.
//  3. every other row (including demo homes without --with-demo, and phone/app-created "parcel-*" rows,
//     which carry data.lot rather than data.lotplan, and other new ids): deleted.
//
// Kept rows get version=version+1 so any stale queued edit conflicts.
// Never classify by registered alone: phone edits set registered=1.
const demoPropertyIDs = `(SELECT json_extract(value,'$.id') FROM json_each(json_extract(?,'$.properties')))`
const keptProperty = `(EXISTS (SELECT 1 FROM property_ownership o WHERE o.property_id=properties.id) OR EXISTS (SELECT 1 FROM market_events e WHERE e.property_id=properties.id) OR (json_valid(data) AND json_extract(data,'$.lotplan') IS NOT NULL))`

// Zone rule: zones are kept (admin-drawn); demo zones are re-inserted by seedData
// with ON CONFLICT DO NOTHING, so an existing demo zone id is left as it is.
const demoZoneIDs = `(SELECT json_extract(value,'$.id') FROM json_each(json_extract(?,'$.zones')))`

func resetCounts(db *sql.DB, demo string) (map[string]int64, error) {
	counts := make(map[string]int64)
	for _, table := range append(append([]string{}, resetTables...), keptTables...) {
		var n int64
		if err := db.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			return nil, fmt.Errorf("%s: %w", table, err)
		}
		counts[table] = n
	}
	extra := map[string]string{
		"properties: demo/import rows (restored)":              `SELECT count(*) FROM properties WHERE id IN ` + demoPropertyIDs,
		"properties: Pricefinder-linked/other imported (kept)": `SELECT count(*) FROM properties WHERE id NOT IN ` + demoPropertyIDs + ` AND ` + keptProperty + ` AND COALESCE(json_extract(data,'$.source'),'')<>'rex_import'`,
		"properties: app-created (deleted)":                    `SELECT count(*) FROM properties WHERE id NOT IN ` + demoPropertyIDs + ` AND NOT ` + keptProperty + ` AND COALESCE(json_extract(data,'$.source'),'')<>'rex_import'`,
		"properties: rex_import (deleted unless demo)":         `SELECT count(*) FROM properties WHERE id NOT IN ` + demoPropertyIDs + ` AND json_extract(data,'$.source')='rex_import'`,
		"contacts: rex_import (cleared)":                       `SELECT count(*) FROM contacts WHERE json_extract(data,'$.source')='rex_import'`,
		"rex_links: rex_import (cleared)":                      `SELECT count(*) FROM rex_links WHERE local_id IN (SELECT id FROM contacts WHERE json_extract(data,'$.source')='rex_import' UNION SELECT id FROM properties WHERE json_extract(data,'$.source')='rex_import')`,
		"rex_outbox: all (cleared)":                            `SELECT count(*) FROM rex_outbox`,
		"property_owners: all (cleared then demo restored)":    `SELECT count(*) FROM property_owners`,
		"zones: demo":               `SELECT count(*) FROM zones WHERE id IN ` + demoZoneIDs,
		"zones: admin-drawn (kept)": `SELECT count(*) FROM zones WHERE id NOT IN ` + demoZoneIDs,
	}
	for k, q := range extra {
		var n int64
		args := []any{}
		for i := 0; i < strings.Count(q, "?"); i++ {
			args = append(args, demo)
		}
		if err := db.QueryRow(q, args...).Scan(&n); err != nil {
			return nil, fmt.Errorf("%s: %w", k, err)
		}
		counts[k] = n
	}
	files, err := photoFiles()
	if err != nil {
		return nil, err
	}
	counts["photo files"] = int64(len(files))
	return counts, nil
}

var resetDetailKeys = []string{
	"properties: demo/import rows (restored)", "properties: Pricefinder-linked/other imported (kept)", "properties: app-created (deleted)",
	"properties: rex_import (deleted unless demo)", "contacts: rex_import (cleared)", "rex_links: rex_import (cleared)", "rex_outbox: all (cleared)", "property_owners: all (cleared then demo restored)",
	"zones: demo", "zones: admin-drawn (kept)", "photo files",
}

func photoFiles() ([]string, error) {
	entries, err := os.ReadDir(photoDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, filepath.Join(photoDir(), e.Name()))
		}
	}
	return out, nil
}

func pendingMigrations(db *sql.DB) ([]string, error) {
	entries, err := assets.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	var exists int
	if err = db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'`).Scan(&exists); err != nil {
		return nil, err
	}
	pending := []string{}
	for _, e := range entries {
		n := 0
		if exists > 0 {
			if err = db.QueryRow(`SELECT count(*) FROM schema_migrations WHERE version=?`, e.Name()).Scan(&n); err != nil {
				return nil, err
			}
		}
		if n == 0 {
			pending = append(pending, e.Name())
		}
	}
	return pending, nil
}

func printCounts(title string, before, after map[string]int64) {
	fmt.Println(title)
	row := func(k string) {
		if after == nil {
			fmt.Printf("  %-55s %d\n", k, before[k])
		} else {
			fmt.Printf("  %-55s %d -> %d\n", k, before[k], after[k])
		}
	}
	fmt.Println(" cleared (optional demo rows re-inserted):")
	for _, t := range resetTables {
		row(t)
	}
	fmt.Println(" kept:")
	for _, t := range keptTables {
		row(t)
	}
	for _, k := range resetDetailKeys {
		row(k)
	}
}

// resetCommand keeps the original preview/reset behaviour unless explicitly asked
// to import. Import failures do not undo the already committed, backed-up reset.
func resetCommand(db *sql.DB, path string, args []string) error {
	yes, importRex, withDemo := false, false, false
	seen := map[string]bool{}
	for _, arg := range args {
		if seen[arg] {
			return fmt.Errorf("duplicate option %s", arg)
		}
		seen[arg] = true
		switch arg {
		case "--yes":
			yes = true
		case "--rex-import":
			importRex = true
		case "--with-demo":
			withDemo = true
		default:
			return fmt.Errorf("usage: axleo reset-data [--yes] [--rex-import] [--with-demo]")
		}
	}
	if importRex && !yes {
		return fmt.Errorf("--rex-import requires --yes")
	}
	if err := resetData(db, path, yes, withDemo); err != nil {
		return err
	}
	if !importRex {
		return nil
	}
	account := loadRexConfig().AccountID
	if err := rexImportCLI([]string{"--account", account}); err != nil {
		fmt.Println("未导入：", err)
		return fmt.Errorf("reset completed but Rex import failed: %w", err)
	}
	return nil
}

// resetData restores parcels (and optionally demo fixtures). Without yes it only reports.
// The caller must hold serviceLock so the service cannot write concurrently.
func resetData(db *sql.DB, path string, yes bool, withDemo bool) error {
	pending, err := pendingMigrations(db)
	if err != nil {
		return err
	}
	if len(pending) > 0 {
		return fmt.Errorf("database schema is not current (pending %s); run axleo migrate first", strings.Join(pending, ", "))
	}
	var accounts int
	if err = db.QueryRow(`SELECT count(*) FROM users WHERE email IN ('mary@axleo.local','john@axleo.local')`).Scan(&accounts); err != nil {
		return err
	}
	if accounts != 2 {
		return fmt.Errorf("mary@axleo.local and john@axleo.local are required for the demo data")
	}
	raw, err := assets.ReadFile("seed/demo.json")
	if err != nil {
		return err
	}
	var selected struct {
		Properties []map[string]any `json:"properties"`
		Zones      []map[string]any `json:"zones"`
	}
	if err = json.Unmarshal(raw, &selected); err != nil {
		return err
	}
	if !withDemo {
		selected.Properties = parcelProperties(selected.Properties)
	}
	selection, err := json.Marshal(selected)
	if err != nil {
		return err
	}
	demo := string(selection)
	before, err := resetCounts(db, demo)
	if err != nil {
		return err
	}
	if !yes {
		printCounts("DRY RUN - current row counts:", before, nil)
		fmt.Println("No changes made. Re-run as: axleo reset-data --yes (takes a verified backup first).")
		return nil
	}
	dir := env("AXLEO_BACKUP_DIR", filepath.Join(filepath.Dir(path), "backup"))
	saved, err := backup(db, dir)
	if err != nil {
		return fmt.Errorf("backup failed, nothing reset: %w", err)
	}
	if _, err = verify(saved); err != nil {
		return fmt.Errorf("backup verification failed (%s), nothing reset: %w", saved, err)
	}
	fmt.Println("Verified backup:", saved)
	if err = resetTx(db, demo, withDemo); err != nil {
		return fmt.Errorf("reset rolled back, nothing changed: %w", err)
	}
	// All photo rows are gone, so every stored file is now orphaned. Removing
	// them after commit means a crash can only leave orphans, never dangling rows.
	files, err := photoFiles()
	if err != nil {
		return fmt.Errorf("reset committed; listing photo files failed: %w", err)
	}
	for _, f := range files {
		if err = os.Remove(f); err != nil {
			return fmt.Errorf("reset committed; photo cleanup failed: %w", err)
		}
	}
	after, err := resetCounts(db, demo)
	if err != nil {
		return err
	}
	printCounts("Reset complete (before -> after):", before, after)
	fmt.Println("All sessions revoked; every user must log in again.")
	return nil
}

func resetTx(db *sql.DB, demo string, withDemo bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lastSeq int64
	if err = tx.QueryRow(`SELECT COALESCE((SELECT seq FROM sqlite_sequence WHERE name='sync_changes'),0)`).Scan(&lastSeq); err != nil {
		return err
	}
	var logged int64
	if err = tx.QueryRow(`SELECT COALESCE(max(seq),0) FROM sync_changes`).Scan(&logged); err != nil {
		return err
	}
	if logged > lastSeq {
		lastSeq = logged
	}
	for _, table := range resetTables {
		if _, err = tx.Exec(`DELETE FROM ` + table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	if !withDemo {
		if _, err = tx.Exec(`DELETE FROM zones WHERE id IN `+demoZoneIDs, demo); err != nil {
			return fmt.Errorf("clear demo zones: %w", err)
		}
	}
	// Change-log AUTOINCREMENT continues after lastSeq, so new cursors stay above it.
	if _, err = tx.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, syncResetKey, strconv.FormatInt(lastSeq, 10)); err != nil {
		return err
	}
	// Rex imports may carry a lotplan. Never mistake those for DCDB parcels.
	if _, err = tx.Exec(`DELETE FROM properties WHERE id NOT IN `+demoPropertyIDs+` AND (json_extract(data,'$.source')='rex_import' OR NOT `+keptProperty+`)`, demo); err != nil {
		return fmt.Errorf("delete app properties: %w", err)
	}
	stamp := now()
	if _, err = tx.Exec(`UPDATE properties SET registered=EXISTS (SELECT 1 FROM property_ownership o WHERE o.property_id=properties.id), deleted_at=NULL, version=version+1, updated_at=?, flags=NULL, pf_owner='', last_sale='',
		data=json_remove(data,'$.ownerIds','$.previousOwnerIds','$.visits','$.flags','$.notes','$.photos','$.sellerLeadStatus','$.otherListing','$.pfOwner','$.lastSale')
		WHERE id NOT IN `+demoPropertyIDs, stamp, demo); err != nil {
		return fmt.Errorf("reset kept properties: %w", err)
	}
	if !withDemo {
		// A demo home can remain only when a Pricefinder FK references it.
		// Remove the prototype measurements and marketing details from that row.
		if _, err = tx.Exec(`UPDATE properties SET deleted_at=?, registered=0, data=json_remove(data,'$.beds','$.baths','$.cars','$.land','$.lot','$.ll','$.appraisal','$.pending','$.pinned','$.otherAgent','$.otherListing') WHERE id GLOB 'p[0-9]*' AND json_extract(data,'$.lotplan') IS NULL`, stamp); err != nil {
			return fmt.Errorf("strip retained demo homes: %w", err)
		}
	}
	var seed struct {
		Properties []map[string]any `json:"properties"`
	}
	if err = json.Unmarshal([]byte(demo), &seed); err != nil {
		return err
	}
	restore, err := tx.Prepare(`UPDATE properties SET no=?,street=?,suburb=?,geometry=?,pf_owner=?,last_sale=?,flags=?,data=?,
		registered=(? OR EXISTS (SELECT 1 FROM property_ownership o WHERE o.property_id=properties.id)),
		version=version+1,updated_at=?,deleted_at=NULL WHERE id=?`)
	if err != nil {
		return err
	}
	defer restore.Close()
	for _, m := range seed.Properties {
		id := str(m, "id")
		// Same normalisation as seedData: the phone map needs ll, not only lat/lng.
		ensurePropertyLL(m)
		payload, e := json.Marshal(m)
		if e != nil {
			return e
		}
		if _, err = restore.Exec(str(m, "no"), str(m, "street"), str(m, "suburb"), jsonField(m, "geometry"), str(m, "pfOwner"), str(m, "lastSale"), jsonField(m, "flags"), string(payload), strings.HasPrefix(id, "p") && len(id) <= 3, stamp, id); err != nil {
			return fmt.Errorf("restore property %s: %w", id, err)
		}
	}
	// Same inserts as `axleo seed`: parcels by default; demo business rows only
	// when explicitly requested. Accounts are untouched.
	if err = seedData(tx, withDemo); err != nil {
		return err
	}
	check, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	var violation []string
	for check.Next() {
		var table, parent string
		var rowid sql.NullInt64
		var fk int
		if err = check.Scan(&table, &rowid, &parent, &fk); err != nil {
			check.Close()
			return err
		}
		violation = append(violation, fmt.Sprintf("%s row %d -> %s", table, rowid.Int64, parent))
	}
	check.Close()
	if len(violation) > 0 {
		return fmt.Errorf("foreign key check failed: %s", strings.Join(violation, "; "))
	}
	return tx.Commit()
}
