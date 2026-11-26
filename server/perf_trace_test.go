package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// SQL capture for the query-plan check (docs/db-performance.md). The perf test
// sets sqlDriver to "sqlite_trace", a thin wrapper around modernc's driver that
// records the text (and first arguments) of every statement the server runs.

type sqlTraceEntry struct {
	query string
	args  []any
	count int
	total time.Duration // query + row iteration, summed over all executions
}

type sqlTracer struct {
	mu   sync.Mutex
	on   bool
	seen map[string]*sqlTraceEntry
}

var traceSQL = &sqlTracer{seen: map[string]*sqlTraceEntry{}}

func (t *sqlTracer) record(query string, args []driver.NamedValue, took time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.on {
		return
	}
	if e := t.seen[query]; e != nil {
		e.count++
		e.total += took
		return
	}
	vals := make([]any, len(args))
	for i, a := range args {
		vals[i] = a.Value
	}
	t.seen[query] = &sqlTraceEntry{query: query, args: vals, count: 1, total: took}
}

func (t *sqlTracer) start() {
	t.mu.Lock()
	t.on = true
	t.mu.Unlock()
}

func (t *sqlTracer) stop() []sqlTraceEntry {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.on = false
	out := make([]sqlTraceEntry, 0, len(t.seen))
	for _, e := range t.seen {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].total > out[j].total })
	return out
}

var registerTrace sync.Once

func useTraceDriver() {
	registerTrace.Do(func() {
		probe, err := sql.Open("sqlite", "")
		if err != nil {
			panic(err)
		}
		sql.Register("sqlite_trace", traceDriver{probe.Driver()})
		probe.Close()
	})
	sqlDriver = "sqlite_trace"
}

type traceDriver struct{ d driver.Driver }

func (t traceDriver) Open(name string) (driver.Conn, error) {
	c, err := t.d.Open(name)
	if err != nil {
		return nil, err
	}
	return &traceConn{c}, nil
}

type traceConn struct{ c driver.Conn }

func (t *traceConn) Prepare(q string) (driver.Stmt, error) {
	traceSQL.record(q, nil, 0)
	return t.c.Prepare(q)
}
func (t *traceConn) Close() error              { return t.c.Close() }
func (t *traceConn) Begin() (driver.Tx, error) { return t.c.Begin() } //nolint:staticcheck // required by driver.Conn
func (t *traceConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	return t.c.(driver.ConnBeginTx).BeginTx(ctx, o)
}
func (t *traceConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	traceSQL.record(q, nil, 0)
	return t.c.(driver.ConnPrepareContext).PrepareContext(ctx, q)
}
func (t *traceConn) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	start := time.Now()
	res, err := t.c.(driver.ExecerContext).ExecContext(ctx, q, args)
	traceSQL.record(q, args, time.Since(start))
	return res, err
}
func (t *traceConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	start := time.Now()
	rows, err := t.c.(driver.QueryerContext).QueryContext(ctx, q, args)
	if err != nil {
		traceSQL.record(q, args, time.Since(start))
		return nil, err
	}
	return &traceRows{Rows: rows, query: q, args: args, took: time.Since(start)}, nil
}

// traceRows adds the time spent stepping through the result to the statement.
type traceRows struct {
	driver.Rows
	query string
	args  []driver.NamedValue
	took  time.Duration
	done  bool
}

func (r *traceRows) Next(dest []driver.Value) error {
	start := time.Now()
	err := r.Rows.Next(dest)
	r.took += time.Since(start)
	return err
}
func (r *traceRows) Close() error {
	if !r.done {
		r.done = true
		traceSQL.record(r.query, r.args, r.took)
	}
	return r.Rows.Close()
}
func (t *traceConn) Ping(ctx context.Context) error { return t.c.(driver.Pinger).Ping(ctx) }
func (t *traceConn) ResetSession(ctx context.Context) error {
	return t.c.(driver.SessionResetter).ResetSession(ctx)
}
func (t *traceConn) IsValid() bool { return t.c.(driver.Validator).IsValid() }

// planTables are the tables that are large in production (or grow without
// bound). A full SCAN of one of them fails the check unless allowlisted.
var planTables = map[string]bool{
	"properties": true, "contacts": true, "rex_links": true, "property_owners": true,
	"property_ownership": true, "audit_log": true, "rex_request_log": true,
	"market_events": true, "market_imports": true, "listings": true, "todos": true,
	"contact_notes": true, "sync_changes": true, "sync_ops": true, "rex_outbox": true,
	"rex_sync_events": true, "photos": true, "sessions": true,
}

// planAllow lists the full scans that are intended. Each entry names the tables
// it may scan, a regexp matching the statements (whitespace collapsed), and the
// reason. Keep it small; every entry is documented in docs/db-performance.md,
// and an entry that no longer matches any executed scan fails the test.
var planAllow = []struct {
	name   string
	tables []string
	re     *regexp.Regexp
	reason string
}{
	{"sync-scope-subqueries", []string{"photos", "todos"},
		regexp.MustCompile(`IN \(SELECT property_id FROM photos WHERE property_id IS NOT NULL\) OR \w+\.id IN \(SELECT COALESCE\(NULLIF\(td\.property_id,''\),json_extract\(td\.data,'\$\.propId'\)\) FROM todos td`),
		"propertySyncSQL: uncorrelated IN subqueries are materialised once per statement (one pass over photos and todos), then probed per property"},
	{"full-sync-snapshot", []string{"properties", "contacts", "todos", "listings"},
		regexp.MustCompile(`^SELECT t\.id FROM (properties|contacts|todos|listings) t WHERE t\.deleted_at IS NULL( AND \(owner_id IS NULL OR t\.owner_id=\?\))?( AND t\.registered=1 AND \(|$)`),
		"full /sync: returns every row in scope; for properties the scope is the propertySyncSQL predicate over all registered properties (one pass per full pull)"},
	{"contact-search", []string{"contacts"},
		regexp.MustCompile(`FROM contacts c LEFT JOIN users u ON u\.id=c\.owner_id WHERE c\.deleted_at IS NULL AND \((instr|lower)\(`),
		"contact search: substring match (LIKE '%q%', phone digits anywhere) cannot use a b-tree index; one pass over contacts"},
	{"duplicate-rex-links", []string{"contacts"},
		regexp.MustCompile(`^SELECT id,CAST\(json_extract\(data,'\$\.rexId'\) AS TEXT\) FROM contacts`),
		"duplicateRexLinks: reads every contact's data.rexId once (startup, axleo rex-dup-links)"},
	{"admin-aggregates", []string{"properties", "contacts", "todos", "listings"},
		regexp.MustCompile(`^SELECT count\(\*\) FROM (contacts|todos|listings|properties) WHERE deleted_at IS NULL$|^SELECT (count\(\*\)|substr\(a\.created_at,1,10\),count\(\*\)|a\.id,COALESCE\(a\.due,''\),COALESCE\(a\.text,''\),COALESCE\(u\.name,''\)) FROM (contacts|todos|listings) a JOIN users u ON u\.id=a\.(owner_id|assignee_id) WHERE a\.deleted_at IS NULL|^SELECT id FROM (contacts|todos|listings) WHERE deleted_at IS NULL ORDER BY updated_at DESC LIMIT 500$|^SELECT p\.suburb,count\(\*\)|^SELECT count\(\*\),count\(CASE WHEN po\.property_id IS NOT NULL THEN 1 END\) FROM properties a|^SELECT p\.id,trim\(COALESCE\(p\.no,''\)\|\|' '\|\|COALESCE\(p\.street,''\)\),p\.suburb,o\.last_sale_date`),
		"admin console totals, metrics, lists and property reports: admin-only, on demand, aggregate over the whole table and run on the read pool"},
	{"listings-list", []string{"listings"},
		regexp.MustCompile(`^SELECT id FROM listings WHERE 1=1 AND deleted_at IS NULL ORDER BY updated_at,id$`),
		"GET /listings: listings are shared with every user (no owner scope); hundreds of rows"},
	{"newest-first-logs", []string{"rex_request_log", "rex_sync_events", "rex_outbox"},
		regexp.MustCompile(`^SELECT .* FROM (rex_request_log|rex_sync_events|rex_outbox) WHERE (1=1|last_error IS NOT NULL) ORDER BY id DESC LIMIT \?( OFFSET \?)?$|^SELECT COALESCE\(last_error,''\) FROM rex_outbox WHERE last_error IS NOT NULL ORDER BY id DESC LIMIT 1$`),
		"admin log pages: walk the rowid b-tree backwards from the newest row and stop at LIMIT"},
	{"merge-json-references", []string{"contacts", "todos"},
		regexp.MustCompile(`instr\(data,\?\)>0`),
		"contact merge: rewrites references to the merged id stored inside JSON text; runs once per merge, one pass over contacts and todos"},
	{"worker-mobile-dedupe", []string{"contacts"},
		regexp.MustCompile(`^SELECT c\.id,c\.updated_at,c\.data,COALESCE\(c\.mobile,''\), EXISTS`),
		"Rex worker, before creating a Rex contact: matches the mobile by digits, including legacy rows without mobileNorm"},
	{"poller-assignment", []string{"properties"},
		regexp.MustCompile(`^SELECT p\.id, p\.owner_id FROM properties p WHERE json_extract\(p\.data,'\$\.source'\)='rex_import'`),
		"Rex poller: ranks all imported properties to assign newly linked ones; runs only when a poll links new records"},
	{"market-import-index", []string{"properties"},
		regexp.MustCompile(`^SELECT id,no,street,suburb,data FROM properties WHERE deleted_at IS NULL$`),
		"market import: builds the in-memory address index once per uploaded file"},
}

var planAliasRE = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([a-z_][a-z0-9_]*)(?:\s+(?:AS\s+)?([a-z_][a-z0-9_]*))?`)
var planScanRE = regexp.MustCompile(`^SCAN (\S+)(.*)$`)

var sqlKeywords = map[string]bool{"where": true, "join": true, "left": true, "inner": true, "cross": true, "on": true, "group": true, "order": true, "limit": true, "set": true, "using": true, "natural": true, "union": true, "select": true, "values": true, "outer": true, "having": true, "window": true, "as": true}

// planAliases maps every alias (and table name) in q to its table.
func planAliases(q string) map[string]string {
	out := map[string]string{}
	for _, m := range planAliasRE.FindAllStringSubmatch(q, -1) {
		table := strings.ToLower(m[1])
		out[table] = table
		if alias := strings.ToLower(m[2]); alias != "" && !sqlKeywords[alias] {
			out[alias] = table
		}
	}
	return out
}

type planIssue struct {
	query, table, detail string
}

// checkPlans runs EXPLAIN QUERY PLAN for every captured statement that reads
// rows (SELECT, UPDATE, DELETE, INSERT … SELECT) and returns full scans of
// planTables that are not allowlisted. used reports which allowlist entries
// matched, so stale entries can be removed.
func checkPlans(db *sql.DB, entries []sqlTraceEntry) (issues []planIssue, explained int, used map[int]bool, err error) {
	used = map[int]bool{}
	for _, e := range entries {
		q := strings.TrimSpace(e.query)
		head := strings.ToUpper(strings.Fields(q + " x")[0])
		if head != "SELECT" && head != "UPDATE" && head != "DELETE" && head != "INSERT" && head != "WITH" {
			continue
		}
		if strings.Count(q, ";") > 0 && !strings.HasSuffix(q, ";") {
			continue // multi-statement script (migrations)
		}
		rows, qerr := db.Query("EXPLAIN QUERY PLAN "+q, e.args...)
		if qerr != nil {
			return nil, explained, used, fmt.Errorf("explain %q: %w", q, qerr)
		}
		explained++
		aliases := planAliases(q)
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
				rows.Close()
				return nil, explained, used, err
			}
			m := planScanRE.FindStringSubmatch(detail)
			if m == nil || strings.Contains(m[2], "USING INDEX") || strings.Contains(m[2], "USING COVERING INDEX") || strings.Contains(m[2], "USING INTEGER PRIMARY KEY") {
				continue
			}
			table := aliases[strings.ToLower(m[1])]
			if table == "" {
				table = strings.ToLower(m[1])
			}
			if !planTables[table] {
				continue
			}
			allowed := false
			flat := strings.Join(strings.Fields(q), " ")
			for i, a := range planAllow {
				if slices.Contains(a.tables, table) && a.re.MatchString(flat) {
					allowed, used[i] = true, true
					break
				}
			}
			if !allowed {
				issues = append(issues, planIssue{query: q, table: table, detail: detail})
			}
		}
		rows.Close()
	}
	return issues, explained, used, nil
}
