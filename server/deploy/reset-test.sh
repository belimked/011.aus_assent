#!/usr/bin/env bash
set -euo pipefail
# Runs against the Lightsail host (65 retired for axleo on 2026-11-24). Override with AXLEO_TARGET / AXLEO_SSH_KEY.
if (( $# > 1 )) || { (( $# == 1 )) && [[ $1 != --rex-import ]]; }; then
  echo 'usage: bash server/deploy/reset-test.sh [--rex-import]' >&2
  exit 2
fi
mode=${1:-demo}
target=${AXLEO_TARGET:-admin@16.178.91.22}
key=${AXLEO_SSH_KEY:-$(cd "$(dirname "$0")/../.." && pwd)/aws-lightsail.pem}
ssh_opts=(-i "$key" -o BatchMode=yes -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new)
on_local_exit() {
  rc=$?
  if (( rc != 0 )); then
    # Covers SSH/sudo failures before the remote EXIT trap could be installed.
    if ! ssh "${ssh_opts[@]}" "$target" 'sudo systemctl start axleo'; then
      echo 'Could not confirm axleo.service restarted; check it on the server immediately.' >&2
    fi
    echo 'Rollback: stop axleo on the server, then run axleo restore <Verified backup path printed above> as the axleo user with /etc/axleo/env loaded.' >&2
  fi
}
trap on_local_exit EXIT
# The remote script is sent literally on stdin; /etc/axleo/env is never echoed to stdout.
{
  while IFS= read -r line; do printf '%s\n' "$line"; done <<'EOF'
set -euo pipefail
mode=$1
snapshot=''
on_exit() {
  rc=$?
  trap - EXIT
  set +e
  # Always try to start the service, even when reset or import failed.
  systemctl start axleo
  start_rc=$?
  if (( start_rc != 0 )); then echo 'ERROR: could not start axleo.service' >&2; rc=1; fi
  if (( rc == 0 )); then
    curl -fsS --retry 5 --retry-delay 2 http://127.0.0.1:8082/api/v1/health
    health_rc=$?
    if (( health_rc != 0 )); then echo 'ERROR: health check failed' >&2; rc=1; fi
  fi
  if (( rc != 0 )); then
    echo 'Reset/import failed; service restart attempted.' >&2
    if [[ -n $snapshot ]]; then
      echo "Rollback (stop service first): sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; /opt/axleo/current/axleo restore $snapshot'" >&2
    else
      echo 'No verified snapshot captured; check reset output and /data/axleo/backup before restoring.' >&2
    fi
  fi
  exit "$rc"
}
trap on_exit EXIT
if [[ $mode == --rex-import ]]; then
  set -a
  . /etc/axleo/env
  set +a
  if [[ ${REX_ACCOUNT_ID:-} != 2057 ]]; then
    echo 'Refusing --rex-import: REX_ACCOUNT_ID is not 2057 (sandbox).' >&2
    exit 1
  fi
fi
systemctl stop axleo
args=()
if [[ $mode == --rex-import ]]; then args=(--rex-import); fi
# Capture the verified snapshot even when the reset succeeds but the import fails.
set +e
output=$(sudo -u axleo bash -c 'set -a; . /etc/axleo/env; set +a; cd /data/axleo; exec /opt/axleo/current/axleo reset-data --yes "$@"' _ "${args[@]}" 2>&1)
reset_rc=$?
set -e
printf '%s\n' "$output"
if [[ $output =~ Verified[[:space:]]backup:[[:space:]]([^[:space:]]+) ]]; then snapshot=${BASH_REMATCH[1]}; fi
if (( reset_rc != 0 )); then exit "$reset_rc"; fi
echo 'Reset and optional Rex import complete; restarting service and checking health.'
EOF
} | ssh "${ssh_opts[@]}" "$target" "sudo bash -s -- '$mode'"
