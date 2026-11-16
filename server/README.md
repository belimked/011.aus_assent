# AxleoCollect API (B1)

Go 1.27, SQLite (pure Go), standard-library HTTP server. From `server/`:

```sh
go test ./... && go vet ./...
go build -ldflags '-X main.version=local' -o axleo .
export AXLEO_DB=./axleo.db AXLEO_SEED_PASSWORD='a-long-initial-password'
./axleo migrate
./axleo seed
./axleo serve
```

`AXLEO_ADDR` defaults to `127.0.0.1:8082`; `AXLEO_DB` to `./axleo.db`; migration snapshots use `AXLEO_BACKUP_DIR` (default `backup/` beside the DB). `axleo passwd <email>` reads the replacement password from stdin and revokes existing sessions. Without `AXLEO_SEED_PASSWORD`, the first seed prints a generated password **once**; save it. `seed` is repeatable and does not overwrite existing accounts or records.

All protected endpoints use `Authorization: Bearer <token>`; `/api/v1/health` is public. Collections return JSON arrays, `?since=<RFC3339Nano updated_at>` includes updated and soft-deleted records; PATCH requires `version`, with stale updates returning `409` and `current`. Admin-only endpoints are under `/api/v1/admin/`.

Regenerate embedded demo data at repository root with `node tools/export-seed-json.mjs`. Demo `Owen Chen` is assigned to Mary; `Natalie Wong` to John. Original `owner`/`who`/`by` display strings are preserved.

Deployment templates are in `deploy/`: run `setup-65.sh` on the target as root **after review**, set `/etc/axleo/env`, then run `release.sh` locally with Go and `SSHPASS` set. The scripts are provided but are not run automatically. Daily systemd timer uses `sqlite3 .backup` and retains 14 snapshots. Do not expose the service directly to the Internet without TLS/reverse proxy.
