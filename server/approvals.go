package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// Requests use {kind:"merge",primaryId,secondaryId} or
// {kind:"reassign",entity:"contacts"|"todos",id,targetId}.
func (a *app) approvalRoute(w http.ResponseWriter, r *http.Request, p principal) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if path == "approvals" && r.Method == "GET" {
		a.approvalList(w, p)
		return
	}
	if path == "approvals" && r.Method == "POST" {
		m, err := body(r)
		if err != nil || !a.validRequest(m, p) {
			problem(w, 404, "not found")
			return
		}
		id := ulid()
		if str(m, "kind") == "reassign" {
			m["recordId"] = str(m, "id")
		}
		m["id"] = id
		m["status"] = "pending"
		m["ownerId"] = p.id
		b, _ := json.Marshal(m)
		tx, _ := a.db.Begin()
		defer tx.Rollback()
		_, err = tx.Exec(`INSERT INTO approvals(id,kind,payload,status,requested_by,owner_id,data,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)`, id, str(m, "kind"), string(b), "pending", p.id, p.id, string(b), now(), now())
		if err == nil {
			err = a.record(tx, p, "POST", "approvals", id, m)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, 400, "invalid request")
			return
		}
		respond(w, 201, m)
		return
	}
	if strings.HasPrefix(path, "approvals/") && strings.HasSuffix(path, "/decide") && r.Method == "POST" {
		if p.role != "admin" {
			problem(w, 404, "not found")
			return
		}
		id := strings.TrimSuffix(strings.TrimPrefix(path, "approvals/"), "/decide")
		m, err := body(r)
		if err != nil || (str(m, "decision") != "approve" && str(m, "decision") != "reject") {
			problem(w, 400, "invalid decision")
			return
		}
		var raw, status string
		err = a.db.QueryRow(`SELECT data,status FROM approvals WHERE id=? AND deleted_at IS NULL`, id).Scan(&raw, &status)
		if err != nil || status != "pending" {
			problem(w, 404, "not found")
			return
		}
		var request map[string]any
		_ = json.Unmarshal([]byte(raw), &request)
		previousAssignee := a.previousApprovalAssignee(request)
		tx, err := a.db.Begin()
		if err != nil {
			problem(w, 500, "database error")
			return
		}
		defer tx.Rollback()
		if str(m, "decision") == "approve" {
			err = a.executeApproval(tx, p, request)
		}
		if err == nil {
			request["status"] = map[bool]string{true: "approved", false: "rejected"}[str(m, "decision") == "approve"]
			request["note"] = str(m, "note")
			request["decidedBy"] = p.id
			b, _ := json.Marshal(request)
			_, err = tx.Exec(`UPDATE approvals SET data=?,status=?,decided_by=?,version=version+1,updated_at=? WHERE id=? AND status='pending'`, string(b), str(request, "status"), p.id, now(), id)
		}
		if err == nil {
			err = a.record(tx, p, "decide", "approvals", id, request)
		}
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, 409, "request no longer valid")
			return
		}
		if str(m, "decision") == "approve" {
			a.notifyApprovedTodo(request, previousAssignee, p.id)
		}
		respond(w, 200, request)
		return
	}
	// Merge: preview, then execute with the preview hash. No approval step; agents
	// merge two of their own contacts, admins any two (contact_merge.go).
	if r.Method == "POST" && path == "contacts/merge-preview" {
		m, err := body(r)
		if err != nil {
			problem(w, 400, "invalid JSON")
			return
		}
		a.mergeRoute(w, r, p, str(m, "aId"), str(m, "bId"), true, "")
		return
	}
	if parts := strings.Split(path, "/"); r.Method == "POST" && len(parts) == 3 && parts[0] == "contacts" && parts[2] == "merge" {
		m, err := body(r)
		if err != nil {
			problem(w, 400, "invalid JSON")
			return
		}
		a.mergeRoute(w, r, p, parts[1], str(m, "secondaryId"), false, str(m, "previewHash"))
		return
	}
	// Direct reassign (prototype: no approval step). validRequest limits agents to their own records and teammates.
	if r.Method == "POST" && strings.HasSuffix(path, "/reassign") {
		m, err := body(r)
		if err != nil {
			problem(w, 400, "invalid JSON")
			return
		}
		parts := strings.Split(path, "/")
		if len(parts) != 3 {
			problem(w, 404, "not found")
			return
		}
		m["id"] = parts[1]
		if parts[2] == "reassign" && (parts[0] == "contacts" || parts[0] == "todos") {
			m["kind"] = "reassign"
			m["entity"] = parts[0]
		} else {
			problem(w, 404, "not found")
			return
		}
		if !a.validRequest(m, p) {
			problem(w, 404, "not found")
			return
		}
		previousAssignee := a.previousApprovalAssignee(m)
		tx, _ := a.db.Begin()
		defer tx.Rollback()
		err = a.executeApproval(tx, p, m)
		if err == nil {
			err = tx.Commit()
		}
		if err != nil {
			problem(w, 409, "conflict")
			return
		}
		a.notifyApprovedTodo(m, previousAssignee, p.id)
		respond(w, 200, map[string]any{"ok": true})
		return
	}
	problem(w, 404, "not found")
}
func enqueueRexMerge(tx *sql.Tx, id string, m map[string]any) error {
	payload := map[string]any{"primaryId": str(m, "primaryId"), "secondaryId": str(m, "secondaryId")}
	for k, v := range imap(m["rexPlan"]) {
		payload[k] = v
	}
	b, _ := json.Marshal(payload)
	_, err := tx.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_merge',?,'POST',?,?)`, id, string(b), now())
	return err
}
func (a *app) validRequest(m map[string]any, p principal) bool { return a.validRequestFrom(a.db, m, p) }
func (a *app) validRequestFrom(db querier, m map[string]any, p principal) bool {
	if str(m, "kind") == "merge" {
		x, y := str(m, "primaryId"), str(m, "secondaryId")
		if x == "" || y == "" || x == y {
			return false
		}
		_, e := a.getFrom(db, "contacts", x, p, false)
		_, f := a.getFrom(db, "contacts", y, p, false)
		return e == nil && f == nil
	}
	if str(m, "kind") == "reassign" {
		kind := str(m, "entity")
		if kind != "contacts" && kind != "todos" {
			return false
		}
		if _, err := a.getFrom(db, kind, str(m, "id"), p, false); err != nil {
			return false
		}
		var n int
		_ = db.QueryRow(`SELECT count(*) FROM users WHERE id=? AND team_id=(SELECT team_id FROM users WHERE id=?) AND disabled=0 AND deleted_at IS NULL`, str(m, "targetId"), p.id).Scan(&n)
		return n == 1
	}
	return false
}
func (a *app) approvalList(w http.ResponseWriter, p principal) {
	s, args := scope("approvals", p)
	if p.role == "admin" {
		s += ` AND status='pending'`
	}
	rows, err := a.rd().Query(`SELECT id FROM approvals WHERE deleted_at IS NULL`+s+` ORDER BY created_at DESC`, args...)
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
		v, e := a.getFrom(a.rd(), "approvals", id, p, false)
		if e == nil {
			items = append(items, v)
		}
	}
	respond(w, 200, items)
}
func (a *app) executeApproval(tx *sql.Tx, p principal, m map[string]any) error {
	stamp := now()
	if str(m, "kind") == "reassign" {
		kind := str(m, "entity")
		col := "owner_id"
		field := "ownerId"
		if kind == "todos" {
			col = "assignee_id"
			field = "assigneeId"
		}
		id := str(m, "recordId")
		if id == "" {
			id = str(m, "id")
		}
		target := str(m, "targetId")
		var raw string
		err := tx.QueryRow(`SELECT data FROM `+kind+` WHERE id=? AND deleted_at IS NULL`, id).Scan(&raw)
		if err != nil {
			return err
		}
		var v map[string]any
		_ = json.Unmarshal([]byte(raw), &v)
		v[field] = target
		if kind == "todos" {
			v["createdBy"] = target
		}
		b, _ := json.Marshal(v)
		if kind == "todos" {
			_, err = tx.Exec(`UPDATE todos SET assignee_id=?,created_by=?,data=?,updated_at=?,version=version+1 WHERE id=?`, target, target, string(b), stamp, id)
		} else {
			_, err = tx.Exec(`UPDATE `+kind+` SET `+col+`=?,data=?,updated_at=?,version=version+1 WHERE id=?`, target, string(b), stamp, id)
		}
		if err == nil {
			err = a.record(tx, p, "reassign", kind, id, m)
		}
		return err
	}
	// Legacy pending merge approvals are executed with the current merge rules
	// (contact_merge.go); the admin's decision replaces the user's preview.
	if str(m, "kind") != "merge" {
		return errors.New("unknown approval kind")
	}
	plan, err := a.executeMerge(tx, p, str(m, "primaryId"), str(m, "secondaryId"), "")
	if err == nil {
		m["mergedInto"] = plan.PrimaryID
	}
	return err
}
