package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"
)

func (a *app) team(w http.ResponseWriter, p principal) {
	var team string
	if err := a.db.QueryRow(`SELECT team_id FROM users WHERE id=?`, p.id).Scan(&team); err != nil {
		problem(w, 500, "database error")
		return
	}
	rows, err := a.db.Query(`SELECT id,name FROM users WHERE team_id=? AND disabled=0 AND deleted_at IS NULL`, team)
	if err != nil {
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
}

// The cursor is a monotonic change-log sequence, not a timestamp. A high-water
// mark is read before the scan so writes made during a pull remain eligible next time.
func (a *app) syncPull(w http.ResponseWriter, r *http.Request, p principal) {
	since := r.URL.Query().Get("since")
	var from int64
	if since != "" {
		var err error
		decoded, decodeErr := base64.RawURLEncoding.DecodeString(since)
		if decodeErr != nil {
			problem(w, 400, "invalid cursor")
			return
		}
		from, err = strconv.ParseInt(string(decoded), 10, 64)
		if err != nil || from < 0 {
			problem(w, 400, "invalid cursor")
			return
		}
	}
	var high int64
	if err := a.db.QueryRow(`SELECT COALESCE(max(seq),0) FROM sync_changes`).Scan(&high); err != nil {
		problem(w, 500, "database error")
		return
	}
	if since != "" {
		// A cursor issued before `axleo reset-data` (or one from another database)
		// cannot be served incrementally: the change log no longer has tombstones
		// for the cleared rows. Answer with a full snapshot, which clients replace.
		var floor int64
		_ = a.db.QueryRow(`SELECT CAST(value AS INTEGER) FROM settings WHERE key=?`, syncResetKey).Scan(&floor)
		if from <= floor || from > high {
			since = ""
		}
	}
	out := map[string]any{"cursor": base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(high, 10))), "full": since == ""}
	deleted := map[string][]string{}
	for _, kind := range syncKinds {
		items := []any{}
		gone := []string{}
		scopeSQL, args := scope(kind, p)
		q := `SELECT DISTINCT c.id FROM sync_changes c JOIN ` + kind + ` t ON t.id=c.id WHERE c.kind='` + kind + `' AND c.seq>? AND c.seq<=?` + strings.ReplaceAll(scopeSQL, "owner_id=?", "t.owner_id=?")
		// qualify the to-do scope columns as well.
		q = strings.ReplaceAll(strings.ReplaceAll(q, "assignee_id=?", "t.assignee_id=?"), "created_by=?", "t.created_by=?")
		if kind == "properties" {
			q += ` AND t.registered=1`
		}
		params := append([]any{from, high}, args...)
		if since == "" {
			// A new login must see the tables, not depend on surviving change-log entries.
			q = `SELECT t.id FROM ` + kind + ` t WHERE t.deleted_at IS NULL` + strings.ReplaceAll(scopeSQL, "owner_id=?", "t.owner_id=?")
			q = strings.ReplaceAll(strings.ReplaceAll(q, "assignee_id=?", "t.assignee_id=?"), "created_by=?", "t.created_by=?")
			params = args
			if kind == "properties" {
				q += ` AND t.registered=1`
			}
		}
		rows, err := a.db.Query(q, params...)
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
		for _, id := range ids {
			item, err := a.get(kind, id, p, true)
			if err != nil {
				problem(w, 500, "database error")
				return
			}
			if item["deleted_at"] != nil {
				gone = append(gone, id)
			} else {
				items = append(items, item)
			}
		}
		// Records that left this user's scope (reassigned, owner changed) are sent as tombstones.
		if since != "" && p.role != "admin" && kind != "properties" && kind != "listings" {
			seen := map[string]bool{}
			for _, id := range ids {
				seen[id] = true
			}
			lost, err := a.db.Query(`SELECT DISTINCT id FROM sync_changes WHERE kind=? AND seq>? AND seq<=? AND (old_a=? OR old_b=?)`, kind, from, high, p.id, p.id)
			if err != nil {
				problem(w, 500, "database error")
				return
			}
			var lostIDs []string
			for lost.Next() {
				var id string
				if lost.Scan(&id) == nil && !seen[id] {
					lostIDs = append(lostIDs, id)
				}
			}
			lost.Close()
			for _, id := range lostIDs {
				if _, err := a.get(kind, id, p, true); err != nil {
					gone = append(gone, id)
				}
			}
		}
		out[kind] = items
		deleted[kind] = gone
	}
	out["deleted"] = deleted
	respond(w, 200, out)
}

var syncKinds = []string{"contacts", "properties", "todos", "captures", "enquiries", "leads", "listings", "ofi_attendees", "sales", "contracts", "referrals", "market_items", "zones", "tags", "activity", "approvals"}

type syncOp struct {
	OpID        string         `json:"opId"`
	Kind        string         `json:"kind"`
	Op          string         `json:"op"`
	ID          string         `json:"id"`
	BaseVersion *int           `json:"baseVersion"`
	Data        map[string]any `json:"data"`
}

func (a *app) syncPush(w http.ResponseWriter, r *http.Request, p principal) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var input struct {
		Ops []syncOp `json:"ops"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || len(input.Ops) > 100 {
		problem(w, 400, "invalid ops")
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	_, _ = a.db.Exec(`DELETE FROM sync_ops WHERE created_at<?`, time.Now().UTC().Add(-7*24*time.Hour).Format(layout))
	results := []any{}
	for _, op := range input.Ops {
		result := map[string]any{"opId": op.OpID, "status": "invalid"}
		if op.OpID == "" || len(op.OpID) > 128 {
			results = append(results, result)
			continue
		}
		var raw string
		if a.db.QueryRow(`SELECT result FROM sync_ops WHERE user_id=? AND op_id=?`, p.id, op.OpID).Scan(&raw) == nil {
			var saved map[string]any
			_ = json.Unmarshal([]byte(raw), &saved)
			results = append(results, saved)
			continue
		}
		if allowedKind(op.Kind) && op.ID != "" && len(op.ID) <= 128 && (op.Op == "upsert" || op.Op == "delete") {
			method := "POST"
			path := "/api/v1/" + op.Kind
			existing, err := a.get(op.Kind, op.ID, p, false)
			if op.Op == "delete" || (op.BaseVersion != nil && *op.BaseVersion > 0) {
				method = "PATCH"
				if op.Op == "delete" {
					method = "DELETE"
				}
				path += "/" + op.ID
			}
			if method != "POST" && err != nil {
				result["status"] = "notfound"
			} else if method == "DELETE" && op.BaseVersion != nil && existing["version"] != *op.BaseVersion {
				result["status"] = "conflict"
				result["record"] = existing
			} else if method == "POST" && err == nil {
				result["status"] = "conflict"
				result["record"] = existing
			} else if method == "POST" && op.Data == nil {
				result["status"] = "invalid"
			} else {
				data := map[string]any{}
				for k, v := range op.Data {
					data[k] = v
				}
				data["id"] = op.ID
				if method == "PATCH" {
					data["version"] = float64(*op.BaseVersion)
				}
				payload, _ := json.Marshal(data)
				req := httptest.NewRequest(method, path, bytes.NewReader(payload))
				recorder := httptest.NewRecorder()
				a.mutate(recorder, req, p, op.Kind, func() string {
					if method == "POST" {
						return ""
					}
					return op.ID
				}(), data)
				switch recorder.Code {
				case 200, 201:
					result["status"] = "ok"
					if method != "DELETE" {
						var record map[string]any
						_ = json.Unmarshal(recorder.Body.Bytes(), &record)
						result["record"] = record
					}
				case 409:
					result["status"] = "conflict"
					var conflict map[string]any
					_ = json.Unmarshal(recorder.Body.Bytes(), &conflict)
					result["record"] = conflict["current"]
				case 404:
					result["status"] = "notfound"
				default:
					result["status"] = "invalid"
				}
			}
		}
		encoded, _ := json.Marshal(result)
		if _, err := a.db.Exec(`INSERT INTO sync_ops(user_id,op_id,result,created_at) VALUES(?,?,?,?)`, p.id, op.OpID, string(encoded), now()); err != nil {
			problem(w, 500, "database error")
			return
		}
		results = append(results, result)
	}
	respond(w, 200, map[string]any{"results": results})
}
