package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Synthetic first-login load for one agent. This is a response-size and
// end-to-end handler timing measurement, not a network or device benchmark.
func TestSyncImportScaleSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("large sync measurement")
	}
	a, tokens := setup(t)
	var owner string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	tx, err := a.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	cs, err := tx.Prepare(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES(?,?,?,'now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4000; i++ {
		id := fmt.Sprintf("rex-c-%05d", i)
		data := map[string]any{"id": id, "first": "Contact", "last": "Example", "mobile": "0412345678", "email": fmt.Sprintf("agent%d@example.test", i), "ownerIds": []any{}, "flags": map[string]any{"dnc": false}, "tags": []any{}, "notes": []any{}, "source": "rex_import", "rex": map[string]any{"name_first": "Contact", "extra": strings.Repeat("source only ", 30)}}
		b, _ := json.Marshal(data)
		if _, err = cs.Exec(id, owner, string(b)); err != nil {
			t.Fatal(err)
		}
	}
	_ = cs.Close()
	ps, err := tx.Prepare(`INSERT INTO properties(id,owner_id,no,street,suburb,data,registered,created_at,updated_at) VALUES(?,?,?,'Example Street','Sydney',?,1,'now','now')`)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1700; i++ {
		id := fmt.Sprintf("rex-p-%05d", i)
		data := map[string]any{"id": id, "no": fmt.Sprint(i), "street": "Example Street", "suburb": "Sydney", "state": "NSW", "postcode": "2000", "ll": []float64{-33.86, 151.21}, "flags": map[string]any{"dnc": false}, "ownerIds": []string{}, "source": "rex_import", "rex": map[string]any{"adr_street_name": "Example Street", "extra": strings.Repeat("source only ", 40)}}
		b, _ := json.Marshal(data)
		if _, err = ps.Exec(id, owner, fmt.Sprint(i), string(b)); err != nil {
			t.Fatal(err)
		}
	}
	_ = ps.Close()
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	pull := func(gzip bool) ([]byte, time.Duration) {
		t.Helper()
		req := httptest.NewRequest("GET", "/api/v1/sync", nil)
		req.Header.Set("Authorization", "Bearer "+tokens["mary"])
		if gzip {
			req.Header.Set("Accept-Encoding", "gzip")
		}
		w := httptest.NewRecorder()
		start := time.Now()
		a.routes().ServeHTTP(w, req)
		elapsed := time.Since(start)
		if w.Code != 200 {
			t.Fatalf("sync %d: %s", w.Code, w.Body.String())
		}
		if gzip && w.Header().Get("Content-Encoding") != "gzip" {
			t.Fatal("gzip missing")
		}
		return w.Body.Bytes(), elapsed
	}
	plain, plainTime := pull(false)
	compressed, gzipTime := pull(true)
	decoded := uncompress(t, compressed)
	if !bytes.Equal(plain, decoded) {
		t.Fatal("gzip changed sync JSON")
	}
	if bytes.Contains(plain, []byte("source only")) {
		t.Fatal("Rex source rows leaked")
	}
	var m map[string]any
	if err = json.Unmarshal(decoded, &m); err != nil {
		t.Fatal(err)
	}
	if len(m["contacts"].([]any)) != 4000 || len(m["properties"].([]any)) != 1700 {
		t.Fatalf("sync counts: %d %d", len(m["contacts"].([]any)), len(m["properties"].([]any)))
	}
	t.Logf("sync first pull 4000 contacts + 1700 properties: plain=%d bytes (%s), gzip=%d bytes (%s), ratio=%.2f%%", len(plain), plainTime, len(compressed), gzipTime, 100*float64(len(compressed))/float64(len(plain)))
}
