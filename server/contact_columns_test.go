package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPhoneSyncDNCExport(t *testing.T) {
	a, tok := setup(t)
	push := request(a, "POST", "/api/v1/sync/push", tok["mary"], map[string]any{"ops": []any{map[string]any{"opId": "dnc-op", "kind": "contacts", "op": "upsert", "id": "phone-dnc", "data": map[string]any{"first": "Phone", "last": "DNC", "flags": map[string]any{"dnc": true}, "roles": []string{"owner"}, "tags": []string{"private"}, "consent": map[string]any{"phone": true}}}}})
	if push.Code != 200 || !strings.Contains(push.Body.String(), `"status":"ok"`) {
		t.Fatalf("push: %d %s", push.Code, push.Body)
	}
	var flags, roles, tags, consent string
	if err := a.db.QueryRow(`SELECT flags,roles,tags,consent FROM contacts WHERE id='phone-dnc'`).Scan(&flags, &roles, &tags, &consent); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(flags, `"dnc":true`) || !strings.Contains(roles, "owner") || !strings.Contains(tags, "private") || !strings.Contains(consent, "phone") {
		t.Fatalf("columns: %s %s %s %s", flags, roles, tags, consent)
	}
	property := request(a, "POST", "/api/v1/properties", tok["admi"], map[string]any{"id": "phone-dnc-property", "no": "42", "street": "Phone DNC Street", "suburb": "SyncTest", "ownerIds": []string{"phone-dnc"}})
	if property.Code != 201 {
		t.Fatalf("property: %d %s", property.Code, property.Body)
	}
	export := request(a, "POST", "/api/v1/admin/export", tok["admi"], map[string]any{"fields": []string{"address"}, "suburb": "SyncTest", "reason": "Privacy test", "acknowledge": true, "excludeDNC": true})
	if export.Code != 200 || strings.Contains(export.Body.String(), "Phone DNC Street") {
		t.Fatalf("export leaked: %d %s", export.Code, export.Body)
	}
}

func TestContactColumnsMigrationBackfill(t *testing.T) {
	a, tok := setup(t)
	_ = tok
	var owner string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,data,created_at,updated_at) VALUES('legacy-dnc',?, '{"flags":{"dnc":true},"roles":["owner"],"tags":["private"],"consent":{"phone":true}}',?,?)`, owner, now(), now()); err != nil {
		t.Fatal(err)
	}
	if _, err := a.db.Exec(`DELETE FROM schema_migrations WHERE version='011_contact_columns.sql'`); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "migrating.db")
	if err := migrate(a.db, path); err != nil {
		t.Fatal(err)
	}
	var flags, roles, tags, consent string
	if err := a.db.QueryRow(`SELECT flags,roles,tags,consent FROM contacts WHERE id='legacy-dnc'`).Scan(&flags, &roles, &tags, &consent); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(flags, `"dnc":true`) || !strings.Contains(roles, "owner") || !strings.Contains(tags, "private") || !strings.Contains(consent, "phone") {
		t.Fatalf("backfill: %s %s %s %s", flags, roles, tags, consent)
	}
}
