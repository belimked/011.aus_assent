package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRexConfigGuards(t *testing.T) {
	t.Setenv("REX_BASE_URL", "https://example.invalid")
	t.Setenv("REX_APP_ID", "app")
	t.Setenv("REX_EMAIL", "me@example.com")
	t.Setenv("REX_PASSWORD", "secret")
	t.Setenv("REX_ACCOUNT_ID", "")
	t.Setenv("AXLEO_REX_READ", "1")
	t.Setenv("AXLEO_REX_WRITE", "1")
	t.Setenv("REX_WRITE_ACCOUNT_ID", "758")
	if c := loadRexConfig(); c.canRead() || c.canWrite() {
		t.Fatal(c)
	}
	t.Setenv("REX_ACCOUNT_ID", "758")
	t.Setenv("REX_WRITE_ACCOUNT_ID", "2057")
	if c := loadRexConfig(); !c.canRead() || c.canWrite() || !strings.Contains(c.WriteReason, "match") {
		t.Fatal(c)
	}
	t.Setenv("REX_WRITE_ACCOUNT_ID", "758")
	t.Setenv("AXLEO_REX_ENABLED", "0")
	if c := loadRexConfig(); !c.canWrite() || !c.canRead() {
		t.Fatal(c)
	}
}
func TestRexLogAttemptsAndSecrets(t *testing.T) {
	a, _ := setup(t)
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		if count == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			return
		}
		if r.URL.Path == "/v1/rex/AccountSettings/read" {
			_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": 2057, "name": "Sandbox", "is_test_account": true}, "error": nil})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": "secret-token", "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "app")
	c.Interval = 0
	c.AccountID = "2057"
	c.Caller = "probe"
	c.Password = "my-password"
	c.Log = rexDBLogger(a.db)
	if _, e := c.Login(context.Background(), "me@example.com", "my-password", "2057"); e != nil {
		t.Fatal(e)
	}
	var n int
	var request, response string
	if e := a.db.QueryRow(`SELECT count(*) FROM rex_request_log`).Scan(&n); e != nil || n != 4 {
		t.Fatalf("attempts=%d %v", n, e)
	}
	account, name, test, reason := rexVerifiedStatus(a.db)
	if account != "2057" || name != "Sandbox" || !test || reason != "" {
		t.Fatalf("verification: %q %q %v %q", account, name, test, reason)
	}
	if e := a.db.QueryRow(`SELECT request_json,response_excerpt FROM rex_request_log ORDER BY id DESC LIMIT 1`).Scan(&request, &response); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(request+response, "my-password") || strings.Contains(request+response, "secret-token") {
		t.Fatal("credentials leaked")
	}
	if rexKind("Authentication", "login") != "auth" || rexKind("Contacts", "describeSearchFields") != "read" || rexKind("Contacts", "unknown") != "write" {
		t.Fatal("kind")
	}
	c.Log = rexDBLogger(a.db)
	_, _ = a.db.Exec(`DROP TABLE rex_request_log`)
	if _, _, e := c.Call(context.Background(), "Contacts", "read", map[string]any{}); e != nil {
		t.Fatal(e)
	}
}
func TestAdminRexAccess(t *testing.T) {
	a, tokens := setup(t)
	_, _ = a.db.Exec(`UPDATE users SET role='leader' WHERE email='mary@axleo.local'`)
	for _, path := range []string{"rex/status", "rex/log", "rex/log/1"} {
		if w := request(a, "GET", "/api/v1/admin/"+path, tokens["mary"], nil); w.Code != 403 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if w := request(a, "GET", "/api/v1/admin/rex/status", tokens["admi"], nil); w.Code != 200 {
		t.Fatalf("admin: %d %s", w.Code, w.Body.String())
	}
}
