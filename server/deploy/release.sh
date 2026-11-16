#!/usr/bin/env bash
set -euo pipefail
: "${SSHPASS:?Set SSHPASS for SSH and sudo}"
cd "$(dirname "$0")/.."
sha=$(git rev-parse --short=12 HEAD)
target=dev@10.3.19.65
mkdir -p .tmp
if command -v go >/dev/null; then
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$sha" -o .tmp/axleo .
else
  docker run --rm -u "$(id -u):$(id -g)" -v "$PWD:/src" -w /src -e GOCACHE=/src/.tmp/gocache -e GOMODCACHE=/src/.tmp/gomod -e HOME=/src/.tmp \
    -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 golang:1.27 go build -ldflags "-X main.version=$sha" -o .tmp/axleo .
fi
sshpass -e scp -o StrictHostKeyChecking=accept-new .tmp/axleo "$target:axleo-$sha"
printf '%s\n' "$SSHPASS" | sshpass -e ssh "$target" "sudo -S bash -ec 'mkdir -p /opt/axleo/releases/$sha; mv /home/dev/axleo-$sha /opt/axleo/releases/$sha/axleo; chmod 755 /opt/axleo/releases/$sha/axleo; readlink -f /opt/axleo/current > /data/axleo/previous-release || true; ln -sfn /opt/axleo/releases/$sha /opt/axleo/current; systemctl restart axleo'"
if ! sshpass -e ssh "$target" 'curl -fsS --retry 5 --retry-delay 2 http://127.0.0.1:8082/api/v1/health'; then
  printf '%s\n' "$SSHPASS" | sshpass -e ssh "$target" "sudo -S bash -ec 'previous=\$(cat /data/axleo/previous-release); test -d \"\$previous\"; ln -sfn \"\$previous\" /opt/axleo/current; systemctl restart axleo'"
  exit 1
fi
