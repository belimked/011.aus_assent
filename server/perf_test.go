package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestPerf runs the production-sized scenarios from docs/db-performance.md on a
// generated database (perf_data_test.go), then checks the query plan of every
// statement the scenarios executed (perf_trace_test.go). It runs in the normal
// `go test`; -short or AXLEO_PERF=0 skips it.
func TestPerf(t *testing.T) {
	if testing.Short() || os.Getenv("AXLEO_PERF") == "0" {
		t.Skip("perf test skipped (-short or AXLEO_PERF=0)")
	}
	total := time.Now()
	template := perfTemplate(t)
	f := perfCopy(t, template, t.TempDir())
	t.Logf("template ready in %v", time.Since(total).Round(time.Millisecond))
	// Rex is off for every request path: no client, no network.
	for _, k := range []string{"REX_BASE_URL", "REX_APP_ID", "REX_EMAIL", "REX_PASSWORD", "AXLEO_REX_READ", "AXLEO_REX_WRITE", "REX_WRITE_ACCOUNT_ID"} {
		t.Setenv(k, "")
	}
	t.Setenv("REX_ACCOUNT_ID", perfAccount)

	useTraceDriver()
	t.Cleanup(func() { sqlDriver = "sqlite" })
	traceSQL.start()
	t.Cleanup(func() { traceSQL.stop() })

	results := []perfResult{}
	measure := func(name string, limit time.Duration, runs int, fn func() error) time.Duration {
		t.Helper()
		best := time.Duration(1<<63 - 1)
		for i := 0; i < runs; i++ {
			start := time.Now()
			if err := fn(); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			best = min(best, time.Since(start))
		}
		results = append(results, perfResult{name, limit, best})
		if limit > 0 && best > limit {
			t.Errorf("%s took %v, limit %v", name, best.Round(time.Millisecond), limit)
		}
		return best
	}

	// 1. Startup: everything serve runs before listening, on the database as
	// production has it before this release: migration 020 (and the backup that
	// precedes any pending migration) is applied to the full data set.
	perfUndoMigration(t, f.path, "020_query_plan_indexes.sql")
	t.Setenv("AXLEO_BACKUP_DIR", t.TempDir())
	var a *app
	measure("startup: open + migrate 020 (with backup) + serveStartup", 3*time.Second, 1, func() error {
		db, err := open(f.path)
		if err != nil {
			return err
		}
		t.Cleanup(func() { db.Close() })
		if err = migrate(db, f.path); err != nil {
			return err
		}
		rdb, err := openReaders(f.path, 4)
		if err != nil {
			return err
		}
		t.Cleanup(func() { rdb.Close() })
		a = &app{db: db, rdb: rdb, failures: map[string][]time.Time{}}
		a.serveStartup()
		return nil
	})
	agent := f.agents[3]
	tok := f.tokens[agent]
	get := func(path, token string) (*httptest.ResponseRecorder, error) {
		w := request(a, "GET", path, token, nil)
		if w.Code != 200 {
			return w, fmt.Errorf("GET %s: %d %.300s", path, w.Code, w.Body)
		}
		return w, nil
	}
	post := func(path, token string, body any) (*httptest.ResponseRecorder, error) {
		w := request(a, "POST", path, token, body)
		if w.Code != 200 && w.Code != 201 {
			return w, fmt.Errorf("POST %s: %d %.300s", path, w.Code, w.Body)
		}
		return w, nil
	}

	// 2. Full and incremental /sync for one agent.
	var cursor string
	var props, size int
	measure("GET /sync full (agent)", 3*time.Second, 2, func() error {
		w, err := get("/api/v1/sync", tok)
		if err != nil {
			return err
		}
		var m struct {
			Cursor     string `json:"cursor"`
			Properties []any  `json:"properties"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &m); err != nil {
			return err
		}
		cursor, props, size = m.Cursor, len(m.Properties), w.Body.Len()
		return nil
	})
	t.Logf("agent full sync: %d properties, %.1f MB", props, float64(size)/1e6)
	if props < 2000 || props > 3500 {
		t.Errorf("agent full sync returned %d properties; the fixture should mirror production (~2,400)", props)
	}
	measure("GET /sync full (admin, informational)", 0, 1, func() error { _, err := get("/api/v1/sync", f.tokens["admin"]); return err })
	// A phone's typical delta: a few contact edits and a property note.
	for i := 0; i < 5; i++ {
		if err := perfPatchContact(a, tok, perfContactID(3+10*i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := perfPatchProperty(a, tok, perfRexPropID(3)); err != nil {
		t.Fatal(err)
	}
	measure("GET /sync incremental (agent)", 300*time.Millisecond, 3, func() error {
		w, err := get("/api/v1/sync?since="+cursor, tok)
		if err == nil && !strings.Contains(w.Body.String(), perfContactID(13)) {
			err = fmt.Errorf("incremental pull misses an edited contact")
		}
		return err
	})

	// 3. Contact search, database only (Rex disabled).
	var mobile string
	if err := a.db.QueryRow(`SELECT mobile FROM contacts WHERE id=?`, perfContactID(1234)).Scan(&mobile); err != nil {
		t.Fatal(err)
	}
	measure("GET /contacts/search name", 200*time.Millisecond, 3, func() error {
		w, err := get("/api/v1/contacts/search?q=casey+dun", tok)
		if err == nil && !strings.Contains(w.Body.String(), `"rexReason":"disabled"`) {
			err = fmt.Errorf("Rex must be disabled: %.200s", w.Body)
		}
		return err
	})
	measure("GET /contacts/search mobile", 200*time.Millisecond, 3, func() error {
		w, err := get("/api/v1/contacts/search?q="+strings.ReplaceAll(mobile, " ", ""), tok)
		if err == nil && !strings.Contains(w.Body.String(), perfContactID(1234)) {
			err = fmt.Errorf("mobile search misses the contact: %.300s", w.Body)
		}
		return err
	})

	// 4. Map selection: a ~400m x 400m block in Sunnybank.
	poly := [][2]float64{{-27.582, 153.058}, {-27.582, 153.062}, {-27.578, 153.062}, {-27.578, 153.058}}
	var within int
	measure("POST /properties/within", 500*time.Millisecond, 3, func() error {
		w, err := post("/api/v1/properties/within", tok, map[string]any{"polygon": poly})
		var m struct{ Total int }
		if err == nil {
			err = json.Unmarshal(w.Body.Bytes(), &m)
		}
		within = m.Total
		return err
	})
	t.Logf("properties/within: %d synced properties in the block", within)

	// 5. Duplicate Rex links (startup and `axleo rex-dup-links`).
	measure("duplicateRexLinks", time.Second, 3, func() error {
		dups, err := duplicateRexLinks(a.rd())
		if err == nil && len(dups) < 5 {
			err = fmt.Errorf("expected the 5 generated duplicates, got %d", len(dups))
		}
		return err
	})

	// 6. Merge preview of two of the agent's contacts.
	mergeA, mergeB := perfContactID(3), perfContactID(13)
	var previewHash string
	measure("POST /contacts/merge-preview", 300*time.Millisecond, 3, func() error {
		w, err := post("/api/v1/contacts/merge-preview", tok, map[string]any{"aId": mergeA, "bId": mergeB})
		if err == nil {
			var m map[string]any
			_ = json.Unmarshal(w.Body.Bytes(), &m)
			previewHash, _ = m["previewHash"].(string)
			if previewHash == "" {
				err = fmt.Errorf("no previewHash: %.200s", w.Body)
			}
		}
		return err
	})

	// 7. Concurrency: a full sync and 20 searches run while the agent writes.
	// Writes go to the single writer; reads to the pool, so a write waits only
	// for other writes.
	load := perfConcurrent(t, a, f, tok, 20*time.Second)
	results = append(results,
		perfResult{fmt.Sprintf("write latency during 1 full sync + 20 searches (max of %d writes)", load.writes), 500 * time.Millisecond, load.writeMax},
		perfResult{fmt.Sprintf("full sync under that load (%d searches ran, informational)", load.searches), 0, load.sync})
	if load.writeMax > 500*time.Millisecond {
		t.Errorf("a write waited %v behind concurrent reads", load.writeMax)
	}
	if load.timedOut {
		t.Errorf("the full sync did not finish within 20s under concurrent searches")
	}
	if os.Getenv("AXLEO_PERF_COMPARE") == "1" {
		// Same load with the old single-connection shape (no read pool, every
		// query on the writer), for docs/db-performance.md.
		single := &app{db: a.db, failures: map[string][]time.Time{}}
		old := perfConcurrent(t, single, f, tok, 20*time.Second)
		note := ""
		if old.timedOut {
			note = ", sync unfinished when the searches stopped at 20s"
		}
		results = append(results,
			perfResult{fmt.Sprintf("without read pool: write latency (max of %d writes, informational)", old.writes), 0, old.writeMax},
			perfResult{fmt.Sprintf("without read pool: full sync under load (%d searches%s, informational)", old.searches, note), 0, old.sync})
	}

	// 8. Other request paths and background work, for the query-plan check.
	measure("other request paths (plan coverage)", 0, 1, func() error {
		return perfCoverage(t, a, f, tok, mergeA, mergeB, previewHash)
	})
	measure("rex poller pass (mock Rex, plan coverage)", 0, 1, func() error { return perfPoll(t, a, f) })
	measure("rex worker steps (mock Rex, plan coverage)", 0, 1, func() error { return perfWorker(t, a, f, tok) })

	// 9. Query plans of everything above.
	entries := traceSQL.stop()
	for i, e := range entries[:min(15, len(entries))] {
		t.Logf("slowest #%d: %v total, %d call(s): %.220s", i+1, e.total.Round(time.Millisecond), e.count, strings.Join(strings.Fields(e.query), " "))
	}
	issues, explained, used, err := checkPlans(a.db, entries)
	if err != nil {
		t.Fatal(err)
	}
	for _, is := range issues {
		t.Errorf("full scan of %s (%s) in:\n%s", is.table, is.detail, is.query)
	}
	for i, al := range planAllow {
		if !used[i] {
			t.Errorf("allowlist entry no longer matches any executed scan, remove it: %s", al.name)
		}
	}
	t.Logf("query plans: %d distinct statements captured, %d explained, %d allowlisted scans, %d issues", len(entries), explained, len(used), len(issues))
	perfReport(t, results, time.Since(total))
}

type perfResult struct {
	name  string
	limit time.Duration
	got   time.Duration
}

func perfReport(t *testing.T, results []perfResult, total time.Duration) {
	var b strings.Builder
	fmt.Fprintf(&b, "| 场景 | 阈值 | 实测 |\n|---|---|---|\n")
	for _, r := range results {
		limit := "—"
		if r.limit > 0 {
			limit = r.limit.String()
		}
		fmt.Fprintf(&b, "| %s | %s | %v |\n", r.name, limit, r.got.Round(100*time.Microsecond))
	}
	fmt.Fprintf(&b, "\ntotal %v\n", total.Round(time.Millisecond))
	t.Log("\n" + b.String())
	if out := os.Getenv("AXLEO_PERF_REPORT"); out != "" {
		_ = os.WriteFile(out, []byte(b.String()), 0600)
	}
}

func perfUndoMigration(t *testing.T, path, name string) {
	t.Helper()
	b, err := assets.ReadFile("migrations/" + name)
	if err != nil {
		t.Fatal(err)
	}
	db, err := open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, m := range regexp.MustCompile(`CREATE INDEX IF NOT EXISTS (\w+)`).FindAllStringSubmatch(string(b), -1) {
		if _, err = db.Exec(`DROP INDEX ` + m[1]); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`DELETE FROM schema_migrations WHERE version=?`, name); err != nil {
		t.Fatal(err)
	}
}

func perfPatchContact(a *app, token, id string) error {
	var version int
	if err := a.db.QueryRow(`SELECT version FROM contacts WHERE id=?`, id).Scan(&version); err != nil {
		return err
	}
	w := request(a, "PATCH", "/api/v1/contacts/"+id, token, map[string]any{"version": version, "last": fmt.Sprintf("Edited%d", version)})
	if w.Code != 200 {
		return fmt.Errorf("PATCH contact %s: %d %.300s", id, w.Code, w.Body)
	}
	return nil
}

func perfPatchProperty(a *app, token, id string) error {
	var version int
	if err := a.db.QueryRow(`SELECT version FROM properties WHERE id=?`, id).Scan(&version); err != nil {
		return err
	}
	w := request(a, "PATCH", "/api/v1/properties/"+id, token, map[string]any{"version": version, "notes": []any{map[string]any{"id": fmt.Sprintf("perf-%d", version), "d": "2026-11-20", "a": "Agent", "t": "Left a card"}}})
	if w.Code != 200 {
		return fmt.Errorf("PATCH property %s: %d %.300s", id, w.Code, w.Body)
	}
	return nil
}

// perfConcurrent runs one full sync and 20 contact searches (looping until the
// sync finishes, or until the deadline) and meanwhile writes contacts.
type perfLoad struct {
	writeMax, sync   time.Duration
	writes, searches int
	timedOut         bool
}

func perfConcurrent(t *testing.T, a *app, f *perfFixture, tok string, deadline time.Duration) perfLoad {
	t.Helper()
	other := f.tokens[f.agents[5]]
	stop := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	var failures []string
	var load perfLoad
	fail := func(s string) { mu.Lock(); failures = append(failures, s); mu.Unlock() }
	syncDone := make(chan struct{})
	start := time.Now()
	go func() {
		defer close(syncDone)
		if w := request(a, "GET", "/api/v1/sync", other, nil); w.Code != 200 {
			fail(fmt.Sprintf("sync %d", w.Code))
		}
		load.sync = time.Since(start)
	}()
	queries := []string{"alex", "brookes", "0412", "taylor+thorne", "quinn"}
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for {
				if w := request(a, "GET", "/api/v1/contacts/search?q="+queries[i%len(queries)], f.tokens[f.agents[i%perfAgents]], nil); w.Code != 200 {
					fail(fmt.Sprintf("search %d", w.Code))
					return
				}
				mu.Lock()
				load.searches++
				mu.Unlock()
				select {
				case <-stop:
					return
				default:
				}
			}
		}(i)
	}
	time.Sleep(20 * time.Millisecond) // let the readers start
	for i := 0; ; i++ {
		begin := time.Now()
		if err := perfPatchContact(a, tok, perfContactID(3+10*(i%50))); err != nil {
			fail(err.Error())
			break
		}
		load.writeMax = max(load.writeMax, time.Since(begin))
		load.writes++
		done := false
		select {
		case <-syncDone:
			done = true
		default:
		}
		if (done && load.writes >= 5) || time.Since(start) > deadline {
			load.timedOut = !done
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(stop)
	wg.Wait()
	<-syncDone
	for _, s := range failures {
		t.Error(s)
	}
	return load
}

// perfCoverage exercises the remaining request handlers once so their SQL is
// captured for the plan check.
func perfCoverage(t *testing.T, a *app, f *perfFixture, tok, mergeA, mergeB, hash string) error {
	t.Helper()
	admin := f.tokens["admin"]
	paths := []struct{ path, token string }{
		{"/api/v1/me", tok}, {"/api/v1/team", tok}, {"/api/v1/health", ""},
		{"/api/v1/contacts", tok}, {"/api/v1/contacts/" + perfContactID(3), tok},
		{"/api/v1/properties/" + perfRexPropID(3), tok}, {"/api/v1/todos", tok},
		{"/api/v1/listings", tok}, {"/api/v1/approvals", tok}, {"/api/v1/approvals", admin},
		{"/api/v1/contacts?since=2026-10-30T00:00:00Z", tok},
		{"/api/v1/admin/metrics?period=30", admin}, {"/api/v1/admin/properties?suburb=Sunnybank", admin},
		{"/api/v1/admin/parcel-stats", admin}, {"/api/v1/admin/rex/status", admin},
		{"/api/v1/admin/rex/log", admin}, {"/api/v1/admin/rex/events", admin},
		{"/api/v1/admin/audit", admin}, {"/api/v1/admin/audit?entity=contacts", admin}, {"/api/v1/admin/rex-outbox", admin},
		{"/api/v1/admin/contacts", admin}, {"/api/v1/admin/todos", admin}, {"/api/v1/admin/listings", admin},
		{"/api/v1/admin/captures", admin}, {"/api/v1/admin/market/imports", admin}, {"/api/v1/admin/market-review", admin},
		{"/api/v1/admin/users", admin}, {"/api/v1/admin/teams", admin}, {"/api/v1/admin/zones", admin},
		{"/api/v1/admin/overview", admin}, {"/api/v1/admin/approvals", admin}, {"/api/v1/admin/settings", admin},
		{"/api/v1/admin/exports", admin},
	}
	for _, p := range paths {
		r := httptest.NewRequest("GET", p.path, nil)
		if p.token != "" {
			r.Header.Set("Authorization", "Bearer "+p.token)
		}
		w := httptest.NewRecorder()
		a.routes().ServeHTTP(w, r)
		if w.Code != 200 {
			return fmt.Errorf("GET %s: %d %.200s", p.path, w.Code, w.Body)
		}
	}
	// Phone push: new contact + property owner link + to-do, then an edit.
	ops := []map[string]any{
		{"opId": "perf-c", "kind": "contacts", "op": "upsert", "id": "perf-new-c", "data": map[string]any{"id": "perf-new-c", "first": "New", "last": "Person", "mobile": "0400 111 222"}},
		{"opId": "perf-t", "kind": "todos", "op": "upsert", "id": "perf-new-t", "data": map[string]any{"id": "perf-new-t", "t": "Call", "d": "2026-11-21", "propId": perfRexPropID(3)}},
	}
	if w := request(a, "POST", "/api/v1/sync/push", tok, map[string]any{"ops": ops}); w.Code != 200 {
		return fmt.Errorf("push %d %.200s", w.Code, w.Body)
	}
	var version int
	_ = a.db.QueryRow(`SELECT version FROM properties WHERE id=?`, perfRexPropID(3)).Scan(&version)
	if w := request(a, "PATCH", "/api/v1/properties/"+perfRexPropID(3), tok, map[string]any{"version": version, "ownerIds": []string{perfContactID(3), "perf-new-c"}}); w.Code != 200 {
		return fmt.Errorf("owners %d %.200s", w.Code, w.Body)
	}
	if w := request(a, "POST", "/api/v1/contacts/"+perfContactID(3)+"/notes", tok, map[string]any{"d": "2026-11-20", "a": "Agent", "t": "Called"}); w.Code != 201 {
		return fmt.Errorf("note %d", w.Code)
	}
	if w := request(a, "POST", "/api/v1/contacts/"+perfContactID(23)+"/reassign", tok, map[string]any{"targetId": f.agents[3]}); w.Code != 200 && w.Code != 404 {
		return fmt.Errorf("reassign %d %.200s", w.Code, w.Body)
	}
	// Merge execute (after a fresh preview: the note above changed contact 3).
	w := request(a, "POST", "/api/v1/contacts/merge-preview", tok, map[string]any{"aId": mergeA, "bId": mergeB})
	var m map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &m)
	primary, _ := m["primaryId"].(string)
	if w = request(a, "POST", "/api/v1/contacts/"+primary+"/merge", tok, map[string]any{"secondaryId": m["secondaryId"], "previewHash": m["previewHash"]}); w.Code != 200 {
		return fmt.Errorf("merge %d %.200s", w.Code, w.Body)
	}
	if w = request(a, "DELETE", "/api/v1/todos/perf-new-t", tok, nil); w.Code != 200 {
		return fmt.Errorf("delete todo %d", w.Code)
	}
	// Market import (RP Data) matches against every property once.
	if _, err := a.importMarket(rpFixtureAt(t, "12 Acacia St", "10 Jul 2016", "Example Owner"), "propertyExport_perf.xlsx", "rpdata", "auto", false, principal{id: f.admin, role: "admin", device: "perf"}); err != nil {
		return err
	}
	return nil
}

// perfRexMock serves the Rex endpoints the poller and worker use: every linked
// record is listed with its stored etag except a few that changed.
func perfRexMock(t *testing.T, f *perfFixture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		args, _ := body["args"].(map[string]any)
		if args == nil {
			args = body
		}
		var result any = map[string]any{"rows": []any{}, "total": 0}
		switch r.URL.Path {
		case "/v1/rex/authentication/login":
			result = "token"
		case "/v1/rex/AccountSettings/read":
			result = map[string]any{"id": 2057, "name": "Sandbox", "is_test_account": true}
		case "/v1/rex/properties/search", "/v1/rex/contacts/search":
			offset, _ := args["offset"].(float64)
			rows := []any{}
			if offset == 0 {
				base, n := 500000, perfRexProps
				if strings.Contains(r.URL.Path, "contacts") {
					base, n = 300000, perfContactLinks
				}
				for i := 0; i < n; i++ {
					etag := "e1"
					if i%2000 == 7 {
						etag = "e2"
					}
					rows = append(rows, map[string]any{"_id": fmt.Sprint(base + i), "etag": etag})
				}
			}
			result = map[string]any{"rows": rows, "total": len(rows)}
		case "/v1/rex/properties/read":
			id := fmt.Sprint(args["id"])
			result = map[string]any{"id": id, "etag": "e2", "adr_street_number": "16", "adr_street_name": "Acacia St", "adr_suburb_or_town": "Sunnybank", "adr_latitude": "-27.57", "adr_longitude": "153.06",
				"related": map[string]any{"contact_reln_property": []any{map[string]any{"contact": map[string]any{"id": "300007"}, "reln_type": map[string]any{"text": "owner"}}, map[string]any{"contact": map[string]any{"id": "399999"}, "reln_type": map[string]any{"text": "owner"}}}}}
		case "/v1/rex/contacts/read":
			id := fmt.Sprint(args["id"])
			result = map[string]any{"id": id, "etag": "e2", "name_first": "Owner", "name_last": id, "phone_number": "0400 000 " + id[len(id)-3:],
				"related": map[string]any{"contact_names": []any{map[string]any{"id": 11}}, "contact_phones": []any{map[string]any{"id": 22}}}}
		case "/v1/rex/contacts/update", "/v1/rex/contacts/create":
			result = map[string]any{"id": 91, "etag": "new"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": result, "error": nil})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func perfPoll(t *testing.T, a *app, f *perfFixture) error {
	t.Helper()
	srv := perfRexMock(t, f)
	t.Setenv("REX_POLL_SOFT_DELETE", "1")
	c := newRexClient(srv.URL, "test")
	c.Token, c.Interval, c.ReadOnly, c.AccountID = "mock", 0, true, perfAccount
	c.Log = rexDBLogger(a.db)
	lastRelationPoll = time.Time{}
	t.Cleanup(func() { lastRelationPoll = time.Time{} })
	return pollRex(context.Background(), a.db, c, perfAccount)
}

func perfWorker(t *testing.T, a *app, f *perfFixture, tok string) error {
	t.Helper()
	srv := perfRexMock(t, f)
	t.Setenv("REX_WRITE_ACCOUNT_ID", perfAccount)
	// Only jobs queued by this test run; older generated jobs are finished.
	if _, err := a.db.Exec(`UPDATE rex_outbox SET status='sent' WHERE status IN ('pending','waiting_auth') AND created_at<'2026-11-02'`); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if err := perfPatchContact(a, tok, perfContactID(33+10*i)); err != nil {
			return err
		}
	}
	w := &rexWorker{db: a.db, client: newRexClient(srv.URL, "app"), email: "e", password: "p", account: perfAccount}
	w.client.Interval, w.client.Log = 0, rexDBLogger(a.db)
	for i := 0; i < 12; i++ {
		if err := w.step(context.Background()); err != nil {
			t.Logf("worker step %d: %v", i, err)
		}
	}
	_ = w.findLocalDuplicate(perfContactID(3), "0400 000 007")
	var left int
	_ = a.db.QueryRow(`SELECT count(*) FROM rex_outbox WHERE status IN ('pending','waiting_auth')`).Scan(&left)
	t.Logf("rex worker: %d job(s) still pending", left)
	return nil
}
