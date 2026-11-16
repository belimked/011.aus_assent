#!/usr/bin/env bash
# Run on 65 as root. Only the isolated drill DB is restored; production is never stopped.
set -euo pipefail
[[ $EUID -eq 0 ]] || { echo 'Run as root' >&2; exit 1; }
: "${AXLEO_ADMIN_EMAIL:?Set AXLEO_ADMIN_EMAIL}"
: "${AXLEO_ADMIN_PASSWORD:?Set AXLEO_ADMIN_PASSWORD}"
source /etc/axleo/env
bin=/opt/axleo/current/axleo
live=http://127.0.0.1:8082/api/v1
base=/data/axleo/drill
mkdir -p "$base"
chmod 700 "$base"
work=$(mktemp -d "$base/run.XXXXXXXX")
cleanup() { [[ -z ${pid:-} ]] || { kill "$pid" 2>/dev/null || :; wait "$pid" 2>/dev/null || :; }; rm -rf -- "$work"; }
trap cleanup EXIT
snapshot=$(ls -1t /data/axleo/backup/axleo-*.db | head -n 1)
"$bin" verify "$snapshot" > "$work/verify.txt"
export AXLEO_DB="$work/drill.db" AXLEO_BACKUP_DIR="$work/backup" AXLEO_ADDR=127.0.0.1:8092
"$bin" migrate
"$bin" restore "$snapshot"
"$bin" serve > "$work/server.log" 2>&1 & pid=$!
for i in {1..30}; do curl -fsS http://127.0.0.1:8092/api/v1/health > "$work/health.json" && break || sleep 1; done
jq -e '.ok == true' "$work/health.json" >/dev/null
login() { curl -fsS -H 'Content-Type: application/json' -d "$(jq -nc --arg e "$AXLEO_ADMIN_EMAIL" --arg p "$AXLEO_ADMIN_PASSWORD" '{email:$e,password:$p,device:"restore-drill"}')" "$1/auth/login" | jq -r '.token'; }
live_token=$(login "$live")
drill_token=$(login http://127.0.0.1:8092/api/v1)
[[ $live_token != null && $drill_token != null ]] || { echo 'Admin login failed' >&2; exit 1; }
curl -fsS -H "Authorization: Bearer $live_token" "$live/admin/overview" | jq -S '{counts,users}' > "$work/live.json"
curl -fsS -H "Authorization: Bearer $drill_token" http://127.0.0.1:8092/api/v1/admin/overview | jq -S '{counts,users}' > "$work/drill.json"
diff -u "$work/live.json" "$work/drill.json"
echo "Drill OK: $snapshot; health and record counts match (at comparison time)."
