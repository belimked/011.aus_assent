#!/usr/bin/env bash
set -euo pipefail
# Build and release to the AWS Lightsail host (Sydney). 65 was retired for axleo on 2026-11-24.
# Override with AXLEO_TARGET / AXLEO_SSH_KEY if needed.
cd "$(dirname "$0")/.."
sha=$(git rev-parse --short=12 HEAD)
target=${AXLEO_TARGET:-admin@16.178.91.22}
key=${AXLEO_SSH_KEY:-$(cd .. && pwd)/aws-lightsail.pem}
ssh_opts=(-i "$key" -o BatchMode=yes -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new)
mkdir -p .tmp
if command -v go >/dev/null; then
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$sha" -o .tmp/axleo .
else
  docker run --rm -u "$(id -u):$(id -g)" -v "$PWD:/src" -w /src -e GOCACHE=/src/.tmp/gocache -e GOMODCACHE=/src/.tmp/gomod -e HOME=/src/.tmp \
    -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH=amd64 golang:1.27 go build -ldflags "-X main.version=$sha" -o .tmp/axleo .
fi
scp "${ssh_opts[@]}" .tmp/axleo "$target:axleo-$sha"
remote_home=$(ssh "${ssh_opts[@]}" "$target" 'printf %s "$HOME"')
ssh "${ssh_opts[@]}" "$target" "sudo bash -ec 'mkdir -p /opt/axleo/releases/$sha; mv $remote_home/axleo-$sha /opt/axleo/releases/$sha/axleo; chown root:root /opt/axleo/releases/$sha/axleo; chmod 755 /opt/axleo/releases/$sha/axleo; readlink -f /opt/axleo/current > /data/axleo/previous-release || true; ln -sfn /opt/axleo/releases/$sha /opt/axleo/current; systemctl restart axleo'"
if ! ssh "${ssh_opts[@]}" "$target" 'curl -fsS --retry 5 --retry-delay 2 http://127.0.0.1:8082/api/v1/health'; then
  ssh "${ssh_opts[@]}" "$target" "sudo bash -ec 'previous=\$(cat /data/axleo/previous-release); test -d \"\$previous\"; ln -sfn \"\$previous\" /opt/axleo/current; systemctl restart axleo'"
  exit 1
fi
