# Database performance

The server keeps everything in one SQLite file (WAL mode). This page describes
how connections are used, which request paths read from the read pool, the
production-sized performance test and the query-plan check that runs with it.

## Connection model

| Handle | Opened by | Connections | Mode | Used for |
|---|---|---|---|---|
| `a.db` (writer) | `open(path)` | exactly 1 (`SetMaxOpenConns(1)`) | `journal_mode=WAL`, `busy_timeout=5000`, `foreign_keys=ON` | every write, every read that decides a write, every transaction, session lookup |
| `a.rdb` (read pool) | `openReaders(path, n)` | `n` = `AXLEO_DB_READERS`, default 4, clamped to 1–32 | DSN pragmas `busy_timeout(5000)`, `query_only(1)`, `foreign_keys(1)` on every connection | pure reads listed below |

- `openReaders` refuses to start unless the file is already in WAL mode (the
  writer switches it on open, and serve opens the writer and migrates first).
  In rollback-journal mode readers would block the writer.
- `query_only(1)` makes any write through the pool fail with
  `SQLITE_READONLY`, so a misrouted write is an error, not a second writer.
- The single-connection guard still holds for the writer: nothing may keep a
  `*sql.Rows` from `a.db` open while issuing another `a.db` query
  (`singleConnGuard`, now `appConnGuard`, which also checks that no read-pool
  connection is left checked out after a request).
- `a.rd()` returns the pool, or the writer when there is none (CLI commands,
  old tests), so every path also works on the writer alone.
- `a.readSnapshot(ctx)` starts a read-only transaction on the pool: all reads
  in it see one committed state. Without a pool it returns the writer and a
  no-op `done`.

### Which paths use the pool, and why

| Path | Handle | Why |
|---|---|---|
| `GET /sync` (`syncPull`) | one `readSnapshot` for the whole pull | the high-water cursor and every record come from the same commit, and a 1–2 s full pull no longer holds the only connection |
| `GET /contacts/search` (database part) | pool | substring search over all contacts, called per keystroke |
| `POST /properties/within` | pool | map selection, scans the ll range |
| `GET` lists and details (`getFrom`) | pool | pure reads |
| `GET /approvals` | pool | list + details |
| `/admin` read-only reports (metrics, audit, outbox, Rex status/log/events, users, properties, parcel stats, market imports, work lists) | pool | can be slow, admin-only |
| `POST /contacts/merge-preview` | one `readSnapshot` | validation and plan read the same state |
| `duplicateRexLinks` (startup) | pool | reads every contact link once |
| `team()`, `/health` migration count | pool | pure reads |

Paths that stay on the writer:

- **Session lookup in `auth()`** (every request). It is a primary-key read, and
  the writer is only held by short write statements now that long reads are
  elsewhere. On the pool, a write request's session lookup queued behind full
  syncs and searches: with 1 full sync + 20 looping searches, writes waited up
  to 630–700 ms; on the writer they wait ~13 ms.
- **Write-then-read and read-to-decide paths**: approvals POST/decide, merge
  execute (inside its transaction), `syncPush`, contact/property PATCH,
  contact search *import* (`localForRex` with `a.db`), zone writes, admin
  mutations. A read that decides a write must see the same state the write
  will change, including the writer's own earlier statements.
- **Reads inside a transaction** use the `tx` (merge, admin console, market
  import). A pool connection cannot see the transaction's uncommitted rows.
- **Rex poller and worker** (`pollRex`, `importSave`, `rexWorker.step`). Each
  record is read-check-write: read the link and etag, check for a pending
  outbox job, then update or soft-delete. The reads are indexed point lookups
  (microseconds), so the pool would gain nothing, and reading from another
  connection would widen the window between the check and the write.
- CLI commands (`rex-dup-links`, `market-import`, reset, backup) build a
  writer-only app; `rd()` falls back to the writer.

## Performance test (`TestPerf`)

`perf_test.go` runs as part of `go test ./...` (about 15 s with a cached
template, about 25 s when the template is generated). `-short` or
`AXLEO_PERF=0` skips it.

### Data

`perf_data_test.go` generates a database from a fixed seed. Names, phone
numbers and addresses are synthetic (`example.invalid` emails, made-up
streets).

| Table | Rows | Shape |
|---|---|---|
| properties | 70,000 | 62,000 public QLD parcels with geometry, 2,800 legacy parcels, 5,000 Rex properties (500 per agent), 200 phone-created; all with `ll`, 9 suburbs |
| contacts | 8,000 | 10 agents, `mobile` + `mobileNorm`, every 8th with `data.rexId`, 5 duplicate `rexId` references |
| rex_links | 12,900 | 7,900 contacts + 5,000 properties, account 2057 |
| property_owners | 2,000 | |
| market (RP Data) | 2,500 | `property_ownership` + `market_events` + `rpdataAttributes` |
| audit_log / rex_request_log | 20,000 each | request log spread over 190 days (startup prunes past 180) |
| rex_outbox | 6,000 | contacts, relations, property notes, to-dos |
| todos / contact_notes / listings / photos | 2,000 / 3,000 / 300 / 200 | |

An agent's full `/sync` returns 2,600 properties (1.7 MB), close to
production's ~2,400.

The template is written to `AXLEO_PERF_CACHE` (default `server/.tmp/perf`,
git-ignored) as `perf-s<migrations>-<hash of migrations>-g<generator
version>-<day>.db`. It is regenerated when a migration changes, when
`perfGenVersion` is bumped, or on a new day (timestamps are relative to the
generation day because retention and report windows use the wall clock).
Older templates are deleted. Each run copies the template to a temp dir.

### Scenarios and baseline

Measured on the development machine (16 cores, local SSD). "Before" is the
same data on this branch before migration 020 and with the session lookup on
the pool. The row "without read pool" runs the same load with every query on
the single writer, i.e. the old connection model (`AXLEO_PERF_COMPARE=1`).

| Scenario | Threshold | Measured | Before |
|---|---|---|---|
| Startup: open, migrate 020 (incl. pre-migration `VACUUM INTO` backup), `serveStartup` (Rex config log, `duplicateRexLinks`, request-log retention) | < 3 s | 1.26–1.73 s | 0.45 s (no pending migration) |
| `GET /sync` full, agent (2,600 properties) | < 3 s | 0.99 s | 4.04 s |
| `GET /sync` full, admin | — | 2.49 s | 10.78 s |
| `GET /sync` incremental (5 contacts + 1 property changed) | < 300 ms | 16 ms | 17 ms |
| `GET /contacts/search` by name (Rex off) | < 200 ms | 27 ms | 27 ms |
| `GET /contacts/search` by mobile (Rex off) | < 200 ms | 27 ms | 28 ms |
| `POST /properties/within` (400 m block) | < 500 ms | 265 ms | 268 ms |
| `duplicateRexLinks` | < 1 s | 36 ms | 37 ms |
| `POST /contacts/merge-preview` | < 300 ms | 0.6 ms | 1.6 ms |
| Worst write during 1 full sync + 20 looping searches | < 500 ms | 13 ms | 626–701 ms |
| Full sync during that load | — | 1.36 s | — |
| Without read pool: worst write during that load | — | 2.83 s | |
| Without read pool: full sync during that load | — | not finished after 20 s (each of its ~15k statements queues behind the searches) | |

Each latency is the best of 2–3 runs; startup and the load test run once.
The full HTTP handler is timed, including authentication and JSON encoding.

The test also exercises, without thresholds, the other request handlers (lists,
details, admin pages, push, owner change, notes, reassign, merge execute,
to-do delete, RP Data import), one Rex poller pass and twelve Rex worker steps
against a local mock of the Rex API, so their SQL reaches the plan check.

## Query-plan check

`perf_trace_test.go` registers `sqlite_trace`, a wrapper around the modernc
driver, and the test sets `sqlDriver` to it. Every statement the server runs
during the test is recorded once, with the arguments of its first execution and
its total time (query plus row iteration). The test logs the 15 most expensive
statements.

After the scenarios, `checkPlans` runs `EXPLAIN QUERY PLAN` for every
recorded `SELECT`, `WITH`, `UPDATE`, `DELETE` and `INSERT` (triggers and FK
checks show up in insert plans). It fails on a `SCAN <table>` step that
does not use an index (covering or not) when the table is one of the large
tables:

properties, contacts, rex_links, property_owners, property_ownership,
audit_log, rex_request_log, market_events, market_imports, listings, todos,
contact_notes, sync_changes, sync_ops, rex_outbox, rex_sync_events, photos,
sessions.

Aliases are resolved (`SCAN t` → `properties`).

### Allowlist rules

A scan is allowed only through `planAllow`. Each entry has a name, the tables
it may scan, a regexp over the statement (whitespace collapsed) and a reason.
An entry that no longer matches any executed scan fails the test, so the list
cannot go stale. Add an entry only when the scan is inherent to the query and
bounded in practice, and add it to the table below.

| Entry | Tables | Reason |
|---|---|---|
| sync-scope-subqueries | photos, todos | `propertySyncSQL`'s uncorrelated `IN (SELECT …)` subqueries are materialised once per statement, then probed per property |
| full-sync-snapshot | properties, contacts, todos, listings | a full `/sync` returns every row in scope; for properties the scope predicate is evaluated over all registered properties (one pass, ~0.4 s) |
| contact-search | contacts | `LIKE '%q%'` and digits-anywhere phone matching cannot use a b-tree index |
| duplicate-rex-links | contacts | reads every contact's `data.rexId` once (startup, CLI) |
| admin-aggregates | properties, contacts, todos, listings | admin console totals, metrics, lists and property reports; admin-only, on demand, from the read pool |
| listings-list | listings | `GET /listings` is shared with every user; hundreds of rows |
| newest-first-logs | rex_request_log, rex_sync_events, rex_outbox | walks the rowid b-tree backwards and stops at `LIMIT` |
| merge-json-references | contacts, todos | merge rewrites references stored in JSON text; once per merge |
| worker-mobile-dedupe | contacts | the Rex worker matches a mobile by digits before creating a Rex contact, including legacy rows without `mobileNorm` |
| poller-assignment | properties | ranks imported properties when a poll links new records |
| market-import-index | properties | builds the address index once per uploaded file |

### Fixed by migration 020

`migrations/020_query_plan_indexes.sql` only creates indexes; no data is
rewritten.

| Index | Was |
|---|---|
| `rex_outbox_relation_property` (partial, `json_extract(payload,'$.relation.propertyId')`) | `attachRexSync` for every synced property scanned all relation jobs: 1.3 ms × 2,600 per agent sync, about 75% of the full sync |
| `contact_notes_contact` | notes per contact (`/sync`, details, merge) and the FK check on contact insert scanned all notes |
| `property_owners_contact` | owned properties per contact and the merge's owner rewrite scanned all owners |
| `todos_created_by` | agent to-do scope `assignee_id=? OR created_by=?` scanned all to-dos (`todos_scope` only serves the first branch) |
| `todos_contact` (partial) | merge's to-do contact rewrite |
| `sync_ops_created` | every `/sync/push` pruned `sync_ops` with a full scan |
| `sessions_user` | admin users list: last session per user |
| `audit_log_entity`, `audit_log_at`, `audit_log_action` | `audit_log` had no index: merged-contact check in the Rex dedupe, metrics `at>=?`, exports list |
| `market_imports_created`, `market_imports_status` | market imports page and latest completed import |

## Running locally

```sh
cd server
go test -count=1 -run '^TestPerf$' -v .                       # scenarios + plan check
AXLEO_PERF_COMPARE=1 go test -count=1 -run '^TestPerf$' -v .  # adds the no-pool comparison
AXLEO_PERF_REPORT=/path/report.md go test -run '^TestPerf$' . # writes the result table
AXLEO_PERF_CACHE=/path/dir go test -run '^TestPerf$' .        # template location
AXLEO_PERF=0 go test ./...                                    # skip it
```

When the plan check fails, the message prints the table, the plan step and the
statement. Fix it with an index in a new migration or a query rewrite. Use the
allowlist only when neither is possible.

## Deployment

- No data migration. Migration 020 creates the indexes above on the first
  start; as with every pending migration, `migrate` first writes a
  `VACUUM INTO` backup to `AXLEO_BACKUP_DIR` (needs free disk about the size of
  the database). On the production-sized test data, backup + 020 + startup
  checks take 1.3–1.7 s.
- The server opens `1 + AXLEO_DB_READERS` connections (default 5) to the same
  file. Each has its own page cache (SQLite default ~2 MB), so memory grows by
  about 8 MB. The database must be in WAL mode, which the writer already
  enforces on every start.
- `AXLEO_DB_READERS` is optional; the default 4 suits the current team.
- Long read transactions delay WAL checkpoints until they finish. The
  longest is a full admin `/sync` (~2.5 s), so the WAL does not grow
  materially.
- Rollback: the previous binary still serves a database that has migration
  020 (indexes are transparent to queries, and `migrate` skips versions it
  already finds). Its `verify` / `restore`, however, reject a snapshot that
  lists the unknown version `020_query_plan_indexes.sql`. To restore with the
  old binary, use the pre-migration backup that `migrate` wrote to
  `AXLEO_BACKUP_DIR`, or drop the 020 indexes and delete that
  `schema_migrations` row first.
