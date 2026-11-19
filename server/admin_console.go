package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *app) console(w http.ResponseWriter, r *http.Request, p principal, path string) bool {
	switch path {
	case "metrics":
		if r.Method != "GET" {
			problem(w, 405, "method not allowed")
			return true
		}
		a.metrics(w, r, p)
	case "export":
		if p.role != "admin" {
			problem(w, 403, "admin only")
			return true
		}
		if r.Method != "POST" {
			problem(w, 405, "method not allowed")
			return true
		}
		a.exportCSV(w, r, p)
	case "exports":
		if p.role != "admin" {
			problem(w, 403, "admin only")
			return true
		}
		rows, e := a.db.Query(`SELECT at,diff FROM audit_log WHERE action='export' ORDER BY id DESC LIMIT 50`)
		if e != nil {
			problem(w, 500, "database error")
			return true
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var at, diff string
			if rows.Scan(&at, &diff) == nil {
				out = append(out, map[string]string{"at": at, "diff": diff})
			}
		}
		respond(w, 200, out)
	case "settings":
		if p.role != "admin" {
			problem(w, 403, "admin only")
			return true
		}
		a.consoleSettings(w, r, p)
	case "teams":
		a.consoleTeams(w, r, p)
	default:
		return false
	}
	return true
}

func (a *app) consoleTeams(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method == "GET" {
		q := `SELECT id,name FROM teams`
		args := []any{}
		if p.role == "leader" {
			q += ` WHERE id=(SELECT team_id FROM users WHERE id=?)`
			args = append(args, p.id)
		}
		rows, e := a.db.Query(q, args...)
		if e != nil {
			problem(w, 500, "database error")
			return
		}
		defer rows.Close()
		out := []any{}
		for rows.Next() {
			var id, name string
			if rows.Scan(&id, &name) == nil {
				out = append(out, map[string]string{"id": id, "name": name})
			}
		}
		respond(w, 200, out)
		return
	}
	if p.role != "admin" {
		problem(w, 403, "admin only")
		return
	}
	if r.Method != "POST" && r.Method != "PATCH" {
		problem(w, 405, "method not allowed")
		return
	}
	m, e := body(r)
	if e != nil || strings.TrimSpace(str(m, "name")) == "" {
		problem(w, 400, "name required")
		return
	}
	id := str(m, "id")
	if r.Method == "POST" {
		id = ulid()
	}
	tx, e := a.db.Begin()
	if e != nil {
		problem(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	if r.Method == "POST" {
		_, e = tx.Exec(`INSERT INTO teams(id,name) VALUES(?,?)`, id, str(m, "name"))
	} else {
		var res sql.Result
		res, e = tx.Exec(`UPDATE teams SET name=? WHERE id=?`, str(m, "name"), id)
		if e == nil {
			n, _ := res.RowsAffected()
			if n == 0 {
				problem(w, 404, "not found")
				return
			}
		}
	}
	if e == nil {
		e = a.record(tx, p, r.Method, "teams", id, m)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		problem(w, 400, "invalid team")
		return
	}
	respond(w, 200, map[string]any{"id": id, "name": str(m, "name")})
}

func (a *app) consoleSettings(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method == "GET" {
		var hours string
		_ = a.db.QueryRow(`SELECT value FROM settings WHERE key='session_hours'`).Scan(&hours)
		var provider string
		var imported sql.NullString
		var count sql.NullInt64
		_ = a.db.QueryRow(`SELECT provider,created_at,success FROM market_imports WHERE status='completed' ORDER BY created_at DESC LIMIT 1`).Scan(&provider, &imported, &count)
		var pending, failed int
		_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE status IN ('pending','waiting_auth')`).Scan(&pending)
		_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE status='failed'`).Scan(&failed)
		var lastError string
		_ = a.db.QueryRow(`SELECT COALESCE(last_error,'') FROM rex_outbox WHERE last_error IS NOT NULL ORDER BY id DESC LIMIT 1`).Scan(&lastError)
		respond(w, 200, map[string]any{"sessionHours": hours, "provider": "Pricefinder", "lastImport": imported.String, "importRows": count.Int64, "rexConfigured": env("AXLEO_REX_ENABLED", "") == "1" && env("REX_EMAIL", "") != "" && env("REX_PASSWORD", "") != "" && env("REX_ACCOUNT_ID", "") != "", "rexPending": pending, "rexFailed": failed, "rexError": lastError})
		return
	}
	if r.Method != "PATCH" {
		problem(w, 405, "method not allowed")
		return
	}
	m, e := body(r)
	hours, e2 := strconv.Atoi(str(m, "sessionHours"))
	if e != nil || e2 != nil || hours < 1 || hours > 720 {
		problem(w, 400, "session hours must be 1–720")
		return
	}
	tx, e := a.db.Begin()
	if e != nil {
		problem(w, 500, "database error")
		return
	}
	defer tx.Rollback()
	_, e = tx.Exec(`UPDATE settings SET value=? WHERE key='session_hours'`, strconv.Itoa(hours))
	if e == nil {
		e = a.record(tx, p, "PATCH", "settings", "session_hours", m)
	}
	if e == nil {
		e = tx.Commit()
	}
	if e != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, map[string]any{"sessionHours": hours})
}

func (a *app) metrics(w http.ResponseWriter, r *http.Request, p principal) {
	days := map[string]int{"today": 1, "7": 7, "30": 30, "90": 90}[r.URL.Query().Get("period")]
	if days == 0 {
		problem(w, 400, "invalid period")
		return
	}
	team, agent := r.URL.Query().Get("team"), r.URL.Query().Get("agent")
	if p.role == "leader" {
		_ = a.db.QueryRow(`SELECT team_id FROM users WHERE id=?`, p.id).Scan(&team)
	}
	if agent != "" {
		var n int
		q := `SELECT count(*) FROM users WHERE id=? AND deleted_at IS NULL`
		args := []any{agent}
		if team != "" {
			q += ` AND team_id=?`
			args = append(args, team)
		}
		_ = a.db.QueryRow(q, args...).Scan(&n)
		if n == 0 {
			problem(w, 403, "agent outside team")
			return
		}
	}
	start := time.Now().UTC().Truncate(24*time.Hour).AddDate(0, 0, 1-days)
	type point struct {
		Day    string `json:"day"`
		Visits int    `json:"visits"`
	}
	trend := []point{}
	kpis := map[string]any{}
	for _, metric := range []struct{ key, query string }{
		{"visits", `SELECT count(*) FROM activity a JOIN users u ON u.id=a.owner_id WHERE a.deleted_at IS NULL AND json_extract(a.data,'$.a')='DK' AND a.created_at>=? AND a.created_at<?`},
		{"captures", `SELECT count(*) FROM contacts a JOIN users u ON u.id=a.owner_id WHERE a.deleted_at IS NULL AND a.created_at>=? AND a.created_at<?`},
		{"appraisals", `SELECT count(*) FROM todos a JOIN users u ON u.id=a.assignee_id WHERE a.deleted_at IS NULL AND lower(COALESCE(a.kind,'')) LIKE '%appraisal%' AND a.created_at>=? AND a.created_at<?`},
		{"ofi", `SELECT count(*) FROM ofi_attendees a JOIN users u ON u.id=a.owner_id WHERE a.deleted_at IS NULL AND a.created_at>=? AND a.created_at<?`},
	} {
		count := func(from, to time.Time) int {
			q := metric.query
			args := []any{from.Format(layout), to.Format(layout)}
			if team != "" {
				q += ` AND a.team_id=?`
				args = append(args, team)
			}
			if agent != "" {
				q += ` AND u.id=?`
				args = append(args, agent)
			}
			var n int
			_ = a.db.QueryRow(q, args...).Scan(&n)
			return n
		}
		end := start.AddDate(0, 0, days)
		kpis[metric.key] = map[string]int{"value": count(start, end), "previous": count(start.AddDate(0, 0, -days), start)}
	}
	for d := 0; d < days; d++ {
		from := start.AddDate(0, 0, d)
		q := `SELECT count(*) FROM activity a JOIN users u ON u.id=a.owner_id WHERE a.deleted_at IS NULL AND json_extract(a.data,'$.a')='DK' AND a.created_at>=? AND a.created_at<?`
		args := []any{from.Format(layout), from.AddDate(0, 0, 1).Format(layout)}
		if team != "" {
			q += ` AND a.team_id=?`
			args = append(args, team)
		}
		if agent != "" {
			q += ` AND u.id=?`
			args = append(args, agent)
		}
		var n int
		_ = a.db.QueryRow(q, args...).Scan(&n)
		trend = append(trend, point{from.Format("2006-01-02"), n})
	}
	q := `SELECT a.id,COALESCE(json_extract(a.data,'$.t'),''),COALESCE(a.created_at,''),COALESCE(u.name,'') FROM activity a LEFT JOIN users u ON u.id=a.owner_id WHERE a.deleted_at IS NULL`
	args := []any{}
	if team != "" {
		q += ` AND a.team_id=?`
		args = append(args, team)
	}
	if agent != "" {
		q += ` AND a.owner_id=?`
		args = append(args, agent)
	}
	q += ` ORDER BY a.created_at DESC LIMIT 20`
	events := []any{}
	rows, e := a.db.Query(q, args...)
	if e == nil {
		for rows.Next() {
			var id, detail, at, name string
			if rows.Scan(&id, &detail, &at, &name) == nil {
				events = append(events, map[string]string{"id": id, "detail": detail, "at": at, "agent": name})
			}
		}
		rows.Close()
	}
	respond(w, 200, map[string]any{"kpis": kpis, "trend": trend, "events": events})
}

var exportFields = map[string]string{"address": "trim(COALESCE(p.no,'')||' '||COALESCE(p.street,''))", "suburb": "p.suburb", "lotPlan": "COALESCE(json_extract(p.data,'$.lotplan'),'')", "contactName": "CASE WHEN COALESCE(json_extract(c.flags,'$.optedOut'),0)=0 AND (json_extract(c.consent,'$.phone')=1 OR json_extract(c.consent,'$.email')=1) THEN trim(COALESCE(c.first,'')||' '||COALESCE(c.last,'')) ELSE '' END", "mobile": "CASE WHEN json_extract(c.consent,'$.phone')=1 AND COALESCE(json_extract(c.flags,'$.optedOut'),0)=0 THEN COALESCE(c.mobile,'') ELSE '' END", "email": "CASE WHEN json_extract(c.consent,'$.email')=1 AND COALESCE(json_extract(c.flags,'$.optedOut'),0)=0 THEN COALESCE(c.email,'') ELSE '' END", "tags": "COALESCE(c.tags,'')", "notesCount": "(SELECT count(*) FROM contact_notes n WHERE n.contact_id=c.id)"}

func (a *app) exportCSV(w http.ResponseWriter, r *http.Request, p principal) {
	var m struct {
		Suburb      string   `json:"suburb"`
		Status      string   `json:"status"`
		Fields      []string `json:"fields"`
		Reason      string   `json:"reason"`
		Acknowledge bool     `json:"acknowledge"`
		ExcludeDNC  bool     `json:"excludeDNC"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 65536)).Decode(&m) != nil || strings.TrimSpace(m.Reason) == "" || !m.Acknowledge || len(m.Fields) == 0 || len(m.Fields) > len(exportFields) {
		problem(w, 400, "invalid export request")
		return
	}
	cols := []string{}
	seen := map[string]bool{}
	for _, f := range m.Fields {
		col, ok := exportFields[f]
		if !ok || seen[f] {
			problem(w, 400, "forbidden field")
			return
		}
		seen[f] = true
		cols = append(cols, col)
	}
	q := `SELECT ` + strings.Join(cols, ",") + ` FROM properties p LEFT JOIN property_owners po ON po.property_id=p.id LEFT JOIN contacts c ON c.id=po.contact_id AND c.deleted_at IS NULL WHERE p.deleted_at IS NULL`
	args := []any{}
	if m.Suburb != "" {
		q += ` AND p.suburb=?`
		args = append(args, m.Suburb)
	}
	switch m.Status {
	case "registered":
		q += ` AND p.registered=1`
	case "unregistered":
		q += ` AND p.registered=0`
	case "contact":
		q += ` AND c.id IS NOT NULL`
	case "":
	default:
		problem(w, 400, "invalid status")
		return
	}
	if m.ExcludeDNC {
		q += ` AND COALESCE(json_extract(c.flags,'$.doNotContact'),0)=0 AND COALESCE(json_extract(p.data,'$.doNotKnock'),0)=0`
	}
	q += ` ORDER BY p.id LIMIT 20001`
	rows, e := a.db.Query(q, args...)
	if e != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	records := [][]string{}
	for rows.Next() {
		values := make([]sql.NullString, len(cols))
		dest := make([]any, len(cols))
		for i := range values {
			dest[i] = &values[i]
		}
		if rows.Scan(dest...) != nil {
			problem(w, 500, "database error")
			return
		}
		record := make([]string, len(cols))
		for i, v := range values {
			cell := v.String
			// Excel interprets formula prefixes even inside quoted CSV cells.
			if strings.HasPrefix(cell, "=") || strings.HasPrefix(cell, "+") || strings.HasPrefix(cell, "-") || strings.HasPrefix(cell, "@") {
				cell = "'" + cell
			}
			record[i] = cell
		}
		records = append(records, record)
	}
	if len(records) > 20000 {
		problem(w, 413, "export exceeds 20000 rows")
		return
	}
	diff, _ := json.Marshal(map[string]any{"suburb": m.Suburb, "status": m.Status, "fields": m.Fields, "rows": len(records), "reason": m.Reason, "excludeDNC": m.ExcludeDNC})
	if _, e = a.db.Exec(`INSERT INTO audit_log(user_id,device,action,entity,entity_id,diff,at) VALUES(?,?,?,?,?,?,?)`, p.id, p.device, "export", "properties", ulid(), string(diff), now()); e != nil {
		problem(w, 500, "database error")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="properties.csv"`)
	w.Write([]byte{0xef, 0xbb, 0xbf})
	writer := csv.NewWriter(w)
	_ = writer.Write(m.Fields)
	_ = writer.WriteAll(records)
}
