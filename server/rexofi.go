package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

var brisbane = time.FixedZone("Australia/Brisbane", 10*3600) // Queensland has no DST.

func rexOFIRows(ctx context.Context, c *rexClient, service string) ([]importRow, error) {
	out := []importRow{}
	for offset := 0; ; offset += 100 {
		c.Entity, c.LocalID, c.RexID = "listings", "", ""
		_, env, err := c.Call(ctx, service, "search", map[string]any{"limit": 100, "offset": offset})
		if err != nil {
			return nil, err
		}
		var page struct {
			Rows  []importRow `json:"rows"`
			Total int         `json:"total"`
		}
		if err = json.Unmarshal(env.Result, &page); err != nil {
			return nil, fmt.Errorf("%s search: %w", service, err)
		}
		out = append(out, page.Rows...)
		if len(page.Rows) < 100 || (page.Total > 0 && len(out) >= page.Total) {
			return out, nil
		}
	}
}

func rexOFIRef(v any) string {
	if m := imap(v); m != nil {
		return iv(m, "id")
	}
	return istring(v)
}

// rexListingStatus maps a Listings row to the app status. Rex system_listing_state
// (sandbox dry run: current / sold / leased / withdrawn) is preferred; listing_status
// is the legacy field this importer used before. Only "listed" means a current
// listing of this agency (drafts are current listings that are not yet published);
// anything unknown stays off the map rather than becoming a P marker.
func rexListingStatus(r importRow) (status, state string) {
	state = iv(r, "system_listing_state")
	if m := imap(r["system_listing_state"]); m != nil {
		state = iv(m, "id")
	}
	if state == "" {
		state = iv(r, "listing_status")
	}
	state = strings.ToLower(strings.TrimSpace(state))
	switch strings.ToLower(iv(r, "system_record_state")) {
	case "archived", "trashed", "deleted":
		return "archived", state
	}
	switch state {
	case "current", "listed", "draft", "active", "published", "unpublished", "under_contract", "conditional":
		return "listed", state
	case "sold", "settled", "unconditional":
		return "sold", state
	case "leased":
		return "leased", state
	case "withdrawn", "expired", "lost", "cancelled", "off_market":
		return "withdrawn", state
	case "archived":
		return "archived", state
	}
	return "unknown", state
}

// rexAgent turns a Rex user reference (listing_agent_1/2, event_agent) into
// {name,rexId,userId}. userId is the matching app user (rexLocalUser, no default
// owner fallback); a shared Rex login such as "Marketing Team" keeps its Rex name
// and an empty userId. Results are cached per poll.
func rexAgent(db *sql.DB, cache map[string]map[string]any, v any) map[string]any {
	u := imap(v)
	if u == nil {
		if s := istring(v); s != "" {
			u = importRow{"id": s}
		} else {
			return nil
		}
	}
	name := strings.TrimSpace(iv(u, "name"))
	if name == "" {
		name = strings.TrimSpace(iv(u, "first_name") + " " + iv(u, "last_name"))
	}
	key := iv(u, "id") + "|" + iv(u, "email") + "|" + iv(u, "email_address") + "|" + name
	if key == "|||" {
		return nil
	}
	if hit, ok := cache[key]; ok {
		return hit
	}
	user := rexLocalUser(db, u, "")
	if name == "" && user != "" {
		_ = db.QueryRow(`SELECT name FROM users WHERE id=?`, user).Scan(&name)
	}
	if name == "" {
		name = iv(u, "email")
	}
	out := map[string]any{"name": name, "rexId": iv(u, "id"), "userId": user}
	cache[key] = out
	return out
}
func rexDate(v string) (time.Time, error) {
	if len(v) == 10 {
		return time.ParseInLocation("2006-01-02", v, brisbane)
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}, err
	}
	return t.In(brisbane), nil
}
func rexEvent(e importRow, at time.Time, back, ahead int) (map[string]any, bool) {
	typ := strings.ToLower(strings.TrimSpace(iv(e, "event_type")))
	if m := imap(e["event_type"]); m != nil {
		typ = strings.ToLower(strings.TrimSpace(iv(m, "id")))
	}
	if typ != "open_home" && typ != "ofi" {
		return nil, false
	}
	zone := brisbane
	if name := iv(e, "event_timezone"); name != "" {
		var err error
		zone, err = time.LoadLocation(name)
		if err != nil {
			return nil, false
		}
	}
	day := at.In(zone)
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, zone).AddDate(0, 0, -back)
	to := from.AddDate(0, 0, back+ahead)
	date := iv(e, "event_date")
	var d time.Time
	var err error
	if len(date) == 10 {
		d, err = time.ParseInLocation("2006-01-02", date, zone)
	} else {
		d, err = time.Parse(time.RFC3339, date)
		if err == nil {
			d = d.In(zone)
			d = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, zone)
		}
	}
	if err != nil || d.Before(from) || d.After(to) {
		return nil, false
	}
	clock := iv(e, "event_time")
	if len(clock) >= 5 {
		clock = clock[:5]
	}
	start, err := time.Parse("15:04", clock)
	if err != nil {
		return nil, false
	}
	end := iv(e, "event_end_time")
	if end == "" {
		end = iv(e, "end_time")
	}
	if len(end) >= 5 {
		end = end[:5]
	}
	if _, err = time.Parse("15:04", end); err != nil {
		end = clock // unknown duration: don't invent a slot
		if minutes, err := strconv.Atoi(iv(e, "event_duration_minutes")); err == nil && minutes >= 0 {
			end = start.Add(time.Duration(minutes) * time.Minute).Format("15:04")
		}
	}
	count := 0
	field := env("REX_OFI_COUNT_FIELD", "attendee_count")
	if n, err := strconv.Atoi(iv(e, field)); err == nil && n >= 0 {
		count = n
	}
	return map[string]any{"d": d.Format("2006-01-02"), "time": clock + "–" + end, "attendees": []any{}, "rexEventId": iv(e, "id"), "rexAttendeeCount": count}, true
}

// pollRexOFI reads only Rex search/read APIs; no Rex write or local attendee PII import.
// Search is deliberately unfiltered until the sandbox confirms date/type criteria names.
func pollRexOFI(ctx context.Context, db *sql.DB, c *rexClient, account string, at time.Time) error {
	if !loadRexConfig().canRead() {
		return nil
	}
	previous := time.Time{}
	var last string
	_ = db.QueryRow(`SELECT at FROM rex_sync_events WHERE entity='ofi_poll' AND rex_id=? AND result='ok' ORDER BY id DESC LIMIT 1`, account).Scan(&last)
	if last != "" {
		previous, _ = time.Parse(layout, last)
	}
	if at.Sub(previous) < 5*time.Minute && !previous.IsZero() {
		return nil
	}
	// Search is unfiltered, so the window changes stored/synced events, not Rex calls.
	back, ahead := 1, 90
	if n, e := strconv.Atoi(os.Getenv("REX_OFI_PAST_DAYS")); e == nil && n >= 0 && n <= 365 {
		back = n
	}
	if n, e := strconv.Atoi(os.Getenv("REX_OFI_FUTURE_DAYS")); e == nil && n >= 0 && n <= 365 {
		ahead = n
	}
	day := at.In(brisbane)
	from := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, brisbane).AddDate(0, 0, -back)
	to := from.AddDate(0, 0, back+ahead)
	listings, err := rexOFIRows(ctx, c, "Listings")
	if err != nil {
		return err
	}
	events, err := rexOFIRows(ctx, c, "ListingEvents")
	if err != nil {
		return err
	}
	grouped := map[string][]map[string]any{}
	agents := map[string]map[string]any{}
	for _, e := range events {
		if ev, ok := rexEvent(e, at, back, ahead); ok {
			if rid := rexOFIRef(e["listing"]); rid != "" {
				if a := rexAgent(db, agents, e["event_agent"]); a != nil {
					ev["agent"], ev["agentId"], ev["rexAgentId"] = a["name"], a["userId"], a["rexId"]
				}
				grouped[rid] = append(grouped[rid], ev)
			}
		}
	}
	for _, r := range listings {
		rid := iv(r, "id")
		if rid == "" {
			return errors.New("Listings search missing id")
		}
		propertyRex := rexOFIRef(r["property"])
		if propertyRex == "" {
			propertyRex = iv(r, "property_id")
		}
		if propertyRex == "" {
			continue
		} // No address to show; do not create a broken listing.
		var propertyID, owner, team string
		err = db.QueryRow(`SELECT p.id,COALESCE(p.owner_id,''),COALESCE(u.team_id,'default') FROM rex_links l JOIN properties p ON p.id=l.local_id LEFT JOIN users u ON u.id=p.owner_id WHERE l.entity='properties' AND l.account_id=? AND l.rex_id=? AND p.deleted_at IS NULL`, account, propertyRex).Scan(&propertyID, &owner, &team)
		if errors.Is(err, sql.ErrNoRows) {
			c.Entity, c.RexID = "properties", propertyRex
			_, resp, e := c.Call(ctx, "Properties", "read", map[string]any{"id": propertyRex})
			if e != nil {
				return e
			}
			var property importRow
			if e = json.Unmarshal(resp.Result, &property); e != nil {
				return e
			}
			if iv(property, "id") == "" {
				return fmt.Errorf("Properties read %s missing id", propertyRex)
			}
			var admin string
			if e = db.QueryRow(`SELECT id FROM users WHERE role='admin' AND disabled=0 AND deleted_at IS NULL LIMIT 1`).Scan(&admin); e != nil {
				return e
			}
			if e = importSave(db, "properties", account, property, admin, false, &importStats{}); e != nil {
				return e
			}
			if e = db.QueryRow(`SELECT p.id,COALESCE(p.owner_id,''),COALESCE(u.team_id,'default') FROM rex_links l JOIN properties p ON p.id=l.local_id LEFT JOIN users u ON u.id=p.owner_id WHERE l.entity='properties' AND l.account_id=? AND l.rex_id=?`, account, propertyRex).Scan(&propertyID, &owner, &team); e != nil {
				return e
			}
		} else if err != nil {
			return err
		}
		id := "rex-listings-" + account + "-" + rid
		var old string
		err = db.QueryRow(`SELECT data FROM listings WHERE id=?`, id).Scan(&old)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		data := map[string]any{}
		if old != "" {
			if e := json.Unmarshal([]byte(old), &data); e != nil {
				return e
			}
		}
		ofi := []any{}
		if existing, ok := data["ofi"].([]any); ok {
			for _, v := range existing {
				m := imap(v)
				if iv(m, "rexEventId") == "" {
					ofi = append(ofi, v)
				} else if d, e := rexDate(iv(m, "d")); e == nil && (d.Before(from) || d.After(to)) {
					ofi = append(ofi, v)
				}
			}
		}
		listingAgents, agentIDs, lead := []any{}, []any{}, map[string]any(nil)
		for _, field := range []string{"listing_agent_1", "listing_agent_2"} {
			if a := rexAgent(db, agents, r[field]); a != nil {
				if lead == nil {
					lead = a
				}
				listingAgents = append(listingAgents, a)
				if a["userId"] != "" {
					agentIDs = append(agentIDs, a["userId"])
				}
			}
		}
		for _, e := range grouped[rid] {
			if _, ok := e["agent"]; !ok {
				// No agent picked on the Rex event: the listing agent runs it.
				e["agent"], e["agentId"], e["rexAgentId"] = "", "", ""
				if lead != nil {
					e["agent"], e["agentId"], e["rexAgentId"], e["agentFrom"] = lead["name"], lead["userId"], lead["rexId"], "listing"
				}
			}
			ofi = append(ofi, e)
		}
		sort.Slice(ofi, func(i, j int) bool {
			a, b := imap(ofi[i]), imap(ofi[j])
			return iv(a, "d")+iv(a, "time") < iv(b, "d")+iv(b, "time")
		})
		data["id"], data["propertyId"], data["ofi"] = id, propertyID, ofi
		data["status"], data["rexState"] = rexListingStatus(r)
		data["rexId"], data["category"] = rid, iv(imap(r["listing_category"]), "id")
		if _, ok := data["price"]; !ok {
			data["price"] = ""
		}
		// Rex owns the listing agents; the phone shows these names as they are in Rex.
		data["agent"], data["agents"], data["agentIds"] = "", listingAgents, agentIDs
		if lead != nil {
			data["agent"] = lead["name"]
		}
		raw := ijson(data)
		var oldOwner, oldProperty, oldTeam string
		if old != "" {
			err = db.QueryRow(`SELECT COALESCE(owner_id,''),COALESCE(property_id,''),team_id FROM listings WHERE id=?`, id).Scan(&oldOwner, &oldProperty, &oldTeam)
			if err != nil {
				return err
			}
		}
		if old == raw && oldOwner == owner && oldProperty == propertyID && oldTeam == team {
			continue
		}
		tx, e := db.Begin()
		if e != nil {
			return e
		}
		stamp := now()
		if old == "" {
			_, e = tx.Exec(`INSERT INTO listings(id,team_id,owner_id,property_id,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?)`, id, team, owner, propertyID, raw, stamp, stamp)
		} else {
			_, e = tx.Exec(`UPDATE listings SET team_id=?,owner_id=?,property_id=?,data=?,updated_at=?,version=version+1,deleted_at=NULL WHERE id=?`, team, owner, propertyID, raw, stamp, id)
		}
		if e == nil {
			_, e = tx.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id,etag) VALUES('listings',?,?,'Listings',?,?,?) ON CONFLICT(entity,local_id) DO UPDATE SET etag=excluded.etag`, id, rid, stamp, account, iv(r, "etag"))
		}
		if e == nil {
			_, e = tx.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,'rex_to_app','listings',?,?,?,'ok')`, stamp, id, rid, ijson(map[string]any{"ofi": len(grouped[rid])}))
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	_, err = db.Exec(`INSERT INTO rex_sync_events(at,direction,entity,local_id,rex_id,changes,result) VALUES(?,'rex_to_app','ofi_poll','',?,'{}','ok')`, at.UTC().Format(layout), account)
	return err
}
