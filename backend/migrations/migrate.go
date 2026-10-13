package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

//go:embed *.sql
var files embed.FS

// Up applies the plain SQL migrations shipped with this release. The tracking
// table is Goose-compatible; the SQL files may also be applied with Goose.
// DDL in MySQL auto-commits. If a migration fails, investigate before rerunning.
func Up(ctx context.Context, db *sql.DB) error {
	var locked int
	if err := db.QueryRowContext(ctx, "SELECT GET_LOCK('genco_finance_migrations',30)").Scan(&locked); err != nil {
		return err
	}
	if locked != 1 {
		return fmt.Errorf("migration lock unavailable")
	}
	defer db.ExecContext(context.Background(), "SELECT RELEASE_LOCK('genco_finance_migrations')")
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS goose_db_version (id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY, version_id BIGINT NOT NULL, is_applied BOOLEAN NOT NULL, tstamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	entries, err := files.ReadDir(".")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(entry.Name(), "_", 2)[0])
		if err != nil {
			return err
		}
		var applied bool
		err = db.QueryRowContext(ctx, "SELECT is_applied FROM goose_db_version WHERE version_id=? ORDER BY id DESC LIMIT 1", version).Scan(&applied)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if applied {
			continue
		}
		content, err := files.ReadFile(entry.Name())
		if err != nil {
			return err
		}
		up := strings.SplitN(string(content), "-- +goose Down", 2)[0]
		up = strings.ReplaceAll(up, "-- +goose Up", "")
		for _, statement := range strings.Split(up, ";") {
			if strings.TrimSpace(statement) == "" {
				continue
			}
			if _, err = db.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migration %s failed: %w", entry.Name(), err)
			}
		}
		if _, err = db.ExecContext(ctx, "INSERT INTO goose_db_version(version_id,is_applied) VALUES(?,TRUE)", version); err != nil {
			return err
		}
	}
	return nil
}
