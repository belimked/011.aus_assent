package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Admin work queues use the same mutation path as /resources, including version,
// audit, Rex and sync triggers. This handler is reachable only behind admin().
func (a *app) adminWork(w http.ResponseWriter, r *http.Request, p principal, path string) {
	parts := strings.Split(path, "/")
	kind := parts[0]
	if p.role == "leader" && kind == "market-review" {
		problem(w, 403, "admin only")
		return
	}
	if kind == "market-review" {
		a.marketReview(w, r, p, parts)
		return
	}
	if p.role == "leader" && kind == "contacts" && r.Method != "GET" {
		problem(w, 403, "admin only")
		return
	}
	if kind != "todos" && kind != "captures" && kind != "ofi_attendees" && kind != "listings" && kind != "contacts" {
		problem(w, 404, "not found")
		return
	}
	if r.Method == "GET" && len(parts) == 1 {
		q := `SELECT id FROM ` + kind + ` WHERE deleted_at IS NULL`
		args := []any{}
		if p.role == "leader" {
			if kind == "todos" {
				q += ` AND assignee_id IN (SELECT id FROM users WHERE team_id=(SELECT team_id FROM users WHERE id=?))`
			} else {
				q += ` AND owner_id IN (SELECT id FROM users WHERE team_id=(SELECT team_id FROM users WHERE id=?))`
			}
			args = append(args, p.id)
		}
		if kind == "captures" {
			q += ` AND COALESCE(json_extract(data,'$.status'),'pending') NOT IN ('approved','rejected')`
		}
		if kind == "contacts" && strings.TrimSpace(r.URL.Query().Get("q")) != "" {
			q += ` AND (first LIKE ? OR last LIKE ? OR mobile LIKE ?)`
			term := "%" + strings.TrimSpace(r.URL.Query().Get("q")) + "%"
			args = append(args, term, term, term)
		}
		if kind == "todos" {
			if v := r.URL.Query().Get("assignee"); v != "" {
				q += ` AND assignee_id=?`
				args = append(args, v)
			}
			if v := r.URL.Query().Get("from"); v != "" {
				q += ` AND due>=?`
				args = append(args, v)
			}
			if v := r.URL.Query().Get("to"); v != "" {
				q += ` AND due<=?`
				args = append(args, v)
			}
			if v := r.URL.Query().Get("done"); v != "" {
				if v != "true" && v != "false" {
					problem(w, 400, "invalid done")
					return
				}
				q += ` AND COALESCE(json_extract(data,'$.done'),0)=?`
				args = append(args, v == "true")
			}
		}
		q += ` ORDER BY updated_at DESC LIMIT 500`
		rows, err := a.db.Query(q, args...)
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		ids := []string{}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		out := []any{}
		for _, id := range ids {
			viewer := p
			if p.role == "leader" {
				viewer.role = "admin"
			}
			item, e := a.get(kind, id, viewer, false)
			if e != nil {
				continue
			}
			if kind == "todos" {
				var no, street, suburb string
				_ = a.db.QueryRow(`SELECT no,street,suburb FROM properties WHERE id=? AND deleted_at IS NULL`, str(item, "propId")).Scan(&no, &street, &suburb)
				item["propertyAddress"] = strings.Trim(strings.TrimSpace(no+" "+street)+", "+suburb, ", ")
			}
			if kind == "ofi_attendees" {
				var first, last, mobile string
				_ = a.db.QueryRow(`SELECT first,last,mobile FROM contacts WHERE id=? AND deleted_at IS NULL`, str(item, "contactId")).Scan(&first, &last, &mobile)
				item["contactFirst"] = first
				item["contactLast"] = last
				item["contactMobile"] = mobile
			}
			if kind == "listings" {
				var no, street, suburb, raw string
				_ = a.db.QueryRow(`SELECT no,street,suburb,data FROM properties WHERE id=?`, str(item, "propertyId")).Scan(&no, &street, &suburb, &raw)
				item["propertyAddress"] = strings.Trim(strings.TrimSpace(no+" "+street)+", "+suburb, ", ")
				item["propertySuburb"] = suburb
				var property map[string]any
				if json.Unmarshal([]byte(raw), &property) == nil {
					item["propertyPhotos"] = property["photos"]
					item["propertyBeds"] = property["beds"]
					item["propertyBaths"] = property["baths"]
					item["propertyCars"] = property["cars"]
				}
			}
			if p.role == "leader" {
				stripLicensed(item)
			}
			if kind == "captures" {
				var agent string
				_ = a.db.QueryRow(`SELECT name FROM users WHERE id=?`, item["ownerId"]).Scan(&agent)
				item["agentName"] = agent
			}
			out = append(out, item)
		}
		respond(w, 200, out)
		return
	}
	if kind == "todos" && ((r.Method == "POST" && len(parts) == 1) || (r.Method == "PATCH" && len(parts) == 2)) || kind == "ofi_attendees" && r.Method == "PATCH" && len(parts) == 2 {
		m, err := body(r)
		if err != nil {
			problem(w, 400, "invalid JSON")
			return
		}
		if kind == "todos" && r.Method == "POST" {
			if p.role == "leader" && !a.teammate(p, str(m, "assigneeId")) {
				problem(w, 403, "outside team")
				return
			}
			var n int
			_ = a.db.QueryRow(`SELECT count(*) FROM users WHERE id=? AND role IN ('agent','leader') AND disabled=0 AND deleted_at IS NULL`, str(m, "assigneeId")).Scan(&n)
			if n != 1 || str(m, "t") == "" || str(m, "d") == "" {
				problem(w, 400, "assignee, content and date required")
				return
			}
			m["createdBy"] = p.id
			m["by"] = p.name
			var name string
			_ = a.db.QueryRow(`SELECT name FROM users WHERE id=?`, str(m, "assigneeId")).Scan(&name)
			m["who"] = name
		}
		if kind == "ofi_attendees" {
			if _, e := a.get(kind, parts[1], p, false); e != nil {
				problem(w, 404, "not found")
				return
			}
		}
		id := ""
		if p.role == "leader" {
			if r.Method == "PATCH" {
				allowed := map[string]bool{"version": true, "d": true, "time": true, "assigneeId": true, "t": true, "kind": true}
				for key := range m {
					if !allowed[key] {
						problem(w, 403, "forbidden task field")
						return
					}
				}
			}
			if kind != "todos" {
				problem(w, 403, "admin only")
				return
			}
			if r.Method == "POST" {
				allowed := map[string]bool{"id": true, "t": true, "d": true, "time": true, "kind": true, "assigneeId": true, "propId": true, "cid": true, "createdBy": true, "by": true, "who": true}
				for key := range m {
					if !allowed[key] {
						problem(w, 403, "forbidden task field")
						return
					}
				}
				var ownTeam string
				_ = a.db.QueryRow(`SELECT team_id FROM users WHERE id=?`, p.id).Scan(&ownTeam)
				if value := str(m, "id"); value != "" {
					var other string
					e := a.db.QueryRow(`SELECT u.team_id FROM todos t JOIN users u ON u.id=t.assignee_id WHERE t.id=?`, value).Scan(&other)
					if e != nil && e != sql.ErrNoRows {
						problem(w, 500, "database error")
						return
					}
					if e == nil && other != ownTeam {
						problem(w, 403, "outside team")
						return
					}
				}
				if value := str(m, "propId"); value != "" {
					var n int
					_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE id=? AND team_id=? AND deleted_at IS NULL`, value, ownTeam).Scan(&n)
					if n != 1 {
						problem(w, 403, "property outside team")
						return
					}
				}
				if value := str(m, "cid"); value != "" {
					var n int
					_ = a.db.QueryRow(`SELECT count(*) FROM contacts c JOIN users u ON u.id=c.owner_id WHERE c.id=? AND u.team_id=? AND c.deleted_at IS NULL`, value, ownTeam).Scan(&n)
					if n != 1 {
						problem(w, 403, "contact outside team")
						return
					}
				}
			}
			assignee := str(m, "assigneeId")
			if r.Method == "PATCH" {
				var existingTeam, existingAssignee string
				err = a.db.QueryRow(`SELECT u.team_id,t.assignee_id FROM todos t JOIN users u ON u.id=t.assignee_id WHERE t.id=? AND t.deleted_at IS NULL`, parts[1]).Scan(&existingTeam, &existingAssignee)
				var ownTeam string
				_ = a.db.QueryRow(`SELECT team_id FROM users WHERE id=?`, p.id).Scan(&ownTeam)
				if err != nil || existingTeam != ownTeam {
					problem(w, 403, "outside team")
					return
				}
				if assignee == "" {
					assignee = existingAssignee
				}
			}
			if !a.teammate(p, assignee) {
				problem(w, 403, "outside team")
				return
			}
			// Use the generic mutation path with admin visibility only after both
			// the existing task and target assignee have passed team checks.
			p.role = "admin"
		}
		if len(parts) == 2 {
			id = parts[1]
		}
		a.mutate(w, r, p, kind, id, m)
		return
	}
	problem(w, 405, "method not allowed")
}

var lotRange = regexp.MustCompile(`(?i)^L?(\d+)-L?(\d+)\s+([A-Z]+\d+)$`)

func stripLicensed(m map[string]any) {
	forbidden := map[string]bool{"pfOwner": true, "ownerNames": true, "OwnerNames": true, "lastSale": true, "lastSalePrice": true, "lastSaleDate": true, "yearsHeld": true, "absenteeOwner": true, "market": true, "ownership": true, "sellerName": true}
	for key, value := range m {
		if forbidden[key] {
			delete(m, key)
			continue
		}
		switch child := value.(type) {
		case map[string]any:
			stripLicensed(child)
		case []any:
			for _, v := range child {
				if nested, ok := v.(map[string]any); ok {
					stripLicensed(nested)
				}
			}
		}
	}
}

func reviewLotMatch(event, parcel string) bool {
	if lotKey(event) == lotKey(parcel) {
		return true
	}
	parts := lotRange.FindStringSubmatch(strings.TrimSpace(event))
	if len(parts) != 4 {
		return false
	}
	start, _ := strconv.Atoi(parts[1])
	end, _ := strconv.Atoi(parts[2])
	if end < start {
		return false
	}
	for n := start; n <= end && n-start < 1000; n++ {
		if lotKey(parcel) == lotKey(strconv.Itoa(n)+parts[3]) {
			return true
		}
	}
	return false
}

func (a *app) marketReview(w http.ResponseWriter, r *http.Request, p principal, parts []string) {
	if r.Method == "GET" && len(parts) == 1 {
		rows, err := a.db.Query(`SELECT id,provider,kind,address,COALESCE(lot_plan,''),
			COALESCE(suburb,''),price,COALESCE(event_date,'')
			FROM market_events WHERE property_id IS NULL AND review_status='pending'
			ORDER BY event_date DESC LIMIT 200`)
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		out := []any{}
		for rows.Next() {
			var id, provider, kind, address, lot, sub, date string
			var price sql.NullInt64
			if rows.Scan(&id, &provider, &kind, &address, &lot, &sub, &price, &date) == nil {
				out = append(out, map[string]any{
					"id": id, "provider": provider, "kind": kind, "address": address,
					"lotPlan": lot, "suburb": sub, "price": price.Int64, "date": date,
				})
			}
		}
		rows.Close()
		respond(w, 200, out)
		return
	}
	if r.Method == "GET" && len(parts) == 3 && parts[2] == "candidates" {
		var address, lot, sub string
		err := a.db.QueryRow(`SELECT address,COALESCE(lot_plan,''),COALESCE(suburb,'')
			FROM market_events WHERE id=? AND property_id IS NULL AND review_status='pending'`,
			parts[1],
		).Scan(&address, &lot, &sub)
		if err != nil {
			problem(w, 404, "not found")
			return
		}
		plan := lot
		if fields := strings.Fields(lot); len(fields) > 1 {
			plan = "%" + fields[len(fields)-1]
		}
		rows, err := a.db.Query(`SELECT p.id,p.no,p.street,p.suburb,p.registered,p.data,
			COALESCE(o.owner_names,'') FROM properties p
			LEFT JOIN property_ownership o ON o.property_id=p.id
			WHERE p.deleted_at IS NULL AND (upper(p.suburb)=upper(?)
			OR upper(json_extract(p.data,'$.lotplan')) LIKE upper(?)) LIMIT 10000`,
			sub, plan,
		)
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		type candidate struct {
			ID, Address, Suburb, LotPlan, OwnerNames string
			Basis                                    string
			LL                                       any
			Registered                               bool
			Score                                    int
		}
		out := []candidate{}
		for rows.Next() {
			var id, no, street, suburb, raw, owners string
			var registered int
			if rows.Scan(&id, &no, &street, &suburb, &registered, &raw, &owners) != nil {
				continue
			}
			var data map[string]any
			_ = json.Unmarshal([]byte(raw), &data)
			lp := str(data, "lotplan")
			if lp == "" {
				lp = str(data, "lot")
			}
			score := 0
			basis := ""
			switch {
			case lot != "" && lotKey(lot) == lotKey(lp):
				score, basis = 100, "lot-exact"
			case lot != "" && reviewLotMatch(lot, lp):
				score, basis = 95, "lot-range"
			case strings.EqualFold(sub, suburb) && addressKey(address) == addressKey(no+" "+street):
				score, basis = 80, "address-exact"
			case strings.EqualFold(sub, suburb) && strings.Contains(addressKey(address), addressKey(street)):
				score, basis = 30, "street"
			}
			if score > 0 {
				out = append(out, candidate{
					ID: id, Address: strings.TrimSpace(no + " " + street), Suburb: suburb,
					LotPlan: lp, OwnerNames: owners, Basis: basis, LL: data["ll"],
					Registered: registered != 0, Score: score,
				})
			}
		}
		rows.Close()
		sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
		if len(out) > 20 {
			out = out[:20]
		}
		respond(w, 200, out)
		return
	}
	if r.Method != "POST" || len(parts) != 3 || (parts[2] != "link" && parts[2] != "ignore") {
		problem(w, 405, "method not allowed")
		return
	}
	m, err := body(r)
	if err != nil {
		problem(w, 400, "invalid JSON")
		return
	}
	tx, err := a.db.Begin()
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	var provider, kind, address, date, importID string
	var price sql.NullInt64
	err = tx.QueryRow(`SELECT provider,kind,address,COALESCE(event_date,''),import_id,price
		FROM market_events WHERE id=? AND property_id IS NULL AND review_status='pending'`,
		parts[1],
	).Scan(&provider, &kind, &address, &date, &importID, &price)
	if err != nil {
		problem(w, 409, "event already reviewed or missing")
		return
	}
	property := ""
	status := "ignored"
	if parts[2] == "link" {
		property = str(m, "propertyId")
		var exists int
		_ = tx.QueryRow(`SELECT count(*) FROM properties WHERE id=? AND deleted_at IS NULL`, property).Scan(&exists)
		if exists != 1 {
			problem(w, 404, "property not found")
			return
		}
		status = "linked"
		if kind == "sale" {
			// Imported raw events intentionally omit owner names and owner addresses.
			// Preserve existing ownership names while applying the dated sale and registration.
			if err = applyMarketOwnership(tx, property, "", price.Int64, date, provider, importID, false); err != nil {
				problem(w, 500, "database error")
				return
			}
		}
		if kind == "sale" || kind == "listing" {
			if err = a.notifyMarket(tx, parts[1], property, kind, address, "", date, map[string]int{}); err != nil {
				problem(w, 500, "database error")
				return
			}
		}
	}
	res, err := tx.Exec(`UPDATE market_events SET property_id=NULLIF(?,''),review_status=? WHERE id=? AND property_id IS NULL AND review_status='pending'`, property, status, parts[1])
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		problem(w, 409, "event already reviewed")
		return
	}
	if err = a.record(tx, p, strings.ToUpper(parts[2]), "market_events", parts[1], map[string]any{"propertyId": property}); err != nil {
		problem(w, 500, "database error")
		return
	}
	if err = tx.Commit(); err != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, map[string]any{"id": parts[1], "status": status})
}
