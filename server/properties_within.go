package main

import (
	"encoding/json"
	"math"
	"net/http"
)

type withinPoint [2]float64
type withinItem struct {
	ID         string      `json:"id"`
	No         string      `json:"no"`
	Street     string      `json:"street"`
	Suburb     string      `json:"suburb"`
	LL         withinPoint `json:"ll"`
	HasContact bool        `json:"hasContact"`
	Source     string      `json:"source"`
	InRex      bool        `json:"inRex"`
}

func withinPolygon(p withinPoint, poly []withinPoint) bool {
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a[1] > p[1]) != (b[1] > p[1]) && p[0] < (b[0]-a[0])*(p[1]-a[1])/(b[1]-a[1])+a[0] {
			inside = !inside
		}
	}
	return inside
}

// Map point: numeric ll first, then numeric lat/lng (legacy rows; see property_ll.go).
const withinHasLL = `json_type(p.data,'$.ll[0]') IN ('integer','real') AND json_type(p.data,'$.ll[1]') IN ('integer','real')`
const withinHasLatLng = `json_type(p.data,'$.lat') IN ('integer','real') AND json_type(p.data,'$.lng') IN ('integer','real')`
const withinLat = `(CASE WHEN ` + withinHasLL + ` THEN json_extract(p.data,'$.ll[0]') WHEN ` + withinHasLatLng + ` THEN json_extract(p.data,'$.lat') END)`
const withinLng = `(CASE WHEN ` + withinHasLL + ` THEN json_extract(p.data,'$.ll[1]') WHEN ` + withinHasLatLng + ` THEN json_extract(p.data,'$.lng') END)`

func (a *app) propertiesWithin(w http.ResponseWriter, r *http.Request, p principal) {
	var input struct {
		Polygon []withinPoint `json:"polygon"`
		Limit   int           `json:"limit"`
		Offset  int           `json:"offset"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(&input) != nil || len(input.Polygon) < 3 || len(input.Polygon) > 100 || input.Offset < 0 || input.Offset > 100000 || input.Limit < 0 || input.Limit > 200 {
		problem(w, 400, "invalid polygon or pagination")
		return
	}
	if input.Limit == 0 {
		input.Limit = 100
	}
	south, north, west, east := 90.0, -90.0, 180.0, -180.0
	for _, pt := range input.Polygon {
		if math.IsNaN(pt[0]) || math.IsInf(pt[0], 0) || math.IsNaN(pt[1]) || math.IsInf(pt[1], 0) || pt[0] < -90 || pt[0] > 90 || pt[1] < -180 || pt[1] > 180 {
			problem(w, 400, "invalid coordinates")
			return
		}
		south = math.Min(south, pt[0])
		north = math.Max(north, pt[0])
		west = math.Min(west, pt[1])
		east = math.Max(east, pt[1])
	}
	// Small neighbourhood patches only; prevents unrestricted enumeration and expensive scans.
	if south == north || west == east || (north-south)*111*(east-west)*111*math.Cos((south+north)*math.Pi/360) > 25 {
		problem(w, 400, "area too large or empty")
		return
	}
	scopeSQL, scopeArgs := scope("properties", p)
	args := []any{south, north, west, east}
	args = append(args, scopeArgs...)
	rows, err := a.rd().Query(`SELECT p.id,coalesce(p.no,''),coalesce(p.street,''),coalesce(p.suburb,''),
 `+withinLat+`,`+withinLng+`,coalesce(json_extract(p.data,'$.source'),''),
 EXISTS(SELECT 1 FROM rex_links rl WHERE rl.entity='properties' AND rl.local_id=p.id),
 EXISTS(SELECT 1 FROM property_owners po JOIN contacts c ON c.id=po.contact_id AND c.deleted_at IS NULL WHERE po.property_id=p.id AND (c.owner_id=? OR ?='admin'))
 FROM properties p WHERE p.deleted_at IS NULL AND json_valid(p.data)
 AND `+withinLat+` BETWEEN ? AND ? AND `+withinLng+` BETWEEN ? AND ?`+scopeSQL+propertySyncSQL("p")+` ORDER BY p.id`, append([]any{p.id, p.role}, args...)...)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	items := []withinItem{}
	total, linked := 0, 0
	for rows.Next() {
		var item withinItem
		if err = rows.Scan(&item.ID, &item.No, &item.Street, &item.Suburb, &item.LL[0], &item.LL[1], &item.Source, &item.InRex, &item.HasContact); err != nil {
			problem(w, 500, "database error")
			return
		}
		if !withinPolygon(item.LL, input.Polygon) {
			continue
		}
		if item.HasContact {
			linked++
		}
		if total >= input.Offset && len(items) < input.Limit {
			items = append(items, item)
		}
		total++
	}
	if rows.Err() != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, map[string]any{"items": items, "total": total, "linked": linked, "limit": input.Limit, "offset": input.Offset})
}
