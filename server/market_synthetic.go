package main

import (
	"database/sql"
	"path/filepath"
	"strconv"
	"strings"
)

// Synthetic RP Data exports (server/cmd/gen-fake-rpdata) are named
// SYNTHETIC_<Suburb>_RPData_propertyExport_<timestamp>.xlsx. They are test-server
// fixtures only: imports from them are flagged and can be purged in one step
// before licensed data is loaded.
const syntheticMarketPrefix = "SYNTHETIC_"

func syntheticMarketFile(filename string) bool {
	return strings.HasPrefix(filepath.Base(filename), syntheticMarketPrefix)
}

// syntheticImports selects market_imports rows created from synthetic files.
const syntheticImports = `SELECT id FROM market_imports WHERE substr(filename,1,10)='` + syntheticMarketPrefix + `'`

type syntheticPurge struct {
	Imports, Events, Alerts, Ownership, Properties int
}

// purgeSyntheticMarket removes every trace of synthetic RP Data imports:
// alerts (soft-deleted so phones receive tombstones), market events, ownership
// rows and rpdataAttributes written from SYNTHETIC_ files, then the import
// records. Data from real imports is kept, including rows a later real import
// took over. Phones are sent a full snapshot on their next pull so properties
// that leave the sync scope disappear from them.
func purgeSyntheticMarket(db *sql.DB, dry bool, p principal) (syntheticPurge, error) {
	var c syntheticPurge
	tx, err := db.Begin()
	if err != nil {
		return c, err
	}
	defer tx.Rollback()
	count := func(q string, n *int) error { return tx.QueryRow(q).Scan(n) }
	steps := []struct {
		count, exec string
		n           *int
	}{
		{`SELECT count(*) FROM market_items WHERE deleted_at IS NULL AND event_id IN (SELECT id FROM market_events WHERE import_id IN (` + syntheticImports + `))`,
			`UPDATE market_items SET deleted_at=?1,updated_at=?1,version=version+1 WHERE deleted_at IS NULL AND event_id IN (SELECT id FROM market_events WHERE import_id IN (` + syntheticImports + `))`, &c.Alerts},
		{`SELECT count(*) FROM market_events WHERE import_id IN (` + syntheticImports + `)`,
			`DELETE FROM market_events WHERE import_id IN (` + syntheticImports + `)`, &c.Events},
		{`SELECT count(*) FROM property_ownership WHERE import_id IN (` + syntheticImports + `)`,
			`DELETE FROM property_ownership WHERE import_id IN (` + syntheticImports + `)`, &c.Ownership},
		{`SELECT count(*) FROM properties WHERE json_valid(data) AND json_extract(data,'$.rpdataAttributes.synthetic')='true'`,
			`UPDATE properties SET data=json_remove(data,'$.rpdataAttributes'),updated_at=?1,version=version+1 WHERE json_valid(data) AND json_extract(data,'$.rpdataAttributes.synthetic')='true'`, &c.Properties},
		// Nothing references the import rows any more. Deleting them (not just
		// marking) lets the same generated file be imported again after a purge.
		{`SELECT count(*) FROM market_imports WHERE id IN (` + syntheticImports + `)`,
			`DELETE FROM market_imports WHERE id IN (` + syntheticImports + `)`, &c.Imports},
	}
	stamp := now()
	for _, s := range steps {
		if err = count(s.count, s.n); err != nil {
			return c, err
		}
		if !dry && *s.n > 0 {
			args := []any{}
			if strings.Contains(s.exec, "?1") {
				args = append(args, stamp)
			}
			if _, err = tx.Exec(s.exec, args...); err != nil {
				return c, err
			}
		}
	}
	if dry {
		return c, nil
	}
	if c.Ownership+c.Properties > 0 {
		var high int64
		if err = tx.QueryRow(`SELECT COALESCE(max(seq),0) FROM sync_changes`).Scan(&high); err != nil {
			return c, err
		}
		if _, err = tx.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, syncResetKey, strconv.FormatInt(high, 10)); err != nil {
			return c, err
		}
	}
	if err = (&app{db: db}).record(tx, p, "DELETE", "market_imports", "synthetic", map[string]any{"imports": c.Imports, "events": c.Events, "alerts": c.Alerts, "ownership": c.Ownership, "properties": c.Properties}); err != nil {
		return c, err
	}
	return c, tx.Commit()
}
