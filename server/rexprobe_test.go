package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func fakeRex(t *testing.T) (*httptest.Server, *[]string) {
	t.Helper()
	var mu sync.Mutex
	calls := []string{}
	tries := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-App-Identifier") != "Integration:PlaceSunnybank:Portal" {
			t.Errorf("app header: %q", r.Header.Get("X-App-Identifier"))
		}
		path := r.URL.Path
		if path != "/v1/rex/authentication/login" && r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("bearer: %s", path)
		}
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		calls = append(calls, path)
		if path == "/v1/rex/listings/search" {
			tries++
		}
		count := tries
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if path == "/v1/rex/listings/search" && count == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			_, _ = w.Write([]byte(`{"error":"rate limited"}`))
			return
		}
		var result any = map[string]any{"id": "42", "owner_email": "personal@example.com", "name": "Real Person"}
		switch path {
		case "/v1/rex/authentication/login":
			if b["password"] != "my-password" {
				t.Error("login payload")
			}
			result = "secret-token"
		case "/v1/rex/user-profile/get-accessible-accounts":
			result = []any{map[string]any{"id": "sandbox", "name": "Company"}}
		case "/v1/rex/contacts/create":
			result = map[string]any{"id": "100"}
		case "/v1/rex/properties/create":
			result = map[string]any{"id": "101"}
		case "/v1/rex/leads/create":
			result = map[string]any{"id": "102"}
		case "/v1/rex/reminders/create":
			result = map[string]any{"id": "103"}
		case "/v1/rex/listings/search":
			result = []any{map[string]any{"id": "5"}}
		case "/v1/rex/account-users/search":
			result = []any{map[string]any{"id": "6"}}
		case "/v1/rex/properties/search":
			result = []any{map[string]any{"id": "7"}}
		case "/v1/rex/system-values/get-value-categories":
			result = []any{map[string]any{"id": "category"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	return srv, &calls
}
func setupProbe(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	t.Setenv("REX_BASE_URL", srv.URL)
	t.Setenv("REX_EMAIL", "login@example.com")
	t.Setenv("REX_PASSWORD", "my-password")
	t.Setenv("REX_ACCOUNT_ID", "sandbox")
	t.Setenv("REX_INTERVAL_MS", "0")
	return filepath.Join(".tmp", "rex-test-"+strings.ReplaceAll(t.Name(), "/", "-"))
}
func TestRexClientPathsAnd429(t *testing.T) {
	srv, calls := fakeRex(t)
	defer srv.Close()
	c := newRexClient(srv.URL, "Integration:PlaceSunnybank:Portal")
	c.Interval = 0
	if rexPath("AdminTags") != "admin-tags" || rexPath("findMatchingContacts") != "find-matching-contacts" {
		t.Fatal("path conversion")
	}
	if _, err := c.Login(context.Background(), "login@example.com", "my-password", "sandbox"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := c.Call(context.Background(), "Listings", "search", map[string]any{"limit": 1}); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 3 {
		t.Fatalf("expected 429 retry: %v", *calls)
	}
}
func TestRexProbeGuardsAndFullRun(t *testing.T) {
	srv, calls := fakeRex(t)
	defer srv.Close()
	base := setupProbe(t, srv)
	defer os.RemoveAll(base)
	if err := rexProbe([]string{"--out", base}); err == nil {
		t.Fatal("missing account accepted")
	}
	for _, path := range *calls {
		if strings.Contains(path, "/create") {
			t.Fatal("guard made write request")
		}
	}
	*calls = nil
	if err := rexProbe([]string{"--account", "sandbox", "--out", base, "--only", "F1,F2,F3,F5"}); err != nil {
		t.Fatal(err)
	}
	for _, path := range *calls {
		if strings.Contains(path, "/create") {
			t.Fatal("read-only run wrote data")
		}
	}
	*calls = nil
	if err := rexProbe([]string{"--account", "sandbox", "--write", "--remindee", "6", "--out", base}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(base, "report.md"))
	if err != nil || !strings.Contains(string(b), "F7") {
		t.Fatalf("report: %s %v", b, err)
	}
	state, err := os.ReadFile(filepath.Join(base, "state.json"))
	if err != nil || !strings.Contains(string(state), `"records": []`) {
		t.Fatalf("state: %s %v", state, err)
	}
	foundDelete := false
	for _, path := range *calls {
		if strings.HasSuffix(path, "/delete") {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Fatal("no cleanup requests")
	}
	err = filepath.Walk(base, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}
		if info.IsDir() {
			return nil
		}
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		for _, secret := range []string{"my-password", "secret-token", "personal@example.com", "Real Person"} {
			if strings.Contains(string(b), secret) {
				t.Errorf("leak in %s: %s", path, secret)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestRexProbeKeepAndCleanup(t *testing.T) {
	srv, calls := fakeRex(t)
	defer srv.Close()
	dir := setupProbe(t, srv)
	defer os.RemoveAll(dir)
	if err := rexProbe([]string{"--account", "sandbox", "--write", "--only", "F1", "--keep", "--out", dir}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil || !strings.Contains(string(b), "100") {
		t.Fatalf("state: %s %v", b, err)
	}
	if err := rexProbe([]string{"cleanup", "--state", filepath.Join(dir, "state.json")}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(filepath.Join(dir, "state.json"))
	if strings.Contains(string(b), "100") {
		t.Fatal("state not cleared")
	}
	_ = calls
}
func TestProbeRateDefault(t *testing.T) {
	c := newRexClient("http://localhost", "app")
	if c.Interval != time.Second {
		t.Fatal(c.Interval)
	}
}
