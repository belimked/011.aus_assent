package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (a *app) overview(w http.ResponseWriter) {
	counts := map[string]int64{}
	for _, kind := range syncKinds {
		var n int64
		if err := a.db.QueryRow(`SELECT count(*) FROM ` + kind + ` WHERE deleted_at IS NULL`).Scan(&n); err != nil {
			problem(w, 500, "database error")
			return
		}
		counts[kind] = n
	}
	var users, pending, writes, schema int64
	_ = a.db.QueryRow(`SELECT count(*) FROM users WHERE deleted_at IS NULL`).Scan(&users)
	_ = a.db.QueryRow(`SELECT count(*) FROM approvals WHERE status='pending' AND deleted_at IS NULL`).Scan(&pending)
	_ = a.db.QueryRow(`SELECT count(*) FROM audit_log WHERE at>=?`, time.Now().UTC().Add(-24*time.Hour).Format(layout)).Scan(&writes)
	_ = a.db.QueryRow(`SELECT count(*) FROM schema_migrations`).Scan(&schema)
	rex := map[string]int64{"waiting_auth": 0, "pending": 0, "sent": 0, "failed": 0, "skipped": 0}
	rows, err := a.db.Query(`SELECT status,count(*) FROM rex_outbox GROUP BY status`)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	for rows.Next() {
		var s string
		var n int64
		if rows.Scan(&s, &n) == nil {
			rex[s] = n
		}
	}
	rows.Close()
	respond(w, 200, map[string]any{"counts": counts, "users": users, "pendingApprovals": pending, "writes24h": writes, "rex": rex, "version": version, "schema": schema})
}
func pageLimit(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if n < 1 || n > 100 {
		return 50
	}
	return n
}
func (a *app) adminData(w http.ResponseWriter, r *http.Request, p principal) {
	kind := r.URL.Query().Get("kind")
	if !allowedKind(kind) {
		problem(w, 400, "invalid kind")
		return
	}
	before := r.URL.Query().Get("before")
	q := `SELECT id FROM ` + kind + ` WHERE deleted_at IS NULL`
	args := []any{}
	if before != "" {
		q += ` AND id>?`
		args = append(args, before)
	}
	q += ` ORDER BY id LIMIT ?`
	args = append(args, pageLimit(r))
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
	items := []any{}
	for _, id := range ids {
		v, e := a.get(kind, id, p, false)
		if e == nil {
			items = append(items, v)
		}
	}
	respond(w, 200, items)
}
func (a *app) adminApprovals(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != "pending" && status != "approved" && status != "rejected" {
		problem(w, 400, "invalid status")
		return
	}
	q := `SELECT a.data,u.name FROM approvals a LEFT JOIN users u ON u.id=a.requested_by WHERE a.deleted_at IS NULL`
	args := []any{}
	if status != "" {
		q += ` AND a.status=?`
		args = append(args, status)
	}
	if before := r.URL.Query().Get("before"); before != "" {
		q += ` AND a.id<?`
		args = append(args, before)
	}
	q += ` ORDER BY a.id DESC LIMIT ?`
	args = append(args, pageLimit(r))
	rows, err := a.db.Query(q, args...)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	out := []any{}
	for rows.Next() {
		var raw string
		var user sql.NullString
		if rows.Scan(&raw, &user) != nil {
			continue
		}
		var item map[string]any
		if json.Unmarshal([]byte(raw), &item) != nil {
			continue
		}
		item["requesterName"] = user.String
		out = append(out, item)
	}
	rows.Close()
	for _, entry := range out {
		item := entry.(map[string]any)
		// Resolve a human-readable target without exposing raw SQL identifiers.
		kind := str(item, "entity")
		id := str(item, "recordId")
		if id == "" {
			id = str(item, "primaryId")
		}
		if kind == "" && str(item, "kind") == "merge" {
			kind = "contacts"
		}
		if (kind == "contacts" || kind == "todos") && id != "" {
			var data string
			if a.db.QueryRow(`SELECT data FROM `+kind+` WHERE id=?`, id).Scan(&data) == nil {
				var target map[string]any
				if json.Unmarshal([]byte(data), &target) == nil {
					summary := strings.TrimSpace(str(target, "first") + " " + str(target, "last"))
					if summary == "" {
						summary = str(target, "t")
					}
					item["summary"] = summary
				}
			}
		}
	}
	respond(w, 200, out)
}
