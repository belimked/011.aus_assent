package main

import (
	"net/http"
	"time"
)

// Owner links have no created_at. A linked contact's created_at is the conservative
// proxy: existing contacts linked later are not counted, never inferred from an import.
func (a *app) metricExtras(r *http.Request, p principal, from, to time.Time, team, agent string) map[string]any {
	start, end := from.Format(layout), to.Format(layout)
	scope := func(owner string) (string, []any) {
		q := ""
		args := []any{}
		if team != "" {
			q += " AND " + owner + " IN (SELECT id FROM users WHERE team_id=?)"
			args = append(args, team)
		}
		if agent != "" {
			q += " AND " + owner + "=?"
			args = append(args, agent)
		}
		return q, args
	}
	count := func(query string, args ...any) int { var n int; _ = a.rd().QueryRow(query, args...).Scan(&n); return n }
	linkCount := func(lo, hi time.Time) int {
		condition, extra := scope("c.owner_id")
		args := append([]any{lo.Format(layout), hi.Format(layout)}, extra...)
		return count(`SELECT count(DISTINCT c.id) FROM contacts c JOIN property_owners po ON po.contact_id=c.id WHERE c.deleted_at IS NULL AND c.created_at>=? AND c.created_at<?`+condition, args...)
	}
	owners := linkCount(from, to)
	previous := linkCount(from.Add(-to.Sub(from)), from)
	days := int(to.Sub(from).Hours() / 24)
	ownerSpark := make([]int, days)
	cond, more := scope("c.owner_id")
	linkSQL := `SELECT substr(c.created_at,1,10),count(DISTINCT c.id) FROM contacts c JOIN property_owners po ON po.contact_id=c.id WHERE c.deleted_at IS NULL AND c.created_at>=? AND c.created_at<?` + cond + ` GROUP BY 1 LIMIT 90`
	linkArgs := append([]any{start, end}, more...)
	if rows, e := a.rd().Query(linkSQL, linkArgs...); e == nil {
		for rows.Next() {
			var day string
			var n int
			if rows.Scan(&day, &n) == nil {
				if date, e := time.Parse("2006-01-02", day); e == nil {
					i := int(date.Sub(from).Hours() / 24)
					if i >= 0 && i < days {
						ownerSpark[i] = n
					}
				}
			}
		}
		rows.Close()
	}
	covered, total := 0, 0
	coverageSQL := `SELECT count(*),count(CASE WHEN po.property_id IS NOT NULL THEN 1 END) FROM properties a LEFT JOIN (SELECT DISTINCT property_id FROM property_owners) po ON po.property_id=a.id WHERE a.deleted_at IS NULL`
	coverageArgs := []any{}
	if team != "" {
		coverageSQL += ` AND a.team_id=?`
		coverageArgs = append(coverageArgs, team)
	}
	if agent != "" {
		coverageSQL += ` AND EXISTS (SELECT 1 FROM zones z WHERE z.owner_id=? AND z.team_id=a.team_id AND z.deleted_at IS NULL AND (upper(z.name)=upper(a.suburb) OR EXISTS (SELECT 1 FROM json_each(json_extract(z.data,'$.suburbs')) WHERE upper(value)=upper(a.suburb))))`
		coverageArgs = append(coverageArgs, agent)
	}
	_ = a.rd().QueryRow(coverageSQL, coverageArgs...).Scan(&total, &covered)
	// Bound the current-period results at the SQL layer, not in the client.
	agendaSQL := `SELECT a.id,COALESCE(a.due,''),COALESCE(a.text,''),COALESCE(u.name,'') FROM todos a JOIN users u ON u.id=a.assignee_id WHERE a.deleted_at IS NULL AND COALESCE(json_extract(a.data,'$.done'),0)=0 AND a.due<=?`
	agendaArgs := []any{time.Now().Format("2006-01-02")}
	cond, more = scope("a.assignee_id")
	agendaSQL += cond
	agendaArgs = append(agendaArgs, more...)
	agendaSQL += ` ORDER BY a.due,a.id LIMIT 20`
	agenda := []any{}
	if rows, e := a.rd().Query(agendaSQL, agendaArgs...); e == nil {
		for rows.Next() {
			var id, due, text, name string
			if rows.Scan(&id, &due, &text, &name) == nil {
				agenda = append(agenda, map[string]string{"id": id, "due": due, "text": text, "agent": name})
			}
		}
		rows.Close()
	}
	performance := []any{}
	q := `SELECT id,name FROM users WHERE deleted_at IS NULL AND disabled=0 AND role IN ('agent','leader')`
	args := []any{}
	if team != "" {
		q += ` AND team_id=?`
		args = append(args, team)
	}
	if agent != "" {
		q += ` AND id=?`
		args = append(args, agent)
	}
	q += ` ORDER BY name LIMIT 200`
	if rows, e := a.rd().Query(q, args...); e == nil {
		ids := [][2]string{}
		for rows.Next() {
			var id, name string
			if rows.Scan(&id, &name) == nil {
				ids = append(ids, [2]string{id, name})
			}
		}
		rows.Close()
		for _, u := range ids {
			id, name := u[0], u[1]
			visits := count(`SELECT count(*) FROM activity WHERE owner_id=? AND deleted_at IS NULL AND json_extract(data,'$.a')='DK' AND created_at>=? AND created_at<?`, id, start, end)
			contacts := count(`SELECT count(*) FROM contacts WHERE owner_id=? AND deleted_at IS NULL AND created_at>=? AND created_at<?`, id, start, end)
			appraisals := count(`SELECT count(*) FROM todos WHERE assignee_id=? AND deleted_at IS NULL AND lower(COALESCE(kind,'')) LIKE '%appraisal%' AND created_at>=? AND created_at<?`, id, start, end)
			listings := count(`SELECT count(*) FROM listings WHERE owner_id=? AND deleted_at IS NULL AND created_at>=? AND created_at<?`, id, start, end)
			done := count(`SELECT count(*) FROM todos WHERE assignee_id=? AND deleted_at IS NULL AND COALESCE(json_extract(data,'$.done'),0)=1 AND substr(COALESCE(json_extract(data,'$.doneAt'),updated_at),1,10)>=? AND substr(COALESCE(json_extract(data,'$.doneAt'),updated_at),1,10)<?`, id, from.Format("2006-01-02"), to.Format("2006-01-02"))
			linked := count(`SELECT count(DISTINCT c.id) FROM contacts c JOIN property_owners po ON po.contact_id=c.id WHERE c.owner_id=? AND c.deleted_at IS NULL AND c.created_at>=? AND c.created_at<?`, id, start, end)
			performance = append(performance, map[string]any{"id": id, "name": name, "visits": visits, "contacts": contacts, "appraisals": appraisals, "listings": listings, "done": done, "linked": linked})
		}
	}
	heat := []any{}
	heatSQL := `SELECT CAST(strftime('%w',datetime(a.created_at,'+10 hours')) AS INTEGER),CAST(strftime('%H',datetime(a.created_at,'+10 hours')) AS INTEGER),count(*) FROM activity a WHERE a.deleted_at IS NULL AND json_extract(a.data,'$.a')='DK' AND a.created_at>=? AND a.created_at<?`
	heatArgs := []any{start, end}
	cond, more = scope("a.owner_id")
	heatSQL += cond
	heatArgs = append(heatArgs, more...)
	heatSQL += ` GROUP BY 1,2 LIMIT 168`
	visitCount := 0
	if rows, e := a.rd().Query(heatSQL, heatArgs...); e == nil {
		for rows.Next() {
			var day, hour, n int
			if rows.Scan(&day, &hour, &n) == nil {
				heat = append(heat, []int{hour, day, n})
				visitCount += n
			}
		}
		rows.Close()
	}
	opportunities := []any{}
	if p.role == "admin" {
		// No licensed owner names or sale dates are returned: only a derived score.
		query := `SELECT p.id,trim(COALESCE(p.no,'')||' '||COALESCE(p.street,'')),p.suburb,o.last_sale_date,COALESCE((SELECT max(a.created_at) FROM activity a WHERE a.deleted_at IS NULL AND a.created_at>=? AND json_extract(a.data,'$.a')='DK' AND (json_extract(a.data,'$.propId')=p.id OR json_extract(a.data,'$.propertyId')=p.id OR (length(COALESCE(p.street,''))>=4 AND instr(lower(COALESCE(json_extract(a.data,'$.t'),'')),lower(p.street))>0))),'') FROM properties p JOIN property_ownership o ON o.property_id=p.id WHERE p.deleted_at IS NULL AND o.last_sale_date<=? AND NOT EXISTS (SELECT 1 FROM property_owners po WHERE po.property_id=p.id) AND COALESCE(json_extract(p.flags,'$.dnc'),0)=0 AND COALESCE(json_extract(p.data,'$.doNotKnock'),0)=0`
		args := []any{time.Now().AddDate(0, -3, 0).UTC().Format(layout), time.Now().AddDate(-10, 0, 0).Format("2006-01-02")}
		if team != "" {
			query += ` AND p.team_id=?`
			args = append(args, team)
		}
		query += ` ORDER BY o.last_sale_date LIMIT 500`
		type opportunity struct {
			id, address, suburb string
			score               int
		}
		top := []opportunity{}
		if rows, e := a.rd().Query(query, args...); e == nil {
			for rows.Next() {
				var id, address, suburb, sale, last string
				if rows.Scan(&id, &address, &suburb, &sale, &last) != nil {
					continue
				}
				sold, e := time.Parse("2006-01-02", sale)
				if e != nil {
					continue
				}
				if last != "" {
					seen, e := time.Parse(layout, last)
					if e == nil && time.Since(seen) < 7*24*time.Hour {
						continue
					}
				}
				held := int(time.Since(sold).Hours() / 24 / 365.25)
				score := 30 + min(held-10, 40)
				if last == "" {
					score += 30
				} else if seen, e := time.Parse(layout, last); e == nil {
					score += min(int(time.Since(seen).Hours()/24/30), 30)
				}
				score = min(score, 100)
				top = append(top, opportunity{id, address, suburb, score})
			}
			rows.Close()
		}
		// Sort by score, then stable address for deterministic screenshots.
		for i := 0; i < len(top); i++ {
			for j := i + 1; j < len(top); j++ {
				if top[j].score > top[i].score {
					top[i], top[j] = top[j], top[i]
				}
			}
		}
		for _, o := range top[:min(len(top), 10)] {
			opportunities = append(opportunities, map[string]any{"id": o.id, "address": o.address, "suburb": o.suburb, "score": o.score})
		}
	}
	return map[string]any{"owners": map[string]any{"value": owners, "previous": previous, "spark": ownerSpark}, "coverage": map[string]int{"total": total, "linked": covered}, "agenda": agenda, "performance": performance, "heatmap": heat, "visitCount": visitCount, "opportunities": opportunities}
}
