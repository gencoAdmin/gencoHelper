# Ubuntu/Debian deployment

Use MySQL 8, Nginx, and one Go service. Build releases on a machine with Go 1.24+ and Node 22+ and copy them to the server. The runtime uses no Docker, Redis, browser storage, or separate job service.

## 1. Prepare and build

```sh
make install
make test
make build
```

Record a release ID such as `20261012-1`. Include `VERSION`, `bin/`, `frontend/dist/`, `deploy/`, and `backend/migrations/` in the release archive. Run the Playwright suite against an installation with disposable data before publishing.

## 2. Prepare the server

```sh
sudo apt update
sudo apt install mysql-server nginx ca-certificates certbot python3-certbot-nginx
sudo useradd --system --home /var/lib/genco-finance --shell /usr/sbin/nologin genco-finance
sudo install -d -m 0755 /opt/genco-finance/releases
sudo install -d -m 0750 -o genco-finance -g genco-finance /var/lib/genco-finance
sudo install -d -m 0750 -o root -g genco-finance /etc/genco-finance
sudo install -d -m 0700 /var/backups/genco-finance
```

Keep MySQL bound to localhost. As the MySQL administrator, edit and apply `deploy/mysql-users.sql.example` to create the database and separate runtime, migration, and backup users. Runtime credentials do not have DDL permission.

Place the release under `/opt/genco-finance/releases/20261012-1`, owned by root and readable by the service. Ensure the binaries and backup scripts are executable. Copy credential examples to `/etc/genco-finance/service.env`, `migration.env`, and `backup.cnf`; replace passwords and the public domain. Set mode `0600`, owned by root. systemd reads the runtime environment before switching to its service user.

## 3. Apply migrations and activate

Before each update, take a backup. Apply migrations explicitly with the migration credentials:

```sh
sudo bash -c 'set -a; source /etc/genco-finance/migration.env; set +a; /opt/genco-finance/releases/20261012-1/bin/genco-migrate'
sudo ln -s /opt/genco-finance/releases/20261012-1 /opt/genco-finance/current.new
sudo mv -Tf /opt/genco-finance/current.new /opt/genco-finance/current
sudo install -m 0644 /opt/genco-finance/current/deploy/genco-finance.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now genco-finance
```

The included migration runner uses Goose-compatible `goose_db_version` tracking and the `-- +goose Up/Down` files. If using Goose instead, use the same migration directory and database and apply `up`. The service never applies migrations at startup. A failed MySQL DDL migration can be partially applied because MySQL DDL auto-commits; inspect the error and schema before retrying.

Set `GENCO_PUBLIC_URL` to the final HTTPS origin. Report screenshot hyperlinks depend on this configured value.

## 4. Nginx and HTTPS

Edit the supplied Nginx server name and copy it to `/etc/nginx/sites-available/genco-finance`. Enable its symlink under `sites-enabled`, remove the default site if appropriate, and check the configuration:

```sh
sudo nginx -t
sudo systemctl reload nginx
curl --fail http://127.0.0.1:8080/api/v1/health/live
curl --fail http://127.0.0.1:8080/api/v1/health/ready
```

After DNS points to the server, issue HTTPS with `sudo certbot --nginx -d finance.example.com --redirect`. Verify certificate auto-renewal using `sudo certbot renew --dry-run`. The application deliberately has no login; its shared installation permits everyone reaching it to manage products and presets. Apply the organization's network access policy to the server.

## 5. Operational checks

```sh
sudo systemctl status genco-finance
sudo journalctl -u genco-finance -n 100 --no-pager
sudo systemctl restart genco-finance
curl --fail http://127.0.0.1:8080/api/v1/health/ready
```

Open the app from two browsers. Save a product and image in one and check both in the other. Change presets and confirm the saved result stays unchanged; explicitly recalculate and inspect the new revision. Start an import, restart the service, and confirm it finishes without duplicate products. Abandoned running jobs are eligible again after their two-minute lease expires. Review the JSON logs for request IDs, errors, and request/job timing. Temporary uploads, previews, and report files are removed after 24 hours.

The service binds to `127.0.0.1:8080`, runs as a non-root user, restarts after failure, and drains on SIGTERM. Enable MySQL and Nginx at boot. Reboot the server and repeat readiness and shared-product checks.

## 6. Daily consistent backups

Copy the backup service and timer to `/etc/systemd/system`, then enable the timer:

```sh
sudo install -m 0644 /opt/genco-finance/current/deploy/genco-finance-backup.{service,timer} /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now genco-finance-backup.timer
sudo systemctl start genco-finance-backup.service
sudo systemctl list-timers genco-finance-backup.timer
```

The backup briefly stops the app so MySQL metadata and screenshot storage match exactly, takes a transaction-consistent SQL dump, archives all writable storage, and restarts the app even if backup creation fails. The backup is published from its staging directory only after all artifacts exist. Copy backups to separate storage and define retention according to operational needs; the supplied script does not delete older backups.

Run `sudo /opt/genco-finance/current/deploy/check-backup.sh /var/backups/genco-finance/TIMESTAMP`. This verifies checksums and compressed archives. A restoration drill is still required to prove that products, revisions, and screenshots can be recovered.

## 7. Restore drill or recovery

Perform a drill on a separate host using the recorded release `VERSION`. Do not restore a test backup over the active installation.

1. Verify the backup checksums and archives with `check-backup.sh`.
2. Stop the service on the restore host. Save its existing data separately if it has any.
3. Create an empty `genco_finance` database and runtime/migration users. Restore SQL with administrator credentials using `gzip -dc database.sql.gz | mysql --defaults-extra-file=/path/to/admin.cnf genco_finance`.
4. Restore files: `sudo tar -xzf storage.tar.gz -C /var/lib/genco-finance`; set ownership to `genco-finance:genco-finance` and keep the service data directory writable.
5. Activate the matching release, configure the public origin, and start the service. Do not run down migrations.
6. Check readiness, list product counts, inspect at least one product's revision history, open original and thumbnail screenshots, and download a newly generated export.
7. Restart the service and repeat the checks. Record the backup ID, restored release ID, product count, image count, and verification time in the operations log.

The installation snapshot and normalized tables are both inside the MySQL dump. `state.json` is used only when the MySQL DSN is absent; production restores must configure the DSN so a leftover local file is not used. Reports and import previews are temporary; expired artifacts may be discarded after recovery.

## 8. Rollback

Keep the previous release directory. Save the current symlink target before an update. After a failed release, stop the service, atomically point `current` to the previous release, restart, and check readiness. Preserve MySQL and image storage. Apply additive, backward-compatible migrations so older binaries can still read the database. The initial schema needs no down migration for binary rollback.

If a future schema/data change is incompatible, binary rollback alone is insufficient: restore the pre-update backup on a recovery host, validate it, and then follow the approved production recovery process.

## Verification record

The Go financial/API/file-persistence tests and Vue TypeScript production build are executable in this workspace. MySQL server access, network listeners, and Chromium are blocked by the agent environment. Complete the server, browser, reboot, HTTPS, and restore checks above on the target host and record their outcomes before production use.
