package main

import (
	"strings"
	"testing"
)

func TestAdminWorkPermissionsSyncAndConflicts(t *testing.T) {
	a, tokens := setup(t)
	admin, john, mary := tokens["admi"], tokens["john"], tokens["mary"]
	var johnID string
	if err := a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&johnID); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"todos", "captures", "listings", "ofi_attendees", "contacts", "market-review"} {
		if w := request(a, "GET", "/api/v1/admin/"+path, john, nil); w.Code != 403 {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if w := request(a, "POST", "/api/v1/admin/todos", john, map[string]any{"assigneeId": johnID, "t": "no"}); w.Code != 403 {
		t.Fatalf("agent write: %d", w.Code)
	}
	cursor := obj(t, request(a, "GET", "/api/v1/sync", john, nil))["cursor"].(string)
	w := request(a, "POST", "/api/v1/admin/todos", admin, map[string]any{"id": "admin-task", "assigneeId": johnID, "d": "2027-08-01", "t": "Call the owner", "kind": "call", "propId": "p1"})
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	item := obj(t, w)
	if item["createdBy"] == nil || item["who"] != "John" || item["by"] != "Admin" {
		t.Fatalf("todo payload: %v", item)
	}
	var changes, audits, outbox int
	_ = a.db.QueryRow(`SELECT count(*) FROM sync_changes WHERE kind='todos' AND id='admin-task'`).Scan(&changes)
	_ = a.db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity='todos' AND entity_id='admin-task'`).Scan(&audits)
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity='todos' AND entity_id='admin-task'`).Scan(&outbox)
	if changes == 0 || audits != 1 || outbox != 1 {
		t.Fatalf("side effects: %d %d %d", changes, audits, outbox)
	}
	if !strings.Contains(request(a, "GET", "/api/v1/sync?since="+cursor, john, nil).Body.String(), "admin-task") {
		t.Fatal("assignee incremental sync missing todo")
	}
	if strings.Contains(request(a, "GET", "/api/v1/sync", mary, nil).Body.String(), "admin-task") {
		t.Fatal("unrelated agent saw todo")
	}
	w = request(a, "PATCH", "/api/v1/admin/todos/admin-task", admin, map[string]any{"version": 1, "done": true})
	if w.Code != 200 {
		t.Fatalf("patch: %d %s", w.Code, w.Body)
	}
	if w = request(a, "PATCH", "/api/v1/admin/todos/admin-task", admin, map[string]any{"version": 1, "d": "2027-08-02"}); w.Code != 409 {
		t.Fatalf("stale version: %d %s", w.Code, w.Body)
	}
}

func TestAdminOFIFollowVersionAndSync(t *testing.T) {
	a, tokens := setup(t)
	admin, john := tokens["admi"], tokens["john"]
	var johnID string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&johnID)
	listing := request(a, "POST", "/api/v1/listings", john, map[string]any{"id": "ofi-listing", "propertyId": "p1", "agent": "John", "ofi": []any{map[string]any{"d": "2027-01-02", "time": "10:00"}}})
	if listing.Code != 201 {
		t.Fatalf("listing: %s", listing.Body)
	}
	attendee := request(a, "POST", "/api/v1/ofi_attendees", john, map[string]any{"id": "ofi-visitor", "listingId": "ofi-listing", "eventIndex": 0, "contactId": "guest", "req": map[string]any{"bp": true}, "ps": true})
	if attendee.Code != 201 {
		t.Fatalf("attendee: %s", attendee.Body)
	}
	cursor := obj(t, request(a, "GET", "/api/v1/sync", john, nil))["cursor"].(string)
	w := request(a, "PATCH", "/api/v1/admin/ofi_attendees/ofi-visitor", admin, map[string]any{"version": 1, "follow": true})
	if w.Code != 200 {
		t.Fatalf("follow: %d %s", w.Code, w.Body)
	}
	if w = request(a, "PATCH", "/api/v1/admin/ofi_attendees/ofi-visitor", admin, map[string]any{"version": 1, "follow": false}); w.Code != 409 {
		t.Fatalf("stale attendee: %d", w.Code)
	}
	var n int
	_ = a.db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity='ofi_attendees' AND entity_id='ofi-visitor' AND action='PATCH'`).Scan(&n)
	if n != 1 {
		t.Fatalf("attendee audit: %d", n)
	}
	sync := request(a, "GET", "/api/v1/sync?since="+cursor, john, nil).Body.String()
	if !strings.Contains(sync, `"follow":true`) {
		t.Fatalf("attendee not synced: %s", sync)
	}
}

func TestMarketReviewActions(t *testing.T) {
	a, tokens := setup(t)
	admin := tokens["admi"]
	if !reviewLotMatch("L1-14 SP135645", "L12 SP135645") || reviewLotMatch("L1-14 SP135645", "L15 SP135645") {
		t.Fatal("range match")
	}
	if w := request(a, "POST", "/api/v1/properties", admin, map[string]any{"id": "parcel", "no": "12", "street": "Lister St", "suburb": "Sunnybank", "lotplan": "L12 SP135645"}); w.Code != 201 {
		t.Fatal(w.Body)
	}
	_, err := a.db.Exec(`INSERT INTO market_imports(id,source,provider,filename,sha256,created_at,status) VALUES ('import','file','fixture','fixture.csv','fixture-hash','2027-01-01','completed')`)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"event-link", "event-ignore"} {
		_, err = a.db.Exec(`INSERT INTO market_events(id,provider,provider_id,kind,address,address_key,lot_plan,suburb,price,event_date,import_id,raw) VALUES (?,?,?,'sale','12 Lister St','12 lister street','L1-14 SP135645','Sunnybank',1100000,'2027-01-01','import','{}')`, id, "fixture", id)
		if err != nil {
			t.Fatal(err)
		}
	}
	if w := request(a, "POST", "/api/v1/admin/market-review/event-link/link", tokens["john"], map[string]any{"propertyId": "parcel"}); w.Code != 403 {
		t.Fatalf("agent link: %d", w.Code)
	}
	w := request(a, "GET", "/api/v1/admin/market-review/event-link/candidates", admin, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "parcel") {
		t.Fatalf("candidates: %d %s", w.Code, w.Body)
	}
	if w = request(a, "POST", "/api/v1/admin/market-review/event-link/link", admin, map[string]any{"propertyId": "parcel"}); w.Code != 200 {
		t.Fatalf("link: %d %s", w.Code, w.Body)
	}
	if w = request(a, "POST", "/api/v1/admin/market-review/event-ignore/ignore", admin, map[string]any{}); w.Code != 200 {
		t.Fatalf("ignore: %d %s", w.Code, w.Body)
	}
	var linked, ignored, registered, audits int
	_ = a.db.QueryRow(`SELECT count(*) FROM market_events WHERE id='event-link' AND property_id='parcel' AND review_status='linked'`).Scan(&linked)
	_ = a.db.QueryRow(`SELECT count(*) FROM market_events WHERE id='event-ignore' AND review_status='ignored'`).Scan(&ignored)
	_ = a.db.QueryRow(`SELECT registered FROM properties WHERE id='parcel'`).Scan(&registered)
	_ = a.db.QueryRow(`SELECT count(*) FROM audit_log WHERE entity='market_events'`).Scan(&audits)
	if linked != 1 || ignored != 1 || registered != 1 || audits != 2 {
		t.Fatalf("effects: %d %d %d %d", linked, ignored, registered, audits)
	}
	if w = request(a, "POST", "/api/v1/admin/market-review/event-link/link", admin, map[string]any{"propertyId": "parcel"}); w.Code != 409 {
		t.Fatalf("double link: %d", w.Code)
	}
}
