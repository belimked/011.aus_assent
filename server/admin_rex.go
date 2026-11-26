package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func (a *app) adminRex(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != "GET" {
		problem(w, 405, "method not allowed")
		return
	}
	if path == "rex/status" {
		c := loadRexConfig()
		var calls, failures int
		_ = a.rd().QueryRow(`SELECT count(*),COALESCE(sum(CASE WHEN ok=0 THEN 1 ELSE 0 END),0) FROM rex_request_log WHERE at>=?`, time.Now().UTC().Add(-time.Hour).Format(layout)).Scan(&calls, &failures)
		var loginAt string
		var loginOK int
		err := a.rd().QueryRow(`SELECT at,ok FROM rex_request_log WHERE kind='auth' AND direction='out' ORDER BY id DESC LIMIT 1`).Scan(&loginAt, &loginOK)
		if err == sql.ErrNoRows {
			loginAt = ""
		}
		verified, name, isTest, verificationError := rexVerifiedStatus(a.rd())
		blocked := !c.Write || verified == "" || verified != c.AccountID || verified != c.WriteAccountID || !rexAccountAllowed(verified) || !isTest && os.Getenv("REX_ALLOW_PRODUCTION_WRITE") != "1"
		reason := ""
		switch {
		case !c.Write:
			reason = c.WriteReason
		case verificationError != "":
			reason = verificationError
		case verified == "":
			reason = "Rex active account not verified"
		case verified != c.AccountID || verified != c.WriteAccountID:
			reason = "Rex active account mismatches configured account"
		case !rexAccountAllowed(verified):
			reason = "account not in write allowlist"
		case !isTest && os.Getenv("REX_ALLOW_PRODUCTION_WRITE") != "1":
			reason = "production account requires REX_ALLOW_PRODUCTION_WRITE=1"
		}
		var lastPoll string
		var pollOK int
		_ = a.rd().QueryRow(`SELECT at,ok FROM rex_poll_status WHERE id=1`).Scan(&lastPoll, &pollOK)
		respond(w, 200, map[string]any{"accountId": c.AccountID, "verifiedAccount": verified, "verifiedAccountName": name, "verifiedIsTest": isTest, "writeBlocked": blocked, "writeBlockReason": reason, "pollEnabled": c.Read, "lastPollAt": lastPoll, "lastPollOK": pollOK == 1 && lastPoll != "", "baseUrl": c.BaseURL, "appId": c.AppID, "read": c.Read, "write": c.Write, "readReason": c.ReadReason, "writeReason": c.WriteReason, "lastLoginAt": loginAt, "lastLoginOK": loginOK == 1 && loginAt != "", "callsHour": calls, "failuresHour": failures})
		return
	}
	if path == "rex/events" {
		a.adminRexEvents(w, r)
		return
	}
	if strings.HasPrefix(path, "rex/log/") {
		id, e := strconv.ParseInt(strings.TrimPrefix(path, "rex/log/"), 10, 64)
		if e != nil || id < 1 {
			problem(w, 400, "invalid id")
			return
		}
		row := a.rd().QueryRow(`SELECT id,at,account_id,direction,kind,service,method,path,caller,attempt,request_json,http_status,ok,error,duration_ms,result_summary,response_excerpt,entity,local_id,rex_id FROM rex_request_log WHERE id=?`, id)
		item, err := scanRexRow(row.Scan)
		if err != nil {
			problem(w, 404, "not found")
			return
		}
		respond(w, 200, item)
		return
	}
	if path != "rex/log" {
		problem(w, 404, "not found")
		return
	}
	q := `SELECT id,at,account_id,direction,kind,service,method,path,caller,attempt,request_json,http_status,ok,error,duration_ms,result_summary,response_excerpt,entity,local_id,rex_id FROM rex_request_log WHERE 1=1`
	args := []any{}
	for _, f := range []struct{ key, col, op string }{{"from", "at", ">="}, {"to", "at", "<="}, {"service", "service", "="}, {"method", "method", "="}, {"kind", "kind", "="}, {"ok", "ok", "="}, {"caller", "caller", "="}, {"entity", "entity", "="}, {"local_id", "local_id", "="}, {"rex_id", "rex_id", "="}} {
		if v := r.URL.Query().Get(f.key); v != "" {
			if f.key == "ok" && v != "0" && v != "1" {
				problem(w, 400, "invalid ok")
				return
			}
			q += " AND " + f.col + f.op + "?"
			args = append(args, v)
		}
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if before > 0 {
		q += " AND id<?"
		args = append(args, before)
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 200 {
		limit = 50
	}
	q += " ORDER BY id DESC LIMIT ?"
	args = append(args, limit)
	rows, err := a.rd().Query(q, args...)
	if err != nil {
		problem(w, 500, "database error")
		return
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		item, e := scanRexRow(rows.Scan)
		if e != nil {
			problem(w, 500, "database error")
			return
		}
		delete(item, "requestJson")
		delete(item, "responseExcerpt")
		items = append(items, item)
	}
	if rows.Err() != nil {
		problem(w, 500, "database error")
		return
	}
	respond(w, 200, map[string]any{"items": items})
}
func rexVerifiedStatus(db querier) (account, name string, isTest bool, reason string) {
	var raw string
	_ = db.QueryRow(`SELECT response_excerpt,error FROM rex_request_log WHERE service='AccountSettings' AND method='read' AND caller='verification' ORDER BY id DESC LIMIT 1`).Scan(&raw, &reason)
	var result struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		IsTest bool   `json:"is_test_account"`
	}
	if json.Unmarshal([]byte(raw), &result) == nil {
		account, name, isTest = result.ID, result.Name, result.IsTest
	}
	return
}
func scanRexRow(scan func(...any) error) (map[string]any, error) {
	var id, attempt, status, ok, duration int64
	var at, account, direction, kind, service, method, path, caller, request, reason, summary, response, entity, localID, rexID string
	err := scan(&id, &at, &account, &direction, &kind, &service, &method, &path, &caller, &attempt, &request, &status, &ok, &reason, &duration, &summary, &response, &entity, &localID, &rexID)
	return map[string]any{"entity": entity, "localId": localID, "rexId": rexID, "id": id, "at": at, "accountId": account, "direction": direction, "kind": kind, "service": service, "method": method, "path": path, "caller": caller, "attempt": attempt, "requestJson": request, "httpStatus": status, "ok": ok == 1, "error": reason, "durationMs": duration, "resultSummary": summary, "responseExcerpt": response}, err
}
