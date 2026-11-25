#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
export TMPDIR="$ROOT/.tmp"
mkdir -p "$TMPDIR/web-e2e"
RUN=$(mktemp -d "$TMPDIR/web-e2e/run.XXXXXX")
export AXLEO_DB="$RUN/axleo.db" AXLEO_ADDR=127.0.0.1:8083
export AXLEO_SEED_PASSWORD=web-e2e-password-2026
cleanup() { if [[ -n "${PID:-}" ]]; then kill "$PID" 2>/dev/null || true; wait "$PID" 2>/dev/null || true; fi; rm -rf "$RUN"; }
trap cleanup EXIT INT TERM
(cd "$ROOT/web" && npm run build)
(cd "$ROOT/server" && go build -o "$RUN/axleo" . && "$RUN/axleo" seed --with-demo)
"$RUN/axleo" serve > "$RUN/server.log" 2>&1 & PID=$!
for i in {1..40}; do
  if curl -fsS http://127.0.0.1:8083/api/v1/health >/dev/null 2>&1; then break; fi
  if ! kill -0 "$PID" 2>/dev/null; then echo 'Server exited:'; tail -80 "$RUN/server.log"; exit 1; fi
  sleep 1
done
curl -fsS http://127.0.0.1:8083/api/v1/health >/dev/null
node "$ROOT/web/e2e/scripts/bootstrap.js"
echo 'Web E2E server ready on 127.0.0.1:8083'
wait "$PID"
