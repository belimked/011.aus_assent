package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestConsolePermissionsAndExport(t *testing.T) {
	a, tokens := setup(t)
	var mary string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`UPDATE users SET role='leader' WHERE id=?`, mary); err != nil {
		t.Fatal(err)
	}
	admin := tokens["admi"]
	leader := tokens["mary"]
	for _, path := range []string{"export", "exports", "settings", "audit", "rex-outbox", "market-review", "data"} {
		w := request(a, "GET", "/api/v1/admin/"+path, leader, nil)
		if w.Code != 403 {
			t.Errorf("leader GET %s: %d", path, w.Code)
		}
	}
	if w := request(a, "PATCH", "/api/v1/admin/users", leader, map[string]any{}); w.Code != 403 {
		t.Fatalf("leader account write: %d", w.Code)
	}
	if w := request(a, "POST", "/api/v1/admin/export", leader, map[string]any{}); w.Code != 403 {
		t.Fatal(w.Code)
	}
	if w := request(a, "GET", "/api/v1/admin/metrics?period=7", leader, nil); w.Code != 200 {
		t.Fatalf("metrics: %d %s", w.Code, w.Body)
	}
	if _, e := a.db.Exec(`INSERT INTO teams(id,name) VALUES('other','Other')`); e != nil {
		t.Fatal(e)
	}
	if _, e := a.db.Exec(`UPDATE users SET team_id='other' WHERE email='john@axleo.local'`); e != nil {
		t.Fatal(e)
	}
	w := request(a, "GET", "/api/v1/admin/metrics?period=7&agent=outside", leader, nil)
	if w.Code != 403 {
		t.Fatalf("outside metrics: %d", w.Code)
	}
	var john string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&john)
	w = request(a, "GET", "/api/v1/admin/metrics?period=7&agent="+john, leader, nil)
	if w.Code != 403 {
		t.Fatalf("other team metrics: %d", w.Code)
	}
	if _, e := a.db.Exec(`INSERT INTO activity(id,owner_id,team_id,data,created_at,updated_at) VALUES('other-visit',?,'other','{"a":"DK"}',?,?)`, john, now(), now()); e != nil {
		t.Fatal(e)
	}
	w = request(a, "GET", "/api/v1/admin/metrics?period=7", leader, nil)
	if w.Code != 200 || obj(t, w)["kpis"].(map[string]any)["visits"].(map[string]any)["value"].(float64) != 0 {
		t.Fatalf("team metrics: %s", w.Body)
	}
	if _, e := a.db.Exec(`INSERT INTO todos(id,assignee_id,created_by,team_id,data,created_at,updated_at) VALUES('other-task',?,?,'other','{"t":"secret"}',?,?)`, john, john, now(), now()); e != nil {
		t.Fatal(e)
	}
	w = request(a, "GET", "/api/v1/admin/todos", leader, nil)
	if w.Code != 200 || strings.Contains(w.Body.String(), "secret") {
		t.Fatalf("team todos: %s", w.Body)
	}
	if w := request(a, "POST", "/api/v1/admin/todos", leader, map[string]any{"assigneeId": "outside", "t": "bad", "d": "2026-01-01"}); w.Code != 403 {
		t.Fatalf("outside team: %d %s", w.Code, w.Body)
	}
	if _, e := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at) VALUES('test','1','Example','Sunnybank','{}',?,?)`, now(), now()); e != nil {
		t.Fatal(e)
	}
	w = request(a, "GET", "/api/v1/admin/properties", leader, nil)
	if w.Code != 200 || bytes.Contains(w.Body.Bytes(), []byte("ownerNames")) || bytes.Contains(w.Body.Bytes(), []byte("lastSalePrice")) {
		t.Fatalf("property exposure: %d %s", w.Code, w.Body)
	}
	w = request(a, "POST", "/api/v1/admin/export", admin, map[string]any{"fields": []string{"address", "suburb"}, "reason": "Approved follow-up", "acknowledge": true, "excludeDNC": true})
	if w.Code != 200 || !bytes.HasPrefix(w.Body.Bytes(), []byte{0xef, 0xbb, 0xbf}) || !strings.Contains(w.Body.String(), "1 Example") {
		t.Fatalf("CSV: %d %s", w.Code, w.Body)
	}
	var n int
	if _, e := a.db.Exec(`UPDATE properties SET flags='{"dnc":true}' WHERE id='test'`); e != nil {
		t.Fatal(e)
	}
	w = request(a, "POST", "/api/v1/admin/export", admin, map[string]any{"fields": []string{"address"}, "reason": "DNC check", "acknowledge": true, "excludeDNC": true})
	if w.Code != 200 || strings.Contains(w.Body.String(), "1 Example") {
		t.Fatalf("DNC export: %s", w.Body)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM audit_log WHERE action='export'`).Scan(&n)
	if n != 2 {
		t.Fatalf("export audit count %d", n)
	}
	w = request(a, "POST", "/api/v1/admin/export", admin, map[string]any{"fields": []string{"ownerNames"}, "reason": "bad", "acknowledge": true})
	if w.Code != 400 {
		t.Fatalf("licensed field: %d", w.Code)
	}
}

func TestConsoleSettingsSessionAndTeams(t *testing.T) {
	a, tokens := setup(t)
	admin := tokens["admi"]
	w := request(a, "PATCH", "/api/v1/admin/settings", admin, map[string]any{"sessionHours": "4"})
	if w.Code != 200 {
		t.Fatalf("settings: %d %s", w.Code, w.Body)
	}
	w = request(a, "POST", "/api/v1/auth/login", "", map[string]any{"email": "john@axleo.local", "password": "password123", "device": "phone"})
	if w.Code != 200 {
		t.Fatalf("login: %s", w.Body)
	}
	_ = obj(t, w)["token"].(string)
	var hours float64
	_ = a.db.QueryRow(`SELECT (julianday(expires_at)-julianday(created_at))*24 FROM sessions WHERE device='phone'`).Scan(&hours)
	if hours < 3.9 || hours > 4.1 {
		t.Fatalf("expiry hours: %v", hours)
	}
	w = request(a, "POST", "/api/v1/admin/teams", admin, map[string]any{"name": "West"})
	if w.Code != 200 {
		t.Fatalf("team: %d %s", w.Code, w.Body)
	}
}
