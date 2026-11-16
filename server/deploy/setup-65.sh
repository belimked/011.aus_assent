#!/usr/bin/env bash
set -euo pipefail
# Run on 65 as root; does not overwrite existing credentials.
id -u axleo >/dev/null 2>&1 || useradd --system --home /data/axleo --shell /usr/sbin/nologin axleo
install -d -o axleo -g axleo -m 750 /data/axleo /data/axleo/db /data/axleo/backup
install -d -o root -g root -m 755 /opt/axleo /opt/axleo/releases
install -d -o root -g axleo -m 750 /etc/axleo
if [ ! -e /etc/axleo/env ]; then
  printf 'AXLEO_ADDR=127.0.0.1:8082\nAXLEO_DB=/data/axleo/db/axleo.db\nAXLEO_BACKUP_DIR=/data/axleo/backup\nAXLEO_SEED_PASSWORD=CHANGE_ME_BEFORE_SEED\n' > /etc/axleo/env
fi
chown root:axleo /etc/axleo/env
chmod 640 /etc/axleo/env
install -m 644 "$(dirname "$0")/axleo.service" /etc/systemd/system/axleo.service
install -m 644 "$(dirname "$0")/axleo-backup.service" /etc/systemd/system/axleo-backup.service
install -m 644 "$(dirname "$0")/axleo-backup.timer" /etc/systemd/system/axleo-backup.timer
systemctl daemon-reload
systemctl enable axleo-backup.timer
