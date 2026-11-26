package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

func startRexPoller(ctx context.Context, db *sql.DB) {
	cfg := loadRexConfig()
	if !cfg.canRead() {
		return
	}
	c := newRexClient(cfg.BaseURL, cfg.AppID)
	c.ReadOnly, c.AccountID, c.Caller, c.Password, c.Log = true, cfg.AccountID, "poller", cfg.Password, rexDBLogger(db)
	c.Gate = rexAccountGate // shared account quota with the worker and contact search
	seconds, _ := strconv.Atoi(os.Getenv("REX_POLL_SECONDS"))
	if seconds == 0 {
		seconds = 60
	} else if seconds < 30 {
		seconds = 30
	}
	go func() {
		ticker := time.NewTicker(time.Duration(seconds) * time.Second)
		defer ticker.Stop()
		for {
			if c.Token == "" {
				_, _ = c.Login(ctx, cfg.Email, cfg.Password, cfg.AccountID)
			}
			if c.Token != "" {
				err := pollRex(ctx, db, c, cfg.AccountID)
				ok, reason := 1, ""
				if err != nil {
					ok, reason = 0, err.Error()
				}
				_, _ = db.Exec(`INSERT INTO rex_poll_status(id,at,ok,error) VALUES(1,?,?,?) ON CONFLICT(id) DO UPDATE SET at=excluded.at,ok=excluded.ok,error=excluded.error`, now(), ok, reason)
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

var relationPollMu sync.Mutex
var lastRelationPoll time.Time

func pollRex(ctx context.Context, db *sql.DB, c *rexClient, account string) error {
	ids, err := sandboxUsers(db)
	if err != nil {
		return err
	}
	newProps, newContacts := map[string]bool{}, map[string]bool{}
	changedProps := []importRow{}
	for _, kind := range []string{"properties", "contacts"} {
		service := map[string]string{"properties": "Properties", "contacts": "Contacts"}[kind]
		seen := map[string]bool{}
		// Links created after the search began (e.g. an app contact created mid-poll)
		// cannot appear in this listing and must not be treated as deleted in Rex.
		searchStarted := now()
		for offset := 0; ; offset += 10000 {
			c.Entity, c.LocalID, c.RexID = kind, "", ""
			_, env, e := c.Call(ctx, service, "search", map[string]any{"limit": 10000, "offset": offset, "result_format": "etags"})
			if e != nil {
				return e
			}
			var page struct {
				Rows  []importRow `json:"rows"`
				Total int         `json:"total"`
			}
			if e = json.Unmarshal(env.Result, &page); e != nil {
				return e
			}
			for _, row := range page.Rows {
				rid, etag := iv(row, "_id"), iv(row, "etag")
				if rid == "" {
					rid = iv(row, "id")
				}
				if rid == "" || etag == "" {
					return fmt.Errorf("%s etag search missing id or etag", service)
				}
				seen[rid] = true
				var local, previous string
				e = db.QueryRow(`SELECT local_id,etag FROM rex_links WHERE entity=? AND account_id=? AND rex_id=?`, kind, account, rid).Scan(&local, &previous)
				if e != nil && e != sql.ErrNoRows {
					return e
				}
				if previous == etag {
					continue
				}
				if local != "" {
					var pending int
					_ = db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity=? AND entity_id=? AND status IN ('pending','waiting_auth')`, kind, local).Scan(&pending)
					if pending > 0 {
						continue
					}
					if kind == "contacts" && mergedAway(db, local) {
						// The link still names a contact merged into another (rex_merge has not
						// moved or combined it yet). importSave would undelete it; the
						// rex_merge job owns this record now.
						continue
					}
				}
				c.Entity, c.LocalID, c.RexID = kind, local, rid
				_, record, e := c.Call(ctx, service, "read", map[string]any{"id": rid})
				if e != nil {
					return e
				}
				var detail importRow
				if e = json.Unmarshal(record.Result, &detail); e != nil {
					return e
				}
				if iv(detail, "id") == "" {
					return fmt.Errorf("%s read missing id", service)
				}
				if iv(detail, "etag") == "" {
					detail["etag"] = etag
				}
				flattenRexRead(detail)
				if local == "" && kind == "contacts" {
					// 问题02: adopt an unlinked same-mobile app contact instead of a second record.
					saved, created, e := rexAdoptOrImport(db, account, detail, ids["admin"])
					if e != nil {
						return e
					}
					if created {
						newContacts[saved] = true
					}
					local = saved
				} else {
					if local == "" {
						newProps["rex-"+kind+"-"+account+"-"+rid] = true
					}
					if e = importSave(db, kind, account, detail, ids["admin"], false, &importStats{}); e != nil {
						return e
					}
				}
				if kind == "properties" {
					changedProps = append(changedProps, detail)
				}
				if local == "" {
					local = "rex-" + kind + "-" + account + "-" + rid
				}
				_, _ = db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, now(), "rex_to_app", kind, local, rid, ijson(map[string]any{"etag": map[string]any{"old": previous, "new": etag}}), "ok")
			}
			if len(page.Rows) < 10000 || (page.Total > 0 && offset+len(page.Rows) >= page.Total) {
				break
			}
		}
		if os.Getenv("REX_POLL_SOFT_DELETE") != "0" {
			rows, e := db.Query(`SELECT local_id,rex_id FROM rex_links WHERE entity=? AND account_id=? AND created_at<?`, kind, account, searchStarted)
			if e != nil {
				return e
			}
			type pair struct{ local, rex string }
			missing := []pair{}
			for rows.Next() {
				var p pair
				if e = rows.Scan(&p.local, &p.rex); e != nil {
					break
				}
				if !seen[p.rex] {
					// No nested queries here: the pool has a single connection and these
					// rows hold it, so a QueryRow inside this loop deadlocks the server.
					missing = append(missing, p)
				}
			}
			if e == nil {
				e = rows.Err()
			}
			rows.Close()
			if e != nil {
				return e
			}
			// A relation target can be readable by id but absent from contact search.
			if kind == "contacts" {
				kept := missing[:0]
				for _, p := range missing {
					var referenced int
					_ = db.QueryRow(`SELECT count(*) FROM property_owners WHERE contact_id=?`, p.local).Scan(&referenced)
					if referenced == 0 {
						kept = append(kept, p)
					}
				}
				missing = kept
			}
			// Guard against a truncated or changed listing wiping local data.
			if len(missing) > 20 && len(missing)*20 > len(seen)+len(missing) {
				return fmt.Errorf("%s poll: %d linked records missing from Rex listing (seen %d); soft delete skipped", kind, len(missing), len(seen))
			}
			for _, p := range missing {
				var pending int
				_ = db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity=? AND entity_id=? AND status IN ('pending','waiting_auth')`, kind, p.local).Scan(&pending)
				if pending > 0 {
					continue
				}
				res, e := db.Exec(`UPDATE `+kind+` SET deleted_at=?,updated_at=?,version=version+1 WHERE id=? AND deleted_at IS NULL`, now(), now(), p.local)
				if e != nil {
					return e
				}
				if n, _ := res.RowsAffected(); n == 0 {
					continue
				}
				_, _ = db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, now(), "rex_to_app", kind, p.local, p.rex, ijson(map[string]any{"deleted_at": map[string]any{"old": nil, "new": now()}}), "ok")
			}
		}
	}
	// Rex etags change on relationship-only edits too. Keep a bounded daily
	// re-read of recent properties as a safety net.
	relationPollMu.Lock()
	interval := 24 * time.Hour
	if n, e := strconv.Atoi(os.Getenv("REX_RELATION_REFRESH_SECONDS")); e == nil && n >= 60 {
		interval = time.Duration(n) * time.Second
	}
	due := time.Since(lastRelationPoll) >= interval
	if due {
		lastRelationPoll = time.Now()
	}
	relationPollMu.Unlock()
	if due {
		limit := 100
		if n, err := strconv.Atoi(os.Getenv("REX_RELATION_REFRESH_LIMIT")); err == nil && n > 0 {
			limit = n
		}
		rows, e := db.Query(`SELECT l.rex_id FROM rex_links l JOIN properties p ON p.id=l.local_id WHERE l.entity='properties' AND l.account_id=? AND p.deleted_at IS NULL AND p.updated_at>=? ORDER BY p.updated_at DESC LIMIT ?`, account, time.Now().UTC().Add(-7*24*time.Hour).Format(layout), limit)
		if e != nil {
			return e
		}
		var idsToRead []string
		for rows.Next() {
			var rid string
			if e = rows.Scan(&rid); e != nil {
				break
			}
			idsToRead = append(idsToRead, rid)
		}
		if e == nil {
			e = rows.Err()
		}
		rows.Close()
		if e != nil {
			return e
		}
		for _, rid := range idsToRead {
			c.Entity, c.LocalID, c.RexID = "properties", "", rid
			_, record, e := c.Call(ctx, "Properties", "read", map[string]any{"id": rid})
			if e != nil {
				return e
			}
			var detail importRow
			if e = json.Unmarshal(record.Result, &detail); e != nil {
				return e
			}
			if iv(detail, "id") != "" {
				changedProps = append(changedProps, detail)
			}
		}
	}
	// Contacts may arrive later than the property search. Read missing relation targets
	// directly so a newly linked owner is not silently discarded.
	for _, row := range changedProps {
		for _, rel := range importRelations(row) {
			rid := iv(imap(rel["contact"]), "id")
			if rid == "" {
				continue
			}
			var local string
			e := db.QueryRow(`SELECT local_id FROM rex_links WHERE entity='contacts' AND account_id=? AND rex_id=?`, account, rid).Scan(&local)
			if e == sql.ErrNoRows {
				c.Entity, c.LocalID, c.RexID = "contacts", "", rid
				_, record, err := c.Call(ctx, "Contacts", "read", map[string]any{"id": rid})
				if err != nil {
					return err
				}
				var detail importRow
				if err = json.Unmarshal(record.Result, &detail); err != nil {
					return err
				}
				if iv(detail, "id") == "" {
					return fmt.Errorf("contact read missing id: %s", rid)
				}
				flattenRexRead(detail)
				saved, created, err := rexAdoptOrImport(db, account, detail, ids["admin"])
				if err != nil {
					return err
				}
				if created {
					newContacts[saved] = true
				}
			} else if e != nil {
				return e
			}
		}
		if err := saveImportRelations(db, account, row); err != nil {
			return err
		}
	}
	if err := assignPollOwners(db, newProps, newContacts); err != nil {
		return err
	}
	if err := assignSandbox(db, ids, newProps, newContacts); err != nil {
		return err
	}
	return pollRexOFI(ctx, db, c, account, time.Now())
}

// flattenRexRead lifts the primary name/phone/email from a Contacts::read result
// (nested under related.*) to the top-level keys that search rows and importSave use.
func flattenRexRead(r importRow) {
	rel := imap(r["related"])
	if rel == nil {
		return
	}
	pick := func(list string, primary string) importRow {
		rows, _ := rel[list].([]any)
		var first importRow
		for _, x := range rows {
			m := imap(x)
			if m == nil {
				continue
			}
			if first == nil {
				first = m
			}
			if primary != "" && ibool(m[primary]) {
				return m
			}
		}
		return first
	}
	if n := pick("contact_names", ""); n != nil {
		if iv(r, "name_first") == "" {
			r["name_first"] = n["name_first"]
		}
		if iv(r, "name_last") == "" {
			r["name_last"] = n["name_last"]
		}
	}
	if p := pick("contact_phones", "phone_primary"); p != nil && iv(r, "phone_number") == "" {
		r["phone_number"] = p["phone_number"]
	}
	if e := pick("contact_emails", "email_primary"); e != nil && iv(r, "email_address") == "" {
		r["email_address"] = e["email_address"]
	}
}
