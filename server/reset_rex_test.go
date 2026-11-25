package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResetClearsRexImportAndPreservesRequestLog(t *testing.T) {
	a, path, _ := resetFixture(t)
	var mary string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	s := &importStats{}
	prop := importRow{"id": "95001", "etag": "v1", "adr_street_number": "4", "adr_street_name": "Elsewhere", "adr_suburb_or_town": "Sydney"}
	if err := importSave(a.db, "properties", "2057", prop, mary, false, s); err != nil {
		t.Fatal(err)
	}
	if err := importSave(a.db, "contacts", "2057", importRow{"id": "95002", "etag": "v1", "name_first": "Imported"}, mary, false, s); err != nil {
		t.Fatal(err)
	}
	// Rex properties may contain lotplan; they are not part of the demo/DCDB seed.
	for _, q := range []string{
		`UPDATE properties SET data=json_set(data,'$.lotplan','X1') WHERE id='rex-properties-2057-95001'`,
		`UPDATE properties SET data=json_set(data,'$.source','rex_import'),no='999' WHERE id='p1'`,
		`INSERT INTO property_owners(property_id,contact_id) VALUES('rex-properties-2057-95001','rex-contacts-2057-95002')`,
		`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts','rex-contacts-2057-95002','PATCH','{}','now')`,
	} {
		if _, err := a.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	rexDBLogger(a.db)(rexLog{At: now(), AccountID: "2057", Direction: "out", Kind: "read", Service: "Contacts", Method: "search", Path: "/v1/rex/contacts/search", Caller: "import", Attempt: 1, RequestJSON: "{}", HTTPStatus: 200, OK: true, ResultSummary: "rows=1"})
	var before int
	if err := a.db.QueryRow(`SELECT count(*) FROM rex_request_log`).Scan(&before); err != nil || before < 1 {
		t.Fatal(before, err)
	}
	raw, err := assets.ReadFile("seed/demo.json")
	if err != nil {
		t.Fatal(err)
	}
	preview, err := resetCounts(a.db, string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if preview["properties: rex_import (deleted unless demo)"] != 1 || preview["contacts: rex_import (cleared)"] != 1 || preview["rex_links: rex_import (cleared)"] != 2 || preview["rex_request_log"] != int64(before) {
		t.Fatal(preview)
	}
	if err := resetCommand(a.db, path, nil); err != nil {
		t.Fatal(err)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE id='rex-properties-2057-95001'`).Scan(&n)
	if n != 1 {
		t.Fatal("preview deleted Rex property")
	}
	if err := resetCommand(a.db, path, []string{"--yes", "--with-demo"}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`SELECT count(*) FROM contacts WHERE json_extract(data,'$.source')='rex_import'`,
		`SELECT count(*) FROM properties WHERE json_extract(data,'$.source')='rex_import'`,
		`SELECT count(*) FROM properties WHERE id='rex-properties-2057-95001'`,
		`SELECT count(*) FROM rex_links`, `SELECT count(*) FROM rex_outbox`,
	} {
		if err := a.db.QueryRow(q).Scan(&n); err != nil || n != 0 {
			t.Fatalf("%s = %d: %v", q, n, err)
		}
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_request_log`).Scan(&n)
	if n != before {
		t.Fatal("request log cleared", n, before)
	}
	var source, no string
	if err := a.db.QueryRow(`SELECT COALESCE(json_extract(data,'$.source'),''),no FROM properties WHERE id='p1'`).Scan(&source, &no); err != nil || source == "rex_import" || no == "999" {
		t.Fatal("demo property not restored", source, no, err)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM property_owners`).Scan(&n)
	if n != 5 {
		t.Fatal("demo relations not restored", n)
	}
	_ = a.db.QueryRow(`SELECT count(*) FROM pragma_foreign_key_check`).Scan(&n)
	if n != 0 {
		t.Fatal("orphan FKs", n)
	}
}

func TestResetRexImportGateAndSuccess(t *testing.T) {
	for _, readEnabled := range []bool{false, true} {
		name := "disabled"
		if readEnabled {
			name = "enabled"
		}
		t.Run(name, func(t *testing.T) {
			a, path, _ := resetFixture(t)
			if err := createUser(a.db, "import@axleo.local", "Import", "agent", "password123"); err != nil {
				t.Fatal(err)
			}
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/v1/rex/authentication/login" {
					_, _ = w.Write([]byte(`{"result":"token","error":null}`))
					return
				}
				if r.URL.Path == "/v1/rex/AccountSettings/read" {
					_, _ = w.Write([]byte(`{"result":{"id":2057,"name":"Sandbox","is_test_account":true},"error":null}`))
					return
				}
				var b map[string]any
				_ = json.NewDecoder(r.Body).Decode(&b)
				if b["extra_options"] == nil && r.URL.Path == "/v1/rex/properties/search" {
					t.Error("missing relation fields")
				}
				if r.URL.Path != "/v1/rex/properties/search" && r.URL.Path != "/v1/rex/contacts/search" {
					t.Errorf("unexpected request: %s", r.URL.Path)
				}
				id := "12"
				if strings.Contains(r.URL.Path, "contacts") {
					id = "13"
				}
				row := map[string]any{"id": id, "etag": "1", "name_first": "Tester", "adr_street_number": "3", "adr_street_name": "Test", "adr_suburb_or_town": "Sydney"}
				_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"rows": []any{row}, "total": 1}, "error": nil})
			}))
			defer srv.Close()
			t.Setenv("REX_BASE_URL", srv.URL)
			t.Setenv("REX_APP_ID", "Integration:Test")
			t.Setenv("REX_EMAIL", "tester@example.com")
			t.Setenv("REX_PASSWORD", "secret")
			t.Setenv("REX_ACCOUNT_ID", "2057")
			t.Setenv("REX_INTERVAL_MS", "0")
			if readEnabled {
				t.Setenv("AXLEO_REX_READ", "1")
			} else {
				t.Setenv("AXLEO_REX_READ", "0")
			}
			if _, err := a.db.Exec(`INSERT INTO rex_outbox(entity,entity_id,op,payload,created_at) VALUES('contacts','test','POST','{}','now')`); err != nil {
				t.Fatal(err)
			}
			err := resetCommand(a.db, path, []string{"--yes", "--rex-import", "--with-demo"})
			if readEnabled && err != nil {
				t.Fatal(err)
			}
			if !readEnabled && (err == nil || calls != 0) {
				t.Fatal("disabled import was called", err, calls)
			}
			var n int
			_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox`).Scan(&n)
			if n != 0 {
				t.Fatal("reset did not finish", n)
			}
			_ = a.db.QueryRow(`SELECT count(*) FROM properties WHERE id='p1'`).Scan(&n)
			if n != 1 {
				t.Fatal("demo removed")
			}
			_ = a.db.QueryRow(`SELECT count(*) FROM rex_links WHERE account_id='2057'`).Scan(&n)
			if readEnabled && (n != 2 || calls != 4) {
				t.Fatalf("import failed: links=%d calls=%d", n, calls)
			}
			if !readEnabled && n != 0 {
				t.Fatalf("read disabled but %d links", n)
			}
		})
	}
}
