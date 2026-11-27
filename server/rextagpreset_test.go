package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tagPresetWorker is a sandboxWorker whose fake Rex keeps an AdminTags list.
type tagPresetFake struct {
	presets              []map[string]any
	searchErr, createErr bool
	searches, creates    []string
	contactTags          []string
}

func tagPresetWorker(t *testing.T, a *app, f *tagPresetFake) *rexWorker {
	t.Helper()
	rexTagPresets.reset()
	t.Cleanup(rexTagPresets.reset)
	w, _ := sandboxWorker(t, a, func(path string, b map[string]any) (any, int) {
		switch path {
		case "/v1/rex/admin-tags/search":
			name := b["criteria"].([]any)[0].(map[string]any)["value"].(string)
			f.searches = append(f.searches, name)
			if f.searchErr {
				return rexTestError("permission denied"), 200
			}
			var rows []any
			for _, p := range f.presets {
				if strings.EqualFold(p["tag"].(string), name) {
					rows = append(rows, p)
				}
			}
			return map[string]any{"rows": rows, "total": len(rows)}, 200
		case "/v1/rex/admin-tags/create":
			d := b["data"].(map[string]any)
			f.creates = append(f.creates, d["tag"].(string))
			if d["object_type"] != "contact" || b["return_id"] != true {
				t.Errorf("create body %v", b)
			}
			if f.createErr {
				return rexTestError("not allowed"), 200
			}
			f.presets = append(f.presets, map[string]any{"id": 7, "tag": d["tag"], "object_type": "contact"})
			return 7, 200
		case "/v1/rex/contacts/update":
			for _, r := range b["data"].(map[string]any)["related"].(map[string]any)["contact_tags"].([]any) {
				f.contactTags = append(f.contactTags, r.(map[string]any)["tag"].(string))
			}
			return map[string]any{"id": 91}, 200
		case "/v1/rex/contacts/read":
			return map[string]any{"id": 91, "etag": "new"}, 200
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	})
	return w
}

func tagPresetStep(t *testing.T, a *app, w *rexWorker, id int, tag string) string {
	t.Helper()
	workerQueue(t, a, "rex_contact_tag", "c:"+tag, `{"contactId":"c","tag":"`+tag+`"}`)
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	var s string
	_ = a.db.QueryRow(`SELECT status||':'||COALESCE(last_error,'') FROM rex_outbox WHERE id=?`, id).Scan(&s)
	return s
}

func TestRexTagPresetCreatedBeforeContactWrite(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	f := &tagPresetFake{}
	w := tagPresetWorker(t, a, f)
	if s := tagPresetStep(t, a, w, 1, "Pool owner"); s != "sent:added to Rex tag list" {
		t.Fatal(s)
	}
	if strings.Join(f.searches, "|") != "Pool owner" || strings.Join(f.creates, "|") != "Pool owner" || strings.Join(f.contactTags, "|") != "Pool owner" {
		t.Fatal(f.searches, f.creates, f.contactTags)
	}
	// Cached: the same tag (any case) on another job is neither looked up nor created again.
	if s := tagPresetStep(t, a, w, 2, "POOL OWNER"); s != "sent:" {
		t.Fatal(s)
	}
	if len(f.searches) != 1 || len(f.creates) != 1 || len(f.contactTags) != 2 {
		t.Fatal(f.searches, f.creates, f.contactTags)
	}
}

func TestRexTagPresetExistingCaseInsensitive(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	f := &tagPresetFake{presets: []map[string]any{{"id": 3, "tag": "Hot Buyer", "object_type": "contact", "cached_count": 4}}}
	w := tagPresetWorker(t, a, f)
	if s := tagPresetStep(t, a, w, 1, "hot buyer"); s != "sent:" {
		t.Fatal(s)
	}
	if len(f.searches) != 1 || len(f.creates) != 0 || strings.Join(f.contactTags, "|") != "hot buyer" {
		t.Fatal(f.searches, f.creates, f.contactTags)
	}
	// A property tag of the same name is not the contact preset.
	f.presets = []map[string]any{{"id": 4, "tag": "Corner", "object_type": map[string]any{"id": "property"}}}
	if s := tagPresetStep(t, a, w, 2, "Corner"); s != "sent:added to Rex tag list" || strings.Join(f.creates, "|") != "Corner" {
		t.Fatal(s, f.creates)
	}
}

func TestRexTagPresetFailureDoesNotBlockContact(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	f := &tagPresetFake{searchErr: true}
	w := tagPresetWorker(t, a, f)
	s := tagPresetStep(t, a, w, 1, "VIP")
	if !strings.HasPrefix(s, "sent:Rex tag list lookup failed") || !strings.Contains(s, "permission denied") || strings.Join(f.contactTags, "|") != "VIP" || len(f.creates) != 0 {
		t.Fatal(s, f.contactTags, f.creates)
	}
	f.searchErr, f.createErr = false, true
	s = tagPresetStep(t, a, w, 2, "Investor")
	if !strings.HasPrefix(s, "sent:Rex tag list create failed") || strings.Join(f.contactTags, "|") != "VIP|Investor" {
		t.Fatal(s, f.contactTags)
	}
	// Failures are not cached: the next job for the same tag tries again.
	f.createErr = false
	if s = tagPresetStep(t, a, w, 3, "VIP"); s != "sent:added to Rex tag list" || len(f.searches) != 3 {
		t.Fatal(s, f.searches)
	}
}

func TestRexTagPresetDisabled(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	f := &tagPresetFake{}
	w := tagPresetWorker(t, a, f)
	t.Setenv("REX_TAG_PRESET", "0")
	if s := tagPresetStep(t, a, w, 1, "Pool"); s != "sent:" {
		t.Fatal(s)
	}
	if len(f.searches) != 0 || len(f.creates) != 0 || strings.Join(f.contactTags, "|") != "Pool" {
		t.Fatal(f.searches, f.creates, f.contactTags)
	}
}

func TestRexTagPresetNotForRemoval(t *testing.T) {
	a := workerDB(t)
	linkedContact(t, a)
	f := &tagPresetFake{}
	w := tagPresetWorker(t, a, f)
	if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('rex_contact_tag','c:Pool','DELETE','{"contactId":"c","tag":"Pool"}','now')`); err != nil {
		t.Fatal(err)
	}
	if err := w.step(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(f.searches) != 0 || len(f.creates) != 0 || len(f.contactTags) != 0 {
		t.Fatal(f.searches, f.creates, f.contactTags)
	}
}

// rex-probe TP is read-only: it lists, describes and looks up AdminTags, never creates.
func TestRexProbeTagPresetReadOnly(t *testing.T) {
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		calls = append(calls, r.URL.Path)
		var result any = map[string]any{}
		switch r.URL.Path {
		case "/v1/rex/authentication/login":
			result = "token"
		case "/v1/rex/admin-tags/search":
			rows := []any{map[string]any{"id": 1, "tag": "Hot Buyer", "object_type": "contact", "cached_count": 2}}
			if c, ok := b["criteria"].([]any); ok && !strings.EqualFold(c[0].(map[string]any)["value"].(string), "hot buyer") {
				rows = nil
			}
			result = map[string]any{"rows": rows, "total": len(rows)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	dir := setupProbe(t, srv)
	defer os.RemoveAll(dir)
	if err := rexProbe([]string{"--account", "2057", "--only", "TP", "--out", dir}); err != nil {
		t.Fatal(err)
	}
	searches := 0
	for _, p := range calls {
		if strings.Contains(p, "/create") {
			t.Fatal("TP wrote:", calls)
		}
		if p == "/v1/rex/admin-tags/search" {
			searches++
		}
	}
	report, _ := os.ReadFile(filepath.Join(dir, "report.md"))
	if searches != 2 || !strings.Contains(string(report), "| TP | PASS |") || !strings.Contains(string(report), "大小写不敏感）：true") {
		t.Fatal(calls, string(report))
	}
}
