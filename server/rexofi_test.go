package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRexEventRealSearchRow(t *testing.T) {
	// ListingEvents::search row from the test account; agent email is synthetic.
	const fixture = `{"account_id":"2057","etag":"14871972-2026-09-14 05:12:21","event_agent":null,"event_date":"2026-09-26","event_duration_minutes":"30","event_time":"09:45:00","event_timezone":"Australia/Perth","event_type":"open_home","event_type_display":"Open Home","event_venue":null,"id":"14871972","listing":{"id":5133946,"listing_agent_1":{"email":"agent@example.invalid"},"listing_category":{"id":"residential_rental"}}}`
	var row importRow
	if err := json.Unmarshal([]byte(fixture), &row); err != nil {
		t.Fatal(err)
	}
	// Brisbane has reached Sep 26, but Perth is still on Sep 25.
	at := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	if _, ok := rexEvent(row, at, 0, 0); ok {
		t.Fatal("Perth event should not count as today yet")
	}
	at = at.Add(time.Hour)
	ev, ok := rexEvent(row, at, 0, 0)
	if !ok || ev["d"] != "2026-09-26" || ev["time"] != "09:45–10:15" || rexOFIRef(row["listing"]) != "5133946" {
		t.Fatalf("real search row: %v, accepted=%v", ev, ok)
	}
	row["event_end_time"] = "10:25:00"
	if ev, ok = rexEvent(row, at, 0, 0); !ok || ev["time"] != "09:45–10:25" {
		t.Fatalf("explicit end should win: %v", ev)
	}
	delete(row, "event_end_time")
	row["end_time"] = "10:20"
	if ev, ok = rexEvent(row, at, 0, 0); !ok || ev["time"] != "09:45–10:20" {
		t.Fatalf("legacy end should win: %v", ev)
	}
	delete(row, "end_time")
	row["event_duration_minutes"] = "invalid"
	if ev, ok = rexEvent(row, at, 0, 0); !ok || ev["time"] != "09:45–09:45" {
		t.Fatalf("invalid duration: %v", ev)
	}
	for _, typ := range []string{"open_inspection", "not_open_home", "open", "reopen_home"} {
		row["event_type"] = typ
		if _, ok := rexEvent(row, at, 0, 0); ok {
			t.Fatalf("accepted %s", typ)
		}
	}
	row["event_type"] = "ofi"
	if _, ok := rexEvent(row, at, 0, 0); !ok {
		t.Fatal("legacy ofi rejected")
	}
}

func TestRexOFIPoll(t *testing.T) {
	a, _ := setup(t)
	day := time.Date(2026, 11, 20, 14, 5, 0, 0, time.UTC) // Brisbane: Nov 21 00:05
	t.Setenv("REX_BASE_URL", "mock")
	t.Setenv("REX_APP_ID", "test")
	t.Setenv("REX_EMAIL", "test")
	t.Setenv("REX_PASSWORD", "test")
	t.Setenv("REX_ACCOUNT_ID", "2057")
	calls := 0
	event := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var result any
		switch r.URL.Path {
		case "/v1/rex/listings/search":
			result = map[string]any{"rows": []any{map[string]any{"id": "l1", "property": map[string]any{"id": "p1"}}}}
		case "/v1/rex/listing-events/search":
			rows := []any{}
			if event {
				rows = append(rows, map[string]any{"id": "e1", "listing": map[string]any{"id": "l1"}, "event_type": "open_home", "event_date": "2026-11-21", "event_time": "10:00:00", "event_end_time": "10:30:00", "attendee_count": 3}, map[string]any{"id": "e2", "listing": map[string]any{"id": "l1"}, "event_type": "open_home", "event_date": "2026-11-20T15:00:00Z", "event_time": "11:00", "event_end_time": "11:30"}, map[string]any{"id": "old", "listing": map[string]any{"id": "l1"}, "event_type": "open_home", "event_date": "2026-11-19", "event_time": "12:00"})
			}
			result = map[string]any{"rows": rows}
		case "/v1/rex/properties/read":
			result = map[string]any{"id": "p1", "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank"}
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Interval = 0
	c.ReadOnly = true
	t.Setenv("AXLEO_REX_READ", "0")
	if err := pollRexOFI(context.Background(), a.db, c, "2057", day); err != nil || calls != 0 {
		t.Fatalf("disabled: %v %d", err, calls)
	}
	t.Setenv("AXLEO_REX_READ", "1")
	for i := 0; i < 2; i++ {
		if err := pollRexOFI(context.Background(), a.db, c, "2057", day.Add(time.Duration(i)*6*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	var raw string
	var version int
	if err := a.db.QueryRow(`SELECT data,version FROM listings WHERE id='rex-listings-2057-l1'`).Scan(&raw, &version); err != nil {
		t.Fatal(err)
	}
	var data struct {
		PropertyID string `json:"propertyId"`
		OFI        []struct {
			D     string `json:"d"`
			Count int    `json:"rexAttendeeCount"`
		} `json:"ofi"`
	}
	if err := json.Unmarshal([]byte(raw), &data); err != nil || data.PropertyID != "rex-properties-2057-p1" || len(data.OFI) != 2 || data.OFI[0].D != "2026-11-21" || data.OFI[0].Count != 3 || version != 1 {
		t.Fatalf("replay: %s version=%d error=%v", raw, version, err)
	}
	event = false
	if err := pollRexOFI(context.Background(), a.db, c, "2057", day.Add(12*time.Minute)); err != nil {
		t.Fatal(err)
	}
	_ = a.db.QueryRow(`SELECT data FROM listings WHERE id='rex-listings-2057-l1'`).Scan(&raw)
	_ = json.Unmarshal([]byte(raw), &data)
	if len(data.OFI) != 0 {
		t.Fatalf("removed events: %s", raw)
	}
}

func TestRexListingStatus(t *testing.T) {
	for _, c := range []struct {
		row  importRow
		want string
	}{
		{importRow{"system_listing_state": "current"}, "listed"},
		{importRow{"system_listing_state": map[string]any{"id": "current", "text": "Current"}}, "listed"},
		{importRow{"system_listing_state": "Sold"}, "sold"},
		{importRow{"system_listing_state": "leased"}, "leased"},
		{importRow{"system_listing_state": "withdrawn"}, "withdrawn"},
		{importRow{"system_listing_state": "current", "system_record_state": "archived"}, "archived"},
		{importRow{"listing_status": "current"}, "listed"},
		{importRow{}, "unknown"},
		{importRow{"system_listing_state": "mystery"}, "unknown"},
	} {
		if got, _ := rexListingStatus(c.row); got != c.want {
			t.Errorf("%v: %s, want %s", c.row, got, c.want)
		}
	}
}

// Rex listing state, listing agents and each open home's own agent reach the
// listing record; a shared Rex user ("Marketing Team") keeps its Rex name and is
// never mapped to the default owner or the property owner.
func TestRexOFIAgentsStatusWindow(t *testing.T) {
	a, tokens := setup(t)
	day := time.Date(2026, 11, 20, 1, 0, 0, 0, time.UTC) // Brisbane: Nov 20 11:00
	for k, v := range map[string]string{"REX_BASE_URL": "mock", "REX_APP_ID": "test", "REX_EMAIL": "test", "REX_PASSWORD": "test", "REX_ACCOUNT_ID": "2057", "AXLEO_REX_READ": "1", "REX_DEFAULT_OWNER_EMAIL": "admin@axleo.local"} {
		t.Setenv(k, v)
	}
	var mary, john string
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='mary@axleo.local'`).Scan(&mary)
	_ = a.db.QueryRow(`SELECT id FROM users WHERE email='john@axleo.local'`).Scan(&john)
	if _, err := a.db.Exec(`UPDATE users SET rex_user_id='u-john' WHERE id=?`, john); err != nil {
		t.Fatal(err)
	}
	marketing := map[string]any{"id": "u-mkt", "name": "Marketing Team"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any
		switch r.URL.Path {
		case "/v1/rex/listings/search":
			result = map[string]any{"rows": []any{
				map[string]any{"id": "l1", "property": map[string]any{"id": "p1"}, "system_listing_state": "current", "listing_category": map[string]any{"id": "residential_sale"}, "listing_agent_1": map[string]any{"id": "u-mary", "name": "Mary Rex", "email": "MARY@axleo.local"}, "listing_agent_2": map[string]any{"id": "u-john", "name": "John Rex"}},
				map[string]any{"id": "l2", "property": map[string]any{"id": "p1"}, "system_listing_state": "sold", "listing_agent_1": marketing},
			}}
		case "/v1/rex/listing-events/search":
			ev := func(id, date string, agent any) map[string]any {
				return map[string]any{"id": id, "listing": map[string]any{"id": "l1"}, "event_type": "open_home", "event_date": date, "event_time": "10:00:00", "event_duration_minutes": "30", "event_agent": agent}
			}
			result = map[string]any{"rows": []any{ev("e1", "2026-11-22", marketing), ev("e2", "2026-11-23", map[string]any{"id": "u-john", "name": "John Rex"}), ev("e3", "2026-11-24", nil), ev("e4", "2026-12-18", marketing), ev("e5", "2027-03-30", marketing)}}
		case "/v1/rex/properties/read":
			result = map[string]any{"id": "p1", "adr_street_number": "16", "adr_street_name": "Gager St", "adr_suburb_or_town": "Sunnybank", "attr_bedrooms": "3", "attr_bathrooms": "2", "attr_garages": "2"}
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	c := newRexClient(srv.URL, "test")
	c.Interval = 0
	c.ReadOnly = true
	if err := pollRexOFI(context.Background(), a.db, c, "2057", day); err != nil {
		t.Fatal(err)
	}
	read := func(id string) map[string]any {
		t.Helper()
		var raw string
		if err := a.db.QueryRow(`SELECT data FROM listings WHERE id=?`, id).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = json.Unmarshal([]byte(raw), &m)
		return m
	}
	l1, l2 := read("rex-listings-2057-l1"), read("rex-listings-2057-l2")
	if l1["status"] != "listed" || l1["rexState"] != "current" || l1["category"] != "residential_sale" || l1["agent"] != "Mary Rex" || ijson(l1["agentIds"]) != ijson([]string{mary, john}) {
		t.Fatalf("current listing: %v", l1)
	}
	if l2["status"] != "sold" || l2["agent"] != "Marketing Team" || ijson(l2["agentIds"]) != "[]" {
		t.Fatalf("sold listing: %v", l2)
	}
	events := l1["ofi"].([]any)
	if len(events) != 4 {
		t.Fatalf("default window should keep Dec 18 and drop Mar 30: %v", events)
	}
	want := []struct{ agent, id, from string }{{"Marketing Team", "", ""}, {"John Rex", john, ""}, {"Mary Rex", mary, "listing"}, {"Marketing Team", "", ""}}
	for i, w := range want {
		e := events[i].(map[string]any)
		if e["agent"] != w.agent || e["agentId"] != w.id || str(e, "agentFrom") != w.from {
			t.Fatalf("event %d: %v, want %v", i, e, w)
		}
	}
	// Every agent receives the listing (listings stay team-visible); attendees of
	// one agent's check-in are not sent to the open home's other agents.
	if _, err := a.db.Exec(`INSERT INTO ofi_attendees(id,owner_id,event_id,contact_id,data,created_at,updated_at) VALUES('att',?,'rex-listings-2057-l1','c1','{"id":"att","listingId":"rex-listings-2057-l1","eventIndex":1,"note":"private"}',?,?)`, mary, now(), now()); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"mary", "john"} {
		pull := obj(t, request(a, "GET", "/api/v1/sync", tokens[who], nil))
		found := false
		for _, v := range pull["listings"].([]any) {
			if v.(map[string]any)["id"] == "rex-listings-2057-l1" {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s did not receive the listing", who)
		}
		if n := len(pull["ofi_attendees"].([]any)); (who == "mary") != (n == 1) {
			t.Fatalf("%s attendees: %d", who, n)
		}
	}
	// A phone echo cannot replace Rex-owned agents or erase Rex property attributes.
	listing := obj(t, request(a, "GET", "/api/v1/listings/rex-listings-2057-l1", tokens["admi"], nil))
	w := request(a, "PATCH", "/api/v1/listings/rex-listings-2057-l1", tokens["admi"], map[string]any{"version": listing["version"], "agent": "Admin", "agentIds": []string{}, "price": "$1m"})
	if got := obj(t, w); w.Code != 200 || got["agent"] != "Mary Rex" || got["price"] != "$1m" || len(got["agentIds"].([]any)) != 2 {
		t.Fatalf("listing echo: %d %v", w.Code, got)
	}
	property := obj(t, request(a, "GET", "/api/v1/properties/rex-properties-2057-p1", tokens["admi"], nil))
	w = request(a, "PATCH", "/api/v1/properties/rex-properties-2057-p1", tokens["admi"], map[string]any{"version": property["version"], "beds": nil, "baths": nil, "cars": nil, "notes": "x"})
	if got := obj(t, w); w.Code != 200 || got["beds"] != "3" || got["baths"] != "2" || got["cars"] != "2" {
		t.Fatalf("property echo: %d %v", w.Code, got)
	}
	// The 'properties' outbox job is a no-op marker in rexworker (no Rex call);
	// nothing else may carry the attributes towards Rex.
	var outbox int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE entity<>'properties' AND payload LIKE '%beds%'`).Scan(&outbox)
	if outbox != 0 {
		t.Fatal("property attributes queued for Rex")
	}
}
