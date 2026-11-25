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
