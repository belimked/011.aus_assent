# AxleoCollect API (B4)

Go 1.27, SQLite (pure Go), standard-library HTTP server. From `server/`:

Market data is licensed to the agency (not supplied by Rex). Never export or redistribute provider records; the admin UI offers no bulk export. CSV (UTF-8/BOM) imports are supported; **XLSX 请另存为 CSV**. See `../docs/market-data.md` for aliases, privacy, API authorization and import instructions. `axleo market-import <file> [--provider pricefinder] [--kind auto|sale|listing|ownership] [--dry-run]`; admin-only `POST /api/v1/admin/market/imports` (multipart `file`, `kind`, `provider`, `dryRun`; 20 MB), `GET /api/v1/admin/market/imports`, `GET /api/v1/admin/market/imports/{id}`. `axleo market-pull` is a no-op until an authorized API contract/documentation is supplied; the optional deploy timer is disabled by default.


```sh
go test ./... && go vet ./...
go build -ldflags '-X main.version=local' -o axleo .
export AXLEO_DB=./axleo.db AXLEO_SEED_PASSWORD='a-long-initial-password'
./axleo migrate
./axleo seed
./axleo serve
```

`AXLEO_ADDR` defaults to `127.0.0.1:8082`; `AXLEO_DB` to `./axleo.db`; migration snapshots use `AXLEO_BACKUP_DIR` (default `backup/` beside the DB). `axleo passwd <email>` reads the replacement password from stdin and revokes existing sessions. Without `AXLEO_SEED_PASSWORD`, the first seed prints a generated password **once**; save it. `seed` is repeatable and does not overwrite existing accounts or records.

All protected endpoints use `Authorization: Bearer <token>`; `/api/v1/health` is public. Collections return JSON arrays; PATCH requires `version`, with stale updates returning `409` and `current`. `GET /sync?since=<opaque cursor>` returns `cursor`, `full`, registered contacts/properties/todos and B3 resources, plus `deleted` ID arrays, all scoped to the bearer. An empty cursor requests a full snapshot. `POST /sync/push` accepts `ops` with `opId`, `kind`, `op`, `id`, optional `baseVersion`, and `data`; `results` contain per-op `status` and `record`. Op IDs are idempotent per user for seven days. `GET /team` returns same-team user IDs and names. API supports cross-origin bearer requests. Admin-only endpoints are under `/api/v1/admin/`.

B3 CRUD kinds: `captures`, `enquiries`, `leads`, `listings`, `ofi_attendees`, `sales`, `contracts`, `referrals`, `market_items`, `zones`, `tags`. `listings` are readable by every agent but only the listing owner/admin can PATCH or DELETE; non-owner writes return **404**. OFI attendee personal data is returned separately from globally visible listings. `GET /approvals` lists own requests (admin: all); `POST /approvals` accepts `{kind:"merge",primaryId,secondaryId}` or `{kind:"reassign",entity:"contacts"|"todos",id,targetId}`. Admin can `POST /approvals/{id}/decide` with `{decision:"approve"|"reject",note}`. Admin direct actions: `POST /contacts/{id}/merge` with `{secondaryId}` and `POST /contacts/{id}/reassign` or `/todos/{id}/reassign` with `{targetId}`. Unauthorized records and agent decisions return 404.

Regenerate embedded demo data at repository root with `node tools/export-seed-json.mjs`. Demo `Owen Chen` is assigned to Mary; `Natalie Wong` to John. Original `owner`/`who`/`by` display strings are preserved.

Deployment templates are in `deploy/`: run `setup-65.sh` on the target as root **after review**, set `/etc/axleo/env`, then run `release.sh` locally with Go and `SSHPASS` set. The scripts are provided but are not run automatically. Daily systemd timer calls `axleo backup /data/axleo/backup` (SQLite online snapshot) and retains 14 snapshots. `axleo verify <file>` checks integrity/schema and prints table counts; `axleo restore <file>` requires the service stopped and saves a copy of the existing database first. `/admin/` serves the embedded administration UI; admin-only JSON endpoints include `/admin/overview`, `/admin/approvals?status=`, `/admin/data?kind=&before=`, plus filtered `/admin/audit` and `/admin/rex-outbox`. See `docs/ops-runbook.md` for operating instructions. Do not expose the service directly to the Internet without TLS/reverse proxy.
