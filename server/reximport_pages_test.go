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

func TestIstringNumericRexID(t *testing.T) {
	if got := istring(float64(28734913)); got != "28734913" {
		t.Fatalf("istring(float) = %q", got)
	}
	if got := istring("28734913"); got != "28734913" {
		t.Fatalf("istring(string) = %q", got)
	}
	if got := istring(1.5); got != "1.5" {
		t.Fatalf("istring(1.5) = %q", got)
	}
}

func TestFlattenRexRead(t *testing.T) {
	var r importRow
	_ = json.Unmarshal([]byte(`{"id":28734913,"is_dnd":null,"related":{"contact_names":[{"id":1,"name_first":"cran18787","name_last":"berrie1"}],"contact_phones":[{"id":2,"phone_number":"111","phone_primary":false},{"id":3,"phone_number":"3924098231","phone_primary":true}],"contact_emails":[{"id":4,"email_address":"c@x.com","email_primary":true}]}}`), &r)
	flattenRexRead(r)
	if iv(r, "phone_number") != "3924098231" || iv(r, "email_address") != "c@x.com" || iv(r, "name_first") != "cran18787" || iv(r, "id") != "28734913" {
		t.Fatalf("flatten: %v", r)
	}
}

func TestImportSaveKeepsAppFields(t *testing.T) {
	a, _, _ := resetFixture(t)
	db := a.db
	var mary string
	if err := db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary); err != nil {
		t.Fatal(err)
	}
	row := importRow{"id": "501", "etag": "501-1", "name_first": "A", "name_last": "B", "phone_number": "1"}
	if err := importSave(db, "contacts", "2057", row, mary, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	id := "rex-contacts-2057-501"
	if _, err := db.Exec(`UPDATE contacts SET data=json_set(data,'$.notes',json('[{"t":"keep"}]'),'$.tags',json('["DK - x"]'),'$.flags.optOut',1) WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	row = importRow{"id": float64(501), "etag": "501-2", "name_first": "A2", "name_last": "B", "phone_number": "2", "is_dnd": true}
	if err := importSave(db, "contacts", "2057", row, mary, false, &importStats{}); err != nil {
		t.Fatal(err)
	}
	var notes, tags, mobile, first string
	var dnc, opt int
	if err := db.QueryRow(`SELECT json_extract(data,'$.notes[0].t'),json_extract(data,'$.tags[0]'),mobile,first,json_extract(data,'$.flags.dnc'),json_extract(data,'$.flags.optOut') FROM contacts WHERE id=?`, id).Scan(&notes, &tags, &mobile, &first, &dnc, &opt); err != nil {
		t.Fatal(err)
	}
	if notes != "keep" || tags != "DK - x" || mobile != "2" || first != "A2" || dnc != 1 || opt != 1 {
		t.Fatalf("got notes=%q tags=%q mobile=%q first=%q dnc=%d opt=%d", notes, tags, mobile, first, dnc, opt)
	}
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM contacts WHERE id LIKE 'rex-contacts-2057-%'`).Scan(&n)
	if n != 1 {
		t.Fatalf("duplicates: %d", n)
	}
}
