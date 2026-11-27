package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Registered owner names (UAT 3 item 6, docs/market-data.md "登记业主姓名").
// The client allows the app to show RP Data / ownership-import owner names to
// every agent in the team, but not to use them for direct marketing. So:
//   - property reads and sync never carry them (marketPropertyFrom strips
//     rpdataAttributes.owners/vendors and sends only market.hasOwners);
//   - the phone reads them per property here, on demand;
//   - every read that returns a name is logged in owner_name_views (one row per
//     user and property per ownerViewWindow);
//   - nothing here creates contacts, writes Rex, or feeds exports/mailing lists.

const ownerNamesNotice = "RP Data · not for direct marketing"

// ownerViewWindow: repeat views of the same property by the same user inside
// this window are not logged again (opening the page twice is one view).
const ownerViewWindow = 10 * time.Minute

// ownerNamesRoute matches GET /api/v1/properties/{id}/owner-names.
func ownerNamesRoute(r *http.Request) (string, bool) {
	if r.Method != "GET" {
		return "", false
	}
	rest, ok := strings.CutPrefix(r.URL.Path, "/api/v1/properties/")
	if !ok {
		return "", false
	}
	id, ok := strings.CutSuffix(rest, "/owner-names")
	if !ok || id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func (a *app) ownerNames(w http.ResponseWriter, p principal, id string) {
	db := a.rd()
	// Same team and not deleted; private properties of another agent stay invisible (scope).
	s, args := scope("properties", p)
	var team, raw string
	err := db.QueryRow(`SELECT team_id,data FROM properties WHERE id=? AND deleted_at IS NULL AND team_id=(SELECT team_id FROM users WHERE id=?)`+s, append([]any{id, p.id}, args...)...).Scan(&team, &raw)
	if err != nil {
		problem(w, 404, "not found")
		return
	}
	var names, source, stamp string
	var absentee bool
	_ = db.QueryRow(`SELECT owner_names,source,updated_at,absentee_owner FROM property_ownership WHERE property_id=?`, id).Scan(&names, &source, &stamp, &absentee)
	names = strings.TrimSpace(names)
	if names == "" {
		absentee = false
		var data map[string]any
		_ = json.Unmarshal([]byte(raw), &data)
		if attrs, ok := data["rpdataAttributes"].(map[string]any); ok {
			if v := strings.TrimSpace(str(attrs, "owners")); v != "" {
				names, source, stamp = v, "rpdata", str(attrs, "updatedAt")
			}
		}
	}
	if names == "" {
		source, stamp = "", ""
	} else {
		// One statement on the writer: the check and the insert cannot interleave.
		at := time.Now().UTC()
		_, err = a.db.Exec(`INSERT INTO owner_name_views(id,team_id,user_id,property_id,source,at) SELECT ?,?,?,?,?,? WHERE NOT EXISTS (SELECT 1 FROM owner_name_views WHERE user_id=? AND property_id=? AND at>?)`,
			ulid(), team, p.id, id, source, at.Format(layout), p.id, id, at.Add(-ownerViewWindow).Format(layout))
		if err != nil {
			// The view must be logged before the name is shown.
			problem(w, 500, "database error")
			return
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, 200, map[string]any{"owners": names, "absenteeOwner": absentee, "source": source, "updatedAt": stamp, "notice": ownerNamesNotice})
}

// ownerNameViews is GET /api/v1/admin/owner-name-views (admin only), newest first.
func (a *app) ownerNameViews(w http.ResponseWriter, r *http.Request, p principal) {
	if p.role != "admin" {
		problem(w, 403, "admin only")
		return
	}
	if r.Method != "GET" {
		problem(w, 405, "method not allowed")
		return
	}
	q := r.URL.Query()
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query := `SELECT v.at,v.user_id,COALESCE(u.name,''),v.property_id,COALESCE(pr.no,''),COALESCE(pr.street,''),COALESCE(pr.suburb,'') FROM owner_name_views v LEFT JOIN users u ON u.id=v.user_id LEFT JOIN properties pr ON pr.id=v.property_id WHERE 1=1`
	args := []any{}
	if v := strings.TrimSpace(q.Get("propertyId")); v != "" {
		query += ` AND v.property_id=?`
		args = append(args, v)
	}
	if v := strings.TrimSpace(q.Get("userId")); v != "" {
		query += ` AND v.user_id=?`
		args = append(args, v)
	}
	query += ` ORDER BY v.at DESC,v.id DESC LIMIT ?`
	rows, err := a.rd().Query(query, append(args, limit)...)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	out := []map[string]string{}
	for rows.Next() {
		var at, user, name, property, no, street, suburb string
		if err := rows.Scan(&at, &user, &name, &property, &no, &street, &suburb); err != nil {
			problem(w, 500, "database error")
			return
		}
		address := strings.Trim(strings.TrimSpace(no+" "+street)+", "+suburb, ", ")
		out = append(out, map[string]string{"at": at, "userId": user, "userName": name, "propertyId": property, "address": address})
	}
	if rows.Err() != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, out)
}

// keepRPNames restores the stored rpdataAttributes.owners/vendors into a
// property about to be written. Reads strip them (marketPropertyFrom), so the
// merged record of a PATCH, or a phone push of the whole property, would
// otherwise erase them from properties.data.
func keepRPNames(db querier, id string, m map[string]any) {
	var raw string
	if db.QueryRow(`SELECT data FROM properties WHERE id=?`, id).Scan(&raw) != nil {
		return
	}
	var stored map[string]any
	if json.Unmarshal([]byte(raw), &stored) != nil {
		return
	}
	old, _ := stored["rpdataAttributes"].(map[string]any)
	attrs, ok := m["rpdataAttributes"].(map[string]any)
	if !ok {
		return
	}
	for _, k := range []string{"owners", "vendors"} {
		if v, ok := old[k]; ok {
			attrs[k] = v
		} else {
			delete(attrs, k)
		}
	}
}
