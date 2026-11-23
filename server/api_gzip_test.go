package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func uncompress(t *testing.T, data []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func TestAPIGzipNegotiation(t *testing.T) {
	a, tokens := setup(t)
	var owner string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&owner)
	raw, _ := json.Marshal(map[string]any{"id": "big-c", "first": "Demo", "notes": []any{}, "source": "rex_import", "rex": map[string]any{"private": strings.Repeat("unneeded", 600)}, "description": strings.Repeat("same words ", 250)})
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES(?,?,?,'now','now')`, "big-c", owner, string(raw)); err != nil {
		t.Fatal(err)
	}
	get := func(url, token, encoding string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", url, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		if encoding != "" {
			r.Header.Set("Accept-Encoding", encoding)
		}
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, r)
		return w
	}
	plain := get("/api/v1/sync", tokens["mary"], "")
	zipped := get("/api/v1/sync", tokens["mary"], "br, gzip;q=1")
	if plain.Code != 200 || zipped.Code != 200 || zipped.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(zipped.Header().Get("Vary"), "Accept-Encoding") {
		t.Fatal("gzip headers", plain.Code, zipped.Code, zipped.Header())
	}
	if !bytes.Equal(plain.Body.Bytes(), uncompress(t, zipped.Body.Bytes())) {
		t.Fatal("gzip JSON differs")
	}
	if bytes.Contains(plain.Body.Bytes(), []byte("unneeded")) {
		t.Fatal("raw Rex leaked into sync")
	}
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("test-prop-%d", i)
		if _, err := a.db.Exec(`INSERT INTO properties(id,no,street,suburb,data,created_at,updated_at) VALUES(?,'3','Long Street Name','Brisbane',?,'now','now')`, id, fmt.Sprintf(`{"id":%q,"lotplan":%q}`, id, strings.Repeat("test", 40))); err != nil {
			t.Fatal(err)
		}
	}
	adminPlain := get("/api/v1/admin/properties?limit=10", tokens["admi"], "")
	admin := get("/api/v1/admin/properties?limit=10", tokens["admi"], "gzip")
	if admin.Code != 200 || admin.Header().Get("Content-Encoding") != "gzip" || !strings.Contains(admin.Header().Get("Vary"), "Accept-Encoding") || !bytes.Equal(adminPlain.Body.Bytes(), uncompress(t, admin.Body.Bytes())) {
		t.Fatal("admin API gzip", admin.Code, admin.Header())
	}
	small := get("/api/v1/health", "", "gzip")
	if small.Header().Get("Content-Encoding") != "" {
		t.Fatal("tiny JSON compressed")
	}
	disabled := get("/api/v1/sync", tokens["mary"], "gzip;q=0")
	if disabled.Header().Get("Content-Encoding") != "" {
		t.Fatal("q=0 compressed")
	}
	static := get("/admin/", "", "gzip")
	if static.Header().Get("Content-Encoding") != "" {
		t.Fatal("static asset recompressed")
	}
	// Streaming and non-JSON responses are forwarded immediately.
	stream := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	r.Header.Set("Accept-Encoding", "gzip")
	gzipAPI(stream, r, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: test\n\n"))
		w.(http.Flusher).Flush()
	}))
	if stream.Header().Get("Content-Encoding") != "" || stream.Body.String() != "data: test\n\n" {
		t.Fatal("SSE compressed or buffered")
	}
}
