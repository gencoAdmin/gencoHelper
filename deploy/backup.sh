#!/usr/bin/env bash
set -euo pipefail
umask 077
backup_root=${GENCO_BACKUP_DIR:-/var/backups/genco-finance}
backup_stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_stage="$backup_root/.staging-$backup_stamp"
mkdir -p "$backup_stage"
was_active=false
if systemctl is-active --quiet genco-finance; then
    was_active=true
    systemctl stop genco-finance
fi
cleanup() {
    if "$was_active"; then systemctl start genco-finance; fi
}
trap cleanup EXIT
mysqldump --defaults-extra-file=/etc/genco-finance/backup.cnf \
    --single-transaction --no-tablespaces --set-gtid-purged=OFF \
    --routines=false genco_finance > "$backup_stage/database.sql"
tar -C /var/lib/genco-finance -czf "$backup_stage/storage.tar.gz" .
cp -L /opt/genco-finance/current/VERSION "$backup_stage/VERSION"
gzip "$backup_stage/database.sql"
(cd "$backup_stage" && sha256sum database.sql.gz storage.tar.gz > SHA256SUMS)
mv "$backup_stage" "$backup_root/$backup_stamp"
echo "Backup completed: $backup_root/$backup_stamp"
