package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

func startRexPoller(ctx context.Context, db *sql.DB) {
	cfg := loadRexConfig()
	if !cfg.canRead() {
		return
	}
	c := newRexClient(cfg.BaseURL, cfg.AppID)
	c.ReadOnly, c.AccountID, c.Caller, c.Password, c.Log = true, cfg.AccountID, "poller", cfg.Password, rexDBLogger(db)
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

func pollRex(ctx context.Context, db *sql.DB, c *rexClient, account string) error {
	ids, err := sandboxUsers(db)
	if err != nil {
		return err
	}
	newProps, newContacts := map[string]bool{}, map[string]bool{}
	for _, kind := range []string{"properties", "contacts"} {
		service := map[string]string{"properties": "Properties", "contacts": "Contacts"}[kind]
		seen := map[string]bool{}
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
				if local == "" {
					added := newContacts
					if kind == "properties" {
						added = newProps
					}
					added["rex-"+kind+"-"+account+"-"+rid] = true
				}
				stats := &importStats{}
				if e = importSave(db, kind, account, detail, ids["admin"], false, stats); e != nil {
					return e
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
			rows, e := db.Query(`SELECT local_id,rex_id FROM rex_links WHERE entity=? AND account_id=?`, kind, account)
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
				_, e = db.Exec(`UPDATE `+kind+` SET deleted_at=?,updated_at=?,version=version+1 WHERE id=? AND deleted_at IS NULL`, now(), now(), p.local)
				if e != nil {
					return e
				}
				_, _ = db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,?,?,?,?,?,?)`, now(), "rex_to_app", kind, p.local, p.rex, ijson(map[string]any{"deleted_at": map[string]any{"old": nil, "new": now()}}), "ok")
			}
		}
	}
	return assignSandbox(db, ids, newProps, newContacts)
}
