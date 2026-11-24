package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type importStats struct{ Added, Updated, Skipped int }
type importRow map[string]any

func imap(v any) importRow { m, _ := v.(map[string]any); return m }
func istring(v any) string {
	if v == nil {
		return ""
	}
	// Rex returns ids as JSON numbers from read but strings from search; never format as 2.8e+07.
	if f, ok := v.(float64); ok && f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatInt(int64(f), 10)
	}
	return fmt.Sprint(v)
}
func iv(m importRow, key string) string { return istring(m[key]) }
func ibool(v any) bool                  { return v == true || v == "1" || v == "true" || v == float64(1) }
func iopt(v any) int {
	if v == nil || ibool(v) {
		return 1
	}
	return 0
}
func ijson(v any) string { b, _ := json.Marshal(v); return string(b) }

// Do not start a login (or modify the database) until the account and read gate are verified.
func rexImportCLI(args []string) error {
	f := flag.NewFlagSet("rex-import", flag.ContinueOnError)
	account := f.String("account", "", "Rex sandbox account")
	dry := f.Bool("dry-run", false, "read without changing business tables")
	limit := f.Int("limit", 0, "maximum records per service (0 = all)")
	assign := f.String("assign", "", "legacy round-robin: two agent emails, comma-separated")
	plan := f.String("plan", "sandbox-test", "sandbox-test or legacy")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 || *limit < 0 || (*plan != "sandbox-test" && *plan != "legacy") {
		return fmt.Errorf("invalid rex-import arguments")
	}
	cfg := loadRexConfig()
	if !cfg.canRead() || *account == "" || *account != cfg.AccountID {
		return fmt.Errorf("rex-import requires AXLEO_REX_READ=1 and --account matching REX_ACCOUNT_ID: %s", cfg.ReadReason)
	}
	db, err := open(env("AXLEO_DB", "./axleo.db"))
	if err != nil {
		return err
	}
	defer db.Close()
	if err = migrate(db, env("AXLEO_DB", "./axleo.db")); err != nil {
		return err
	}
	c := newRexClient(cfg.BaseURL, cfg.AppID)
	c.ReadOnly = true
	c.AccountID = *account
	c.Caller = "import"
	c.Password = cfg.Password
	c.Log = rexDBLogger(db)
	if ms, e := strconv.Atoi(os.Getenv("REX_INTERVAL_MS")); e == nil && ms >= 0 {
		c.Interval = time.Duration(ms) * time.Millisecond
	}
	ctx := context.Background()
	if *assign == "" && *plan == "sandbox-test" {
		ids, err := sandboxUsers(db)
		if err != nil {
			return err
		}
		if _, err = c.Login(ctx, cfg.Email, cfg.Password, *account); err != nil {
			return err
		}
		return runSandboxImport(ctx, db, c, *account, *dry, *limit, ids)
	}
	users := []string{}
	if *assign != "" {
		users = strings.Split(*assign, ",")
		if len(users) != 2 {
			return fmt.Errorf("--assign requires exactly two emails")
		}
	} else {
		users = []string{"mary", "john"}
	}
	for i, u := range users {
		var id string
		if *assign != "" {
			err = db.QueryRow(`SELECT id FROM users WHERE email=? AND disabled=0 AND deleted_at IS NULL`, strings.TrimSpace(u)).Scan(&id)
		} else {
			err = db.QueryRow(`SELECT id FROM users WHERE lower(name)=? AND disabled=0 AND deleted_at IS NULL`, u).Scan(&id)
		}
		if err != nil {
			return fmt.Errorf("agent %q not found: %w", u, err)
		}
		users[i] = id
	}
	if users[0] == users[1] {
		return fmt.Errorf("two distinct agents required")
	}
	if _, err = c.Login(ctx, cfg.Email, cfg.Password, *account); err != nil {
		return err
	}
	return runRexImport(ctx, db, c, *account, *dry, *limit, users)
}

func importPages(ctx context.Context, c *rexClient, service string, limit int, consume func(importRow) error) error {
	count := 0
	for offset := 0; ; offset += 100 {
		size := 100
		if limit > 0 && limit-count < size {
			size = limit - count
		}
		if size <= 0 {
			return nil
		}
		body := map[string]any{"limit": size, "offset": offset, "order_by": map[string]any{"id": "asc"}} // Rex: {"field":"direction"}, verified against 2057
		if service == "Properties" {
			body["extra_options"] = map[string]any{"extra_fields": []string{"contact_reln_property"}}
		}
		_, en, err := c.Call(ctx, service, "search", body)
		if err != nil {
			return err
		}
		var result struct {
			Rows  []importRow `json:"rows"`
			Total int         `json:"total"`
		}
		if err = json.Unmarshal(en.Result, &result); err != nil {
			return fmt.Errorf("%s search result: %w", service, err)
		}
		if len(result.Rows) == 0 {
			return nil
		}
		for _, row := range result.Rows {
			if iv(row, "id") == "" {
				return fmt.Errorf("%s search returned row without id", service)
			}
			if err = consume(row); err != nil {
				return err
			}
			count++
		}
		if len(result.Rows) < size || (result.Total > 0 && offset+len(result.Rows) >= result.Total) {
			return nil
		}
	}
}

func runRexImport(ctx context.Context, db *sql.DB, c *rexClient, account string, dry bool, limit int, users []string) error {
	if len(users) != 2 || users[0] == users[1] {
		return fmt.Errorf("two distinct agents required")
	}
	c.ReadOnly = true
	c.Caller = "import"
	c.AccountID = account
	stats := map[string]*importStats{"contacts": {}, "properties": {}}
	// Read properties first to derive contact assignments; defer relation writes until contacts exist.
	props := []importRow{}
	if err := importPages(ctx, c, "Properties", limit, func(r importRow) error { props = append(props, r); return nil }); err != nil {
		return err
	}
	relatedOwners := map[string]string{}
	contacts := []importRow{}
	if err := importPages(ctx, c, "Contacts", limit, func(r importRow) error { contacts = append(contacts, r); return nil }); err != nil {
		return err
	}
	for i, r := range props {
		if err := importSave(db, "properties", account, r, users[i%2], dry, stats["properties"]); err != nil {
			return err
		}
		owner := users[i%2]
		if !dry {
			if err := db.QueryRow(`SELECT p.owner_id FROM properties p JOIN rex_links l ON l.local_id=p.id AND l.entity='properties' WHERE l.account_id=? AND l.rex_id=?`, account, iv(r, "id")).Scan(&owner); err != nil {
				return err
			}
		}
		for _, rel := range importRelations(r) {
			id := iv(imap(rel["contact"]), "id")
			if id != "" {
				if _, found := relatedOwners[id]; !found {
					relatedOwners[id] = owner
				}
			}
		}
	}
	for i, r := range contacts {
		owner := relatedOwners[iv(r, "id")]
		if owner == "" {
			owner = users[i%2]
		}
		if err := importSave(db, "contacts", account, r, owner, dry, stats["contacts"]); err != nil {
			return err
		}
	}
	if !dry {
		for _, r := range props {
			if err := saveImportRelations(db, account, r); err != nil {
				return err
			}
		}
	}
	for _, kind := range []string{"properties", "contacts"} {
		s := stats[kind]
		fmt.Printf("%s: added=%d updated=%d skipped=%d dry-run=%t\n", kind, s.Added, s.Updated, s.Skipped, dry)
	}
	return nil
}

func importRelations(r importRow) []importRow {
	raw, _ := imap(r["related"])["contact_reln_property"].([]any)
	out := make([]importRow, 0, len(raw))
	for _, v := range raw {
		out = append(out, imap(v))
	}
	return out
}

func importSave(db *sql.DB, kind, account string, r importRow, owner string, dry bool, s *importStats) error {
	rid := iv(r, "id")
	etag := iv(r, "etag")
	var id, prev string
	err := db.QueryRow(`SELECT local_id,etag FROM rex_links WHERE entity=? AND account_id=? AND rex_id=?`, kind, account, rid).Scan(&id, &prev)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	exists := err == nil
	if exists && etag != "" && etag == prev {
		s.Skipped++
		return nil
	}
	if exists {
		s.Updated++
	} else {
		s.Added++
		id = "rex-" + kind + "-" + account + "-" + rid
	}
	if dry {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stamp := now()
	if kind == "contacts" {
		first, last := iv(r, "name_first"), iv(r, "name_last")
		if first == "" && last == "" {
			names, _ := imap(r["related"])["contact_names"].([]any)
			if len(names) > 0 {
				first = iv(imap(names[0]), "name_first")
				last = iv(imap(names[0]), "name_last")
			}
		}
		if exists {
			var existingOwner sql.NullString
			if err = tx.QueryRow(`SELECT owner_id FROM contacts WHERE id=?`, id).Scan(&existingOwner); err != nil {
				return err
			}
			owner = existingOwner.String
		}
		data := importRow{"id": id, "first": first, "last": last, "mobile": iv(r, "phone_number"), "email": iv(r, "email_address"), "address": iv(r, "address"), "owner": owner, "roles": []string{}, "tags": []string{}, "notes": []any{}, "rels": []any{}, "source": "rex_import", "flags": map[string]any{"dnc": ibool(r["is_dnd"])}, "consent": map[string]any{"letter": iopt(r["contact_by_letter"]), "email": iopt(r["contact_by_email"]), "phone": iopt(r["contact_by_phone"]), "sms": iopt(r["contact_by_sms"])}, "rex": r}
		if exists {
			if err = mergeImportData(tx, "contacts", id, data); err != nil {
				return err
			}
			_, err = tx.Exec(`UPDATE contacts SET first=?,last=?,mobile=?,email=?,flags=?,consent=?,data=?,updated_at=?,version=version+1,deleted_at=NULL WHERE id=?`, first, last, data["mobile"], data["email"], ijson(data["flags"]), ijson(data["consent"]), ijson(data), stamp, id)
		} else {
			_, err = tx.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,email,flags,consent,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, owner, first, last, data["mobile"], data["email"], ijson(data["flags"]), ijson(data["consent"]), ijson(data), stamp, stamp)
		}
	} else {
		no, street, suburb := iv(r, "adr_street_number"), iv(r, "adr_street_name"), iv(r, "adr_suburb_or_town")
		data := importRow{"id": id, "no": no, "street": street, "suburb": suburb, "address": strings.TrimSpace(no + " " + street + ", " + suburb), "state": r["adr_state_or_region"], "postcode": r["adr_postcode"], "unit": r["adr_unit_number"], "lat": r["adr_latitude"], "lng": r["adr_longitude"], "beds": r["attr_bedrooms"], "baths": r["attr_bathrooms"], "cars": r["attr_garages"], "flags": map[string]any{"dnc": ibool(r["is_dnd"])}, "ownerIds": []string{}, "pfOwner": "", "lastSale": "", "source": "rex_import", "rex": r}
		lat, e1 := strconv.ParseFloat(iv(r, "adr_latitude"), 64)
		lng, e2 := strconv.ParseFloat(iv(r, "adr_longitude"), 64)
		if e1 == nil && e2 == nil && lat >= -44 && lat <= -10 && lng >= 112 && lng <= 154 {
			data["ll"] = []float64{lat, lng}
		} // no parcel geometry: map as point only
		if exists {
			if err = mergeImportData(tx, "properties", id, data); err != nil {
				return err
			}
			_, err = tx.Exec(`UPDATE properties SET no=?,street=?,suburb=?,data=?,updated_at=?,version=version+1,deleted_at=NULL WHERE id=?`, no, street, suburb, ijson(data), stamp, id)
		} else {
			_, err = tx.Exec(`INSERT INTO properties(id,owner_id,no,street,suburb,data,registered,created_at,updated_at) VALUES(?,?,?,?,?,?,1,?,?)`, id, owner, no, street, suburb, ijson(data), stamp, stamp)
		}
	}
	if err != nil {
		return err
	}
	if exists {
		_, err = tx.Exec(`UPDATE rex_links SET etag=? WHERE entity=? AND local_id=?`, etag, kind, id)
	} else {
		_, err = tx.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id,etag) VALUES(?,?,?,?,?,?,?)`, kind, id, rid, map[string]string{"contacts": "Contacts", "properties": "Properties"}[kind], stamp, account, etag)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func saveImportRelations(db *sql.DB, account string, r importRow) error {
	var pid string
	if err := db.QueryRow(`SELECT local_id FROM rex_links WHERE entity='properties' AND account_id=? AND rex_id=?`, account, iv(r, "id")).Scan(&pid); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM property_owners WHERE property_id=?`, pid); err != nil {
		return err
	}
	for _, rel := range importRelations(r) {
		var cid string
		err = tx.QueryRow(`SELECT local_id FROM rex_links WHERE entity='contacts' AND account_id=? AND rex_id=?`, account, iv(imap(rel["contact"]), "id")).Scan(&cid)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		relation := iv(imap(rel["reln_type"]), "text")
		if relation == "" {
			relation = iv(imap(rel["reln_type"]), "id")
		}
		if relation == "" {
			relation = "owner"
		}
		if _, err = tx.Exec(`INSERT OR REPLACE INTO property_owners(property_id,contact_id,relation) VALUES(?,?,?)`, pid, cid, relation); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// mergeImportData keeps app-side fields (notes, tags, roles, rels, geometry, ...) on
// re-import/poll; only Rex-owned keys are overwritten. flags are merged key by key.
func mergeImportData(tx *sql.Tx, table, id string, data importRow) error {
	var raw string
	if err := tx.QueryRow(`SELECT data FROM `+table+` WHERE id=?`, id).Scan(&raw); err != nil {
		return err
	}
	var old importRow
	if json.Unmarshal([]byte(raw), &old) != nil || old == nil {
		return nil
	}
	owned := map[string]bool{"id": true, "rex": true, "source": true}
	keys := []string{"first", "last", "mobile", "email", "address", "consent"}
	if table == "properties" {
		keys = []string{"no", "street", "suburb", "address", "state", "postcode", "unit", "lat", "lng", "beds", "baths", "cars"}
	}
	for _, k := range keys {
		owned[k] = true
	}
	for k, v := range old {
		if k == "flags" {
			merged := imap(v)
			if merged == nil {
				merged = importRow{}
			}
			for fk, fv := range imap(data["flags"]) {
				merged[fk] = fv
			}
			data["flags"] = merged
			continue
		}
		if k == "ll" && data[k] != nil { // Rex coordinates take precedence when present.
			continue
		}
		if !owned[k] {
			data[k] = v
		}
	}
	return nil
}
