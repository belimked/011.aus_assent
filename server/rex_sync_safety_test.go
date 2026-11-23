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
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"id": 1}, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "app")
	c.Interval = 0
	c.Retries = 0
	c.AccountID = "2057"
	logs := []rexLog{}
	c.Log = func(l rexLog) { logs = append(logs, l) }
	for _, actual := range []string{"", "758", "2057"} {
		c.VerifiedAccount = actual
		if actual == "2057" {
			c.AccountID = "758"
		} else {
			c.AccountID = "2057"
		}
		_, _, err := c.Call(context.Background(), "Contacts", "update", map[string]any{"data": map[string]any{"id": 1}})
		if err == nil || !strings.Contains(err.Error(), "blocked:") {
			t.Fatalf("%q: %v", actual, err)
		}
	}
	if hits != 0 || len(logs) != 3 {
		t.Fatalf("hits=%d logs=%d", hits, len(logs))
	}
	for _, l := range logs {
		if l.OK || l.HTTPStatus != 0 {
			t.Fatalf("blocked log: %+v", l)
		}
	}
	c.AccountID, c.VerifiedAccount = "2057", "2057"
	if _, _, err := c.Call(context.Background(), "Contacts", "update", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
	c.VerifiedAccount = ""
	if _, _, err := c.Call(context.Background(), "Contacts", "read", map[string]any{"id": 1}); err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("read blocked: hits=%d", hits)
	}
}
