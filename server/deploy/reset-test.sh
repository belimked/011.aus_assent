#!/usr/bin/env bash
set -euo pipefail
: "${SSHPASS:?Set SSHPASS for SSH and sudo}"
if (( $# > 1 )) || { (( $# == 1 )) && [[ $1 != --rex-import ]]; }; then
  echo 'usage: SSHPASS=... bash server/deploy/reset-test.sh [--rex-import]' >&2
  exit 2
fi
mode=${1:-demo}
target=dev@10.3.19.65
on_local_exit() {
  rc=$?
  if (( rc != 0 )); then
    # Covers SSH/sudo failures before the remote EXIT trap could be installed.
    if ! printf '%s\n' "$SSHPASS" | sshpass -e ssh "$target" 'sudo -S systemctl start axleo'; then
      echo 'Could not confirm axleo.service restarted; check it on 65 immediately.' >&2
    fi
    echo 'Rollback: stop axleo on 65, then run axleo restore <Verified backup path printed above> as the axleo user with /etc/axleo/env loaded.' >&2
  fi
}
trap on_local_exit EXIT
# sudo -S consumes the first stdin line; the remainder is a literal remote script.
# Neither the sudo password nor /etc/axleo/env is echoed to stdout.
{
  printf '%s\n' "$SSHPASS"
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
} | sshpass -e ssh -o StrictHostKeyChecking=accept-new "$target" "sudo -S bash -s -- '$mode'"
