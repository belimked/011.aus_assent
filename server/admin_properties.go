package main

import (
	"database/sql"
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Admin-only parcel queries deliberately have bounded responses; ownership never reaches sync or exports here.
func (a *app) adminProperties(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if r.Method != "GET" {
		problem(w, 405, "method not allowed")
		return
	}
	if q.Get("bbox") != "" {
		a.adminParcelMap(w, r)
		return
	}
	where := ` WHERE p.deleted_at IS NULL`
	args := []any{}
	if v := strings.TrimSpace(q.Get("q")); v != "" {
		where += ` AND (p.no LIKE ? OR p.street LIKE ? OR p.suburb LIKE ? OR json_extract(p.data,'$.lotplan') LIKE ? OR p.id LIKE ?)`
		for i := 0; i < 5; i++ {
			args = append(args, "%"+v+"%")
		}
	}
	if v := q.Get("suburb"); v != "" {
		where += ` AND p.suburb=?`
		args = append(args, v)
	}
	if v := q.Get("registered"); v != "" {
		if v != "true" && v != "false" {
			problem(w, 400, "invalid registered")
			return
		}
		where += ` AND p.registered=?`
		if v == "true" {
			args = append(args, 1)
		} else {
			args = append(args, 0)
		}
	}
	if v := q.Get("hasOwner"); v != "" {
		if v != "true" && v != "false" {
			problem(w, 400, "invalid hasOwner")
			return
		}
		if v == "true" {
			where += ` AND o.property_id IS NOT NULL`
		} else {
			where += ` AND o.property_id IS NULL`
		}
	}
	if v := q.Get("minYearsHeld"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil || n < 0 || n > 200 {
			problem(w, 400, "invalid minYearsHeld")
			return
		}
		where += ` AND o.last_sale_date!='' AND o.last_sale_date<=?`
		args = append(args, time.Now().UTC().AddDate(-n, 0, 0).Format("2006-01-02"))
	}
	if v := q.Get("cursor"); v != "" {
		where += ` AND p.id>?`
		args = append(args, v)
	}
	rows, err := a.db.Query(`SELECT p.id,p.no,p.street,p.suburb,p.registered,p.data,COALESCE(o.owner_names,''),o.last_sale_price,COALESCE(o.last_sale_date,''),COALESCE(o.absentee_owner,0) FROM properties p LEFT JOIN property_ownership o ON o.property_id=p.id`+where+` ORDER BY p.id LIMIT ?`, append(args, pageLimit(r))...)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var id, no, street, suburb, raw, names, date string
		var registered, absentee int
		var price sql.NullInt64
		if rows.Scan(&id, &no, &street, &suburb, &registered, &raw, &names, &price, &date, &absentee) != nil {
			continue
		}
		item := map[string]any{"id": id, "address": strings.TrimSpace(no + " " + street), "suburb": suburb, "registered": registered != 0, "ownerNames": names, "lastSalePrice": nil, "lastSaleDate": date, "absenteeOwner": absentee != 0, "yearsHeld": nil, "coveringAgents": []any{}}
		if price.Valid {
			item["lastSalePrice"] = price.Int64
		}
		if d, e := time.Parse("2006-01-02", date); e == nil {
			item["yearsHeld"] = math.Floor(time.Since(d).Hours()/24/365.2425*10) / 10
		}
		var data map[string]any
		_ = json.Unmarshal([]byte(raw), &data)
		item["lotplan"] = str(data, "lotplan")
		item["geometry"] = data["geometry"]
		item["ll"] = data["ll"]
		out = append(out, item)
	}
	rows.Close()
	// Resolve coverage after closing the cursor (SQLite's single-connection test fixtures).
	for _, v := range out {
		item := v.(map[string]any)
		agents := []any{}
		users, e := a.db.Query(`SELECT id,name FROM users WHERE role='agent' AND disabled=0 AND deleted_at IS NULL`)
		if e != nil {
			continue
		}
		ids := [][2]string{}
		for users.Next() {
			var id, name string
			if users.Scan(&id, &name) == nil {
				ids = append(ids, [2]string{id, name})
			}
		}
		users.Close()
		for _, u := range ids {
			if a.covered(item["id"].(string), u[0]) {
				agents = append(agents, map[string]string{"id": u[0], "name": u[1]})
			}
		}
		item["coveringAgents"] = agents
	}
	respond(w, 200, out)
}
func (a *app) adminParcelMap(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(r.URL.Query().Get("bbox"), ",")
	if len(parts) != 4 {
		problem(w, 400, "invalid bbox")
		return
	}
	b := make([]float64, 4)
	for i, v := range parts {
		n, e := strconv.ParseFloat(v, 64)
		if e != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			problem(w, 400, "invalid bbox")
			return
		}
		b[i] = n
	}
	if b[0] >= b[2] || b[1] >= b[3] || b[2]-b[0] > 1 || b[3]-b[1] > 1 {
		problem(w, 400, "invalid bbox")
		return
	}
	limit := pageLimit(r)
	if limit > 100 {
		limit = 100
	}
	rows, e := a.db.Query(`SELECT p.id,p.data,p.registered,o.property_id IS NOT NULL FROM properties p LEFT JOIN property_ownership o ON o.property_id=p.id WHERE p.deleted_at IS NULL AND CAST(json_extract(p.data,'$.lng') AS REAL) BETWEEN ? AND ? AND CAST(json_extract(p.data,'$.lat') AS REAL) BETWEEN ? AND ? ORDER BY p.id LIMIT ?`, b[0], b[2], b[1], b[3], limit)
	if e != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	features := []any{}
	for rows.Next() {
		var id, raw string
		var registered, owned int
		if rows.Scan(&id, &raw, &registered, &owned) != nil {
			continue
		}
		var data map[string]any
		_ = json.Unmarshal([]byte(raw), &data)
		coords := []any{}
		if pts, ok := data["geometry"].([]any); ok {
			for _, v := range pts {
				p, ok := v.(map[string]any)
				if ok {
					coords = append(coords, []any{p["lng"], p["lat"]})
				}
			}
		}
		geom := map[string]any{"type": "Point", "coordinates": []any{data["lng"], data["lat"]}}
		if len(coords) >= 3 {
			coords = append(coords, coords[0])
			geom = map[string]any{"type": "Polygon", "coordinates": []any{coords}}
		}
		features = append(features, map[string]any{"type": "Feature", "geometry": geom, "properties": map[string]any{"id": id, "registered": registered != 0, "hasOwner": owned != 0}})
	}
	respond(w, 200, map[string]any{"type": "FeatureCollection", "features": features})
}
func (a *app) adminParcelStats(w http.ResponseWriter) {
	rows, e := a.db.Query(`SELECT p.suburb,count(*),sum(CASE WHEN p.registered=1 THEN 1 ELSE 0 END),sum(CASE WHEN o.property_id IS NOT NULL THEN 1 ELSE 0 END) FROM properties p LEFT JOIN property_ownership o ON o.property_id=p.id WHERE p.deleted_at IS NULL GROUP BY p.suburb ORDER BY p.suburb`)
	if e != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var suburb string
		var total, registered, owned int
		if rows.Scan(&suburb, &total, &registered, &owned) == nil {
			out = append(out, map[string]any{"suburb": suburb, "total": total, "registered": registered, "withOwnership": owned})
		}
	}
	respond(w, 200, out)
}
func (a *app) adminZones(w http.ResponseWriter, r *http.Request, p principal, path string) {
	id := strings.TrimPrefix(path, "zones/")
	if path == "zones" {
		id = ""
	}
	if r.Method == "GET" && id == "" {
		rows, e := a.db.Query(`SELECT id FROM zones WHERE deleted_at IS NULL ORDER BY id`)
		if e != nil {
			problem(w, 500, "database error")
			return
		}
		ids := []string{}
		for rows.Next() {
			var v string
			if rows.Scan(&v) == nil {
				ids = append(ids, v)
			}
		}
		rows.Close()
		out := []any{}
		for _, v := range ids {
			z, e := a.get("zones", v, p, false)
			if e == nil {
				out = append(out, z)
			}
		}
		respond(w, 200, out)
		return
	}
	if (r.Method == "POST" && id != "") || ((r.Method == "PATCH" || r.Method == "DELETE") && id == "") {
		problem(w, 404, "not found")
		return
	}
	var m map[string]any
	if r.Method != "DELETE" {
		var e error
		m, e = body(r)
		if e != nil {
			problem(w, 400, "invalid JSON")
			return
		}
		owner := str(m, "ownerId")
		if r.Method == "PATCH" && owner == "" {
			owner = a.b3Owner("zones", id)
		}
		var n int
		_ = a.db.QueryRow(`SELECT count(*) FROM users WHERE id=? AND role='agent' AND disabled=0 AND deleted_at IS NULL`, owner).Scan(&n)
		if n == 0 {
			problem(w, 400, "active agent required")
			return
		}
		points, pts := m["pts"].([]any)
		suburbs, subs := m["suburbs"].([]any)
		origin, center := m["center"].([]any)
		if (pts && len(points) < 3) || (subs && len(suburbs) == 0) || (center && len(origin) != 2) {
			problem(w, 400, "invalid zone coverage")
			return
		}
		if !pts && !subs && !center && str(m, "suburb") == "" {
			problem(w, 400, "zone coverage required")
			return
		}
		if center {
			radius, ok := m["radius"].(float64)
			if !ok || radius <= 0 {
				problem(w, 400, "radius required")
				return
			}
		}
	}
	a.mutate(w, r, p, "zones", id, m)
}
