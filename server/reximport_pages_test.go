package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRexImportGuard(t *testing.T) {
	t.Setenv("REX_ACCOUNT_ID", "2057")
	t.Setenv("AXLEO_REX_READ", "0")
	if err := rexImportCLI([]string{"--account", "2057"}); err == nil {
		t.Fatal("read gate bypassed")
	}
	t.Setenv("AXLEO_REX_READ", "1")
	if err := rexImportCLI([]string{"--account", "758"}); err == nil {
		t.Fatal("account gate bypassed")
	}
}

func TestImportPages(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		if ob, _ := b["order_by"].(map[string]any); ob["id"] != "asc" || b["limit"] != float64(100) {
			t.Error("pagination request", b)
		}
		off := int(b["offset"].(float64))
		rows := []any{}
		for i := off; i < off+100 && i < 101; i++ {
			rows = append(rows, map[string]any{"id": fmt.Sprint(i), "etag": "v1"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"rows": rows, "total": 101}, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Interval = 0
	count := 0
	if err := importPages(context.Background(), c, "Contacts", 0, func(r importRow) error { count++; return nil }); err != nil {
		t.Fatal(err)
	}
	if count != 101 || calls != 2 {
		t.Fatalf("got %d rows in %d requests", count, calls)
	}
}
