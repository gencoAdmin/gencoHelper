#!/usr/bin/env bash
set -euo pipefail
if [ "$#" -ne 1 ]; then echo 'Usage: check-backup.sh /path/to/backup-directory' >&2; exit 2; fi
backup_check_dir=$(realpath "$1")
cd "$backup_check_dir"
sha256sum -c SHA256SUMS
gzip -t database.sql.gz
gzip -t storage.tar.gz
tar -tzf storage.tar.gz >/dev/null
echo 'Checksums and archives verified. Complete a restore drill as documented in the runbook.'
