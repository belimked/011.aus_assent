package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRexWriteAccountGate(t *testing.T) {
	t.Setenv("REX_WRITE_ALLOWED_ACCOUNTS", "2057,758")
	t.Setenv("REX_WRITE_ACCOUNT_ID", "2057")
	t.Setenv("REX_ALLOW_PRODUCTION_WRITE", "")
	for _, tc := range []struct {
		name          string
		id            int
		test          bool
		settingsError bool
		allowed       bool
	}{
		{"sandbox", 2057, true, false, true},
		{"production-flag-missing", 2057, false, false, false},
		{"different-account", 758, true, false, false},
		{"settings-error", 2057, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes, reads := 0, 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/rex/authentication/login":
					_ = json.NewEncoder(w).Encode(map[string]any{"result": "token", "error": nil})
				case "/v1/rex/AccountSettings/read":
					if tc.settingsError {
						w.WriteHeader(500)
						_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "unavailable"}})
						return
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": tc.id, "name": "Test Account", "is_test_account": tc.test}, "error": nil})
				case "/v1/rex/contacts/update":
					writes++
					_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": 1}, "error": nil})
				case "/v1/rex/contacts/read":
					reads++
					_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": 1}, "error": nil})
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			c := newRexClient(srv.URL, "app")
			c.Interval = 0
			c.Retries = 0
			c.AccountID = "2057"
			var logs []rexLog
			c.Log = func(l rexLog) { logs = append(logs, l) }
			if _, err := c.Login(context.Background(), "e", "p", "2057"); err != nil {
				t.Fatal(err)
			}
			_, _, err := c.Call(context.Background(), "Contacts", "update", map[string]any{})
			if tc.allowed != (err == nil) || writes != map[bool]int{true: 1, false: 0}[tc.allowed] {
				t.Fatalf("write err=%v writes=%d", err, writes)
			}
			if !tc.allowed && (!strings.Contains(err.Error(), "blocked:") || logs[len(logs)-1].OK || logs[len(logs)-1].HTTPStatus != 0) {
				t.Fatalf("blocked log: %+v", logs[len(logs)-1])
			}
			if _, _, err = c.Call(context.Background(), "Contacts", "read", map[string]any{"id": 1}); err != nil || reads != 1 {
				t.Fatalf("read err=%v reads=%d", err, reads)
			}
			found := false
			for _, l := range logs {
				if l.Service == "AccountSettings" && l.Method == "read" && l.Kind == "auth" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing verification log")
			}
		})
	}
}
