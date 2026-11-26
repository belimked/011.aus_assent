package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// userID is shared with contact_merge_test.go.

func searchContact(t *testing.T, a *app, token, id, first, last, mobile, email string) {
	t.Helper()
	w := request(a, "POST", "/api/v1/contacts", token, map[string]any{"id": id, "first": first, "last": last, "mobile": mobile, "email": email, "roles": []string{}, "tags": []string{}, "flags": map[string]any{}, "notes": []any{}})
	if w.Code != 201 {
		t.Fatalf("create %s: %d %s", id, w.Code, w.Body)
	}
}

type rexCall struct {
	Path string
	Body map[string]any
}

// mockSearchRex installs a read-only search client against a mock Rex (account 2057).
func mockSearchRex(t *testing.T, a *app, handle func(path string, b map[string]any, r *http.Request) (any, int)) (*contactSearch, func() []rexCall) {
	t.Helper()
	var mu sync.Mutex
	calls := []rexCall{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]any
		_ = json.NewDecoder(r.Body).Decode(&b)
		var result any = "token"
		code := 200
		switch r.URL.Path {
		case "/v1/rex/authentication/login":
		case "/v1/rex/AccountSettings/read":
			result = map[string]any{"id": 2057, "name": "Sandbox", "is_test_account": true}
		default:
			mu.Lock()
			calls = append(calls, rexCall{r.URL.Path, b})
			mu.Unlock()
			result, code = handle(r.URL.Path, b, r)
		}
		w.WriteHeader(code)
		if code >= 500 {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	t.Cleanup(srv.Close)
	c := newRexClient(srv.URL, "app")
	c.ReadOnly, c.AccountID, c.Caller, c.Interval, c.Retries, c.Log = true, "2057", "search", 0, 0, rexDBLogger(a.db)
	s := &contactSearch{cfg: rexConfig{AccountID: "2057", Email: "e", Password: "p", Read: true}, client: c, sem: make(chan struct{}, 1), cache: map[string]rexCacheEntry{}, offered: map[string]map[string]time.Time{}, budget: searchRexBudget}
	a.search = s
	return s, func() []rexCall { mu.Lock(); defer mu.Unlock(); return append([]rexCall(nil), calls...) }
}

func disableSearchRex(a *app) {
	a.search = &contactSearch{sem: make(chan struct{}, 1), cache: map[string]rexCacheEntry{}, offered: map[string]map[string]time.Time{}, budget: searchRexBudget}
}

type searchResp struct {
	Items       []map[string]any `json:"items"`
	RexTimedOut bool             `json:"rexTimedOut"`
	RexSkipped  bool             `json:"rexSkipped"`
	RexError    bool             `json:"rexError"`
	RexReason   string           `json:"rexReason"`
}

func doSearch(t *testing.T, a *app, token, q string) (searchResp, string) {
	t.Helper()
	w := request(a, "GET", "/api/v1/contacts/search?q="+strings.ReplaceAll(strings.ReplaceAll(q, " ", "%20"), "+", "%2B"), token, nil)
	if w.Code != 200 {
		t.Fatalf("search %q: %d %s", q, w.Code, w.Body)
	}
	var r searchResp
	if err := json.Unmarshal(w.Body.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r, w.Body.String()
}
func itemByID(items []map[string]any, key, id string) map[string]any {
	for _, it := range items {
		if fmt.Sprint(it[key]) == id {
			return it
		}
	}
	return nil
}

func TestContactSearchScope(t *testing.T) {
	a, tok := setup(t)
	disableSearchRex(a)
	searchContact(t, a, tok["mary"], "m1", "Teddie", "Bear", "0401 999 088", "teddie@example.com")
	searchContact(t, a, tok["john"], "j1", "Teddie", "Other", "0402 111 222", "other@example.com")
	r, raw := doSearch(t, a, tok["mary"], "teddie")
	if len(r.Items) != 2 || !r.RexSkipped || r.RexReason != "disabled" {
		t.Fatal(raw)
	}
	if mine := itemByID(r.Items, "id", "m1"); mine["full"] != true || mine["mobile"] != "0401 999 088" || mine["email"] != "teddie@example.com" {
		t.Fatalf("own contact must be full: %v", mine)
	}
	if r.Items[0]["id"] != "m1" {
		t.Fatalf("own contacts first: %v", r.Items)
	}
	john := itemByID(r.Items, "id", "j1")
	if john["full"] != false || john["ownerName"] != "John" || john["name"] != "T••• O•••" {
		t.Fatalf("other agent's contact: %v", john)
	}
	for _, k := range []string{"mobile", "email", "first", "last", "notes"} {
		if _, ok := john[k]; ok {
			t.Fatalf("limited view leaked %s: %v", k, john)
		}
	}
	if strings.Contains(raw, "0402") || strings.Contains(raw, "other@example.com") {
		t.Fatalf("limited results leaked contact details: %s", raw)
	}
	r, raw = doSearch(t, a, tok["admi"], "teddie")
	for _, it := range r.Items {
		if it["full"] != true {
			t.Fatalf("admin sees full details: %s", raw)
		}
	}
	if !strings.Contains(raw, "0402 111 222") {
		t.Fatal(raw)
	}
}

func TestContactSearchMobileNormalisation(t *testing.T) {
	a, tok := setup(t)
	disableSearchRex(a)
	searchContact(t, a, tok["john"], "j1", "Teddie", "Bear", "0401 999 088", "")
	if _, err := a.db.Exec(`INSERT INTO contacts(id,owner_id,first,last,mobile,data,created_at,updated_at) VALUES('legacy',?,'Old','Row','+61 401 999 088','{"first":"Old","last":"Row","mobile":"+61 401 999 088"}',?,?)`, userID(t, a, "john@axleo.local"), now(), now()); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"0401999088", "+61 401 999 088", "0401 999 088", "61401999088", "401 999", "999088"} {
		r, raw := doSearch(t, a, tok["mary"], q)
		if itemByID(r.Items, "id", "j1") == nil || itemByID(r.Items, "id", "legacy") == nil {
			t.Fatalf("%q: %s", q, raw)
		}
	}
	for _, q := range []string{"0", "04", " 4 ", "x"} {
		if r, raw := doSearch(t, a, tok["mary"], q); len(r.Items) != 0 {
			t.Fatalf("%q must be too short: %s", q, raw)
		}
	}
	if q, _ := parseSearchQuery("0401 999"); q.rexAllowed || !q.phone {
		t.Fatal("partial numbers stay out of Rex")
	}
	if q, _ := parseSearchQuery("+61 401 999 088"); !q.rexAllowed || q.key != "phone:0401999088" {
		t.Fatal(q)
	}
	if q, _ := parseSearchQuery("  Teddie   BEAR "); q.key != "name:teddie bear" || q.phone {
		t.Fatal(q)
	}
}

func rexFindResult(contacts ...map[string]any) map[string]any {
	verdict := "none"
	if len(contacts) == 1 {
		verdict = "unique"
	} else if len(contacts) > 1 {
		verdict = "multiple"
	}
	list := []any{}
	for _, c := range contacts {
		list = append(list, c)
	}
	return map[string]any{"verdict": verdict, "truncated": false, "contacts": list}
}

func TestContactSearchMergesRexByPhone(t *testing.T) {
	a, tok := setup(t)
	searchContact(t, a, tok["john"], "j1", "Ted", "Linked", "0401 000 001", "")
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('contacts','j1','777','Contacts',?,'2057')`, now()); err != nil {
		t.Fatal(err)
	}
	_, calls := mockSearchRex(t, a, func(path string, b map[string]any, _ *http.Request) (any, int) {
		if path != "/v1/rex/contacts/find-matching-contacts" {
			t.Errorf("unexpected %s", path)
			return nil, 500
		}
		return rexFindResult(
			map[string]any{"id": 68313642, "type": "person", "name": "Teddie Bear", "email_address": "t@example.com", "phone_number": "0401 999 088"},
			map[string]any{"id": 777, "type": "person", "name": "Ted Linked", "phone_number": "0401 999 088"},
		), 200
	})
	r, raw := doSearch(t, a, tok["mary"], "0401 999 088")
	if r.RexTimedOut || r.RexSkipped || r.RexError || len(r.Items) != 2 {
		t.Fatal(raw)
	}
	in := itemByID(r.Items, "rexId", "68313642")
	if in["source"] != "rex" || in["inRex"] != true || in["mobileMasked"] != "0401 999 •••" || in["name"] != "Teddie Bear" {
		t.Fatalf("rex-only item: %v", in)
	}
	if strings.Contains(raw, "t@example.com") || strings.Contains(raw, "0401 999 088\"") {
		t.Fatalf("rex-only item leaked details: %s", raw)
	}
	if j := itemByID(r.Items, "id", "j1"); j["source"] != "local" || j["full"] != false || j["ownerName"] != "John" || j["inRex"] != true {
		t.Fatalf("linked Rex hit must map to the local contact under scope rules: %v", j)
	}
	cs := calls()
	if len(cs) != 1 {
		t.Fatal(cs)
	}
	phones, _ := cs[0].Body["phones"].([]any)
	if cs[0].Body["contact_type"] != "any" || len(phones) != 1 || phones[0] != "0401 999 088" || cs[0].Body["limit"] != float64(20) {
		t.Fatalf("findMatchingContacts body: %v", cs[0].Body)
	}
	// Same normalised query: served from the 60 s cache.
	if r2, raw2 := doSearch(t, a, tok["mary"], "+61401999088"); len(r2.Items) != 2 || len(calls()) != 1 {
		t.Fatalf("cache: %s %v", raw2, calls())
	}
	var logged int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_request_log WHERE caller='search' AND method='find-matching-contacts'`).Scan(&logged)
	if logged != 1 {
		t.Fatalf("rex_request_log rows: %d", logged)
	}
	// Expired cache entries go back to Rex.
	a.search.mu.Lock()
	for k, e := range a.search.cache {
		e.at = time.Now().Add(-2 * searchCacheTTL)
		a.search.cache[k] = e
	}
	a.search.mu.Unlock()
	doSearch(t, a, tok["mary"], "0401999088")
	if len(calls()) != 2 {
		t.Fatal("expired cache must query Rex again")
	}
}

func TestContactSearchRexNameBody(t *testing.T) {
	a, tok := setup(t)
	_, calls := mockSearchRex(t, a, func(path string, b map[string]any, _ *http.Request) (any, int) {
		if path != "/v1/rex/contacts/search" {
			t.Errorf("unexpected %s", path)
		}
		return map[string]any{"rows": []any{map[string]any{"id": "68313642", "name_first": "Teddie", "name_last": "Bear", "phone_number": "0401 999 088", "system_owner_user": map[string]any{"name": "Rex Agent"}}}, "total": 1}, 200
	})
	r, raw := doSearch(t, a, tok["mary"], "Teddie")
	if len(r.Items) != 1 || r.Items[0]["name"] != "Teddie Bear" || r.Items[0]["ownerName"] != "Rex Agent" {
		t.Fatal(raw)
	}
	b := calls()[0].Body
	crit := b["criteria"].([]any)[0].(map[string]any)
	if crit["name"] != "contact.name_full" || crit["type"] != "like" || crit["value"] != "%Teddie%" || b["limit"] != float64(20) {
		t.Fatalf("Contacts::search body: %v", b)
	}
	// Partial numbers stay in the database.
	if r, _ := doSearch(t, a, tok["mary"], "0401 99"); !r.RexSkipped || r.RexReason != "partial number" || len(calls()) != 1 {
		t.Fatal("partial number reached Rex")
	}
}

func TestContactSearchRexTimeoutKeepsDatabaseResults(t *testing.T) {
	a, tok := setup(t)
	searchContact(t, a, tok["john"], "j1", "Teddie", "Bear", "0401 999 088", "")
	mockSearchRex(t, a, func(path string, b map[string]any, r *http.Request) (any, int) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
		return rexFindResult(), 200
	})
	started := time.Now()
	r, raw := doSearch(t, a, tok["mary"], "Teddie")
	if took := time.Since(started); took > 3*time.Second {
		t.Fatalf("search took %v, budget is 3 s", took)
	}
	if !r.RexTimedOut || r.RexError || itemByID(r.Items, "id", "j1") == nil {
		t.Fatal(raw)
	}
}

func TestContactSearchRexFailureDegrades(t *testing.T) {
	a, tok := setup(t)
	searchContact(t, a, tok["mary"], "m1", "Teddie", "Bear", "0401 999 088", "")
	mockSearchRex(t, a, func(string, map[string]any, *http.Request) (any, int) { return nil, 500 })
	r, raw := doSearch(t, a, tok["mary"], "Teddie")
	if !r.RexError || r.RexTimedOut || len(r.Items) != 1 {
		t.Fatal(raw)
	}
	// Failures are not cached.
	a.search.mu.Lock()
	n := len(a.search.cache)
	a.search.mu.Unlock()
	if n != 0 {
		t.Fatal("failed Rex lookups must not be cached")
	}
}

func TestContactSearchRateLimitDegrades(t *testing.T) {
	a, tok := setup(t)
	searchContact(t, a, tok["mary"], "m1", "Zed", "Limit", "", "")
	_, calls := mockSearchRex(t, a, func(string, map[string]any, *http.Request) (any, int) {
		return map[string]any{"rows": []any{}, "total": 0}, 200
	})
	for i := 0; i < searchRexPerMin; i++ {
		if r, raw := doSearch(t, a, tok["mary"], fmt.Sprintf("name%02d", i)); r.RexSkipped {
			t.Fatalf("call %d skipped: %s", i, raw)
		}
	}
	r, raw := doSearch(t, a, tok["mary"], "Zed")
	if !r.RexSkipped || r.RexReason != "rate limited" || len(r.Items) != 1 {
		t.Fatal(raw)
	}
	if len(calls()) != searchRexPerMin {
		t.Fatal(len(calls()))
	}
	// Cached queries are still answered, and other users have their own budget.
	if r, _ := doSearch(t, a, tok["mary"], "name00"); r.RexSkipped {
		t.Fatal("cache hits are free")
	}
	if r, _ := doSearch(t, a, tok["john"], "Zed"); r.RexSkipped {
		t.Fatal("per-user limit")
	}
}

func TestContactSearchClientIsReadOnlyAndShared(t *testing.T) {
	a, _ := setup(t)
	for k, v := range map[string]string{"REX_BASE_URL": "http://127.0.0.1:1", "REX_APP_ID": "a", "REX_EMAIL": "e", "REX_PASSWORD": "p", "REX_ACCOUNT_ID": "2057", "AXLEO_REX_READ": "1"} {
		t.Setenv(k, v)
	}
	s := a.contactSearcher()
	if s.client == nil || !s.client.ReadOnly || s.client.Gate != rexAccountGate || s.client.Caller != "search" || s.cfg.AccountID != "2057" {
		t.Fatalf("search client: %+v", s.client)
	}
	t.Setenv("AXLEO_REX_WRITE", "1")
	t.Setenv("REX_WRITE_ACCOUNT_ID", "2057")
	if w := newRexWorker(a.db); w == nil || w.client.Gate != rexAccountGate {
		t.Fatal("worker must share the account gate")
	}
	// A write through the search client is refused before any request.
	if _, _, err := s.client.Call(t.Context(), "Contacts", "create", map[string]any{}); err == nil || !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "read-only") {
		t.Fatal(err)
	}
	// Account mismatch: the search refuses to use the session.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var result any = "token"
		if r.URL.Path == "/v1/rex/AccountSettings/read" {
			result = map[string]any{"id": 758, "name": "Production"}
		} else if r.URL.Path != "/v1/rex/authentication/login" {
			t.Errorf("request after account mismatch: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	defer srv.Close()
	s.client = newRexClient(srv.URL, "a")
	s.client.ReadOnly, s.client.AccountID, s.client.Interval = true, "2057", 0
	if _, err := s.rexLookup(t.Context(), searchQuery{raw: "Teddie", key: "name:teddie"}); err == nil || s.client.Token != "" {
		t.Fatal("account mismatch must be refused")
	}
}

func TestRexGateSharedSpacing(t *testing.T) {
	g := &rexGate{Interval: 50 * time.Millisecond}
	a, b := newRexClient("http://x", "a"), newRexClient("http://x", "b")
	a.Gate, b.Gate = g, g
	started := time.Now()
	for i := 0; i < 3; i++ {
		if err := a.wait(t.Context()); err != nil {
			t.Fatal(err)
		}
		if err := b.wait(t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	if took := time.Since(started); took < 250*time.Millisecond {
		t.Fatalf("six calls on one gate must be spaced: %v", took)
	}
}

func TestContactImportRex(t *testing.T) {
	a, tok := setup(t)
	mary := userID(t, a, "mary@axleo.local")
	t.Setenv("REX_USER_MAP", "80559:john@axleo.local") // the caller wins over the Rex owner mapping
	_, calls := mockSearchRex(t, a, func(path string, b map[string]any, _ *http.Request) (any, int) {
		switch path {
		case "/v1/rex/contacts/find-matching-contacts":
			return rexFindResult(map[string]any{"id": 68313642, "name": "Teddie Bear", "phone_number": "0401 999 088"}), 200
		case "/v1/rex/contacts/read":
			if fmt.Sprint(b["id"]) != "68313642" {
				t.Errorf("read %v", b)
			}
			return map[string]any{"id": 68313642, "etag": "e1", "system_owner_user": map[string]any{"id": 80559}, "related": map[string]any{
				"contact_names":  []any{map[string]any{"id": 1, "name_first": "Teddie", "name_last": "Bear"}},
				"contact_phones": []any{map[string]any{"id": 2, "phone_number": "0401 999 088", "phone_primary": true}}}}, 200
		}
		t.Errorf("unexpected %s", path)
		return nil, 500
	})
	if w := request(a, "POST", "/api/v1/contacts/import-rex", tok["mary"], map[string]any{"rexId": "68313642"}); w.Code != 404 {
		t.Fatalf("import before it was offered: %d", w.Code)
	}
	doSearch(t, a, tok["mary"], "0401 999 088")
	if w := request(a, "POST", "/api/v1/contacts/import-rex", tok["john"], map[string]any{"rexId": "68313642"}); w.Code != 404 {
		t.Fatalf("offer is per user: %d", w.Code)
	}
	w := request(a, "POST", "/api/v1/contacts/import-rex", tok["mary"], map[string]any{"rexId": "68313642"})
	if w.Code != 201 {
		t.Fatal(w.Code, w.Body)
	}
	m := obj(t, w)
	item := m["item"].(map[string]any)
	if m["imported"] != true || item["id"] != "rex-contacts-2057-68313642" || item["full"] != true || item["mobile"] != "0401 999 088" {
		t.Fatal(m)
	}
	var owner string
	_ = a.db.QueryRow(`SELECT owner_id FROM contacts WHERE id='rex-contacts-2057-68313642'`).Scan(&owner)
	if owner != mary {
		t.Fatalf("owner %q, want caller", owner)
	}
	// It is now synced to Mary's phone.
	if pull := request(a, "GET", "/api/v1/sync", tok["mary"], nil); !strings.Contains(pull.Body.String(), "rex-contacts-2057-68313642") {
		t.Fatal("imported contact missing from sync")
	}
	// Second import: already linked, no new Rex read.
	before := len(calls())
	w = request(a, "POST", "/api/v1/contacts/import-rex", tok["mary"], map[string]any{"rexId": "68313642"})
	if w.Code != 200 || obj(t, w)["imported"] != false || len(calls()) != before {
		t.Fatal(w.Code, w.Body)
	}
	// Search now shows the local contact, not "In Rex".
	r, raw := doSearch(t, a, tok["john"], "0401 999 088")
	if len(r.Items) != 1 || r.Items[0]["source"] != "local" || r.Items[0]["full"] != false || r.Items[0]["ownerName"] != "Mary" {
		t.Fatal(raw)
	}
}

func TestContactSearchSingleConnection(t *testing.T) {
	a, tok := setup(t)
	searchContact(t, a, tok["mary"], "m1", "Teddie", "Bear", "0401 999 088", "")
	searchContact(t, a, tok["john"], "j1", "Teddie", "Linked", "0401 999 089", "")
	if _, err := a.db.Exec(`INSERT INTO rex_links(entity,local_id,rex_id,rex_service,created_at,account_id) VALUES('contacts','j1','777','Contacts',?,'2057')`, now()); err != nil {
		t.Fatal(err)
	}
	mockSearchRex(t, a, func(path string, b map[string]any, _ *http.Request) (any, int) {
		switch path {
		case "/v1/rex/contacts/search":
			return map[string]any{"rows": []any{map[string]any{"id": "777", "name_first": "Teddie", "name_last": "Linked"}, map[string]any{"id": "888", "name_first": "Teddie", "name_last": "New", "phone_number": "0401 999 087"}}}, 200
		case "/v1/rex/contacts/read":
			return map[string]any{"id": 888, "etag": "e", "related": map[string]any{"contact_names": []any{map[string]any{"name_first": "Teddie", "name_last": "New"}}, "contact_phones": []any{map[string]any{"phone_number": "0401 999 087"}}}}, 200
		}
		return nil, 500
	})
	singleConnGuard(t, a.db, "contact search + import", func() error {
		r, raw := doSearch(t, a, tok["mary"], "Teddie")
		if len(r.Items) != 3 {
			return fmt.Errorf("search: %s", raw)
		}
		if w := request(a, "POST", "/api/v1/contacts/import-rex", tok["mary"], map[string]any{"rexId": "888"}); w.Code != 201 {
			return fmt.Errorf("import %d %s", w.Code, w.Body)
		}
		return nil
	})
}
