package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// MigrationsFS exposes the embedded migrations (used by sqlc and tests).
func MigrationsFS() (fs.FS, error) {
	sub, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return nil, fmt.Errorf("sub migrations fs: %w", err)
	}
	return sub, nil
}

const (
	dieselTable = "__diesel_schema_migrations"
	gooseTable  = "goose_db_version"
)

// Migrate applies every pending migration. A database previously managed by the Rust
// (Diesel) version is adopted: its applied versions are recorded in goose's table first.
func Migrate(ctx context.Context, handle *sql.DB) error {
	if err := adoptDieselHistory(ctx, handle); err != nil {
		return err
	}
	files, err := MigrationsFS()
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle, files)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// adoptDieselHistory copies Diesel's applied versions into goose's version table when the
// database was created by the Rust version and normalizes version numbers to goose's 14-digit format.
// It also cleans up any legacy 18-digit versions and drops the Diesel table so no Diesel traces remain.
func adoptDieselHistory(ctx context.Context, handle *sql.DB) error {
	hasDiesel, err := tableExists(ctx, handle, dieselTable)
	if err != nil {
		return err
	}
	hasGoose, err := tableExists(ctx, handle, gooseTable)
	if err != nil {
		return err
	}
	if !hasDiesel && !hasGoose {
		return nil
	}

	tx, err := handle.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin adoption: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if !hasGoose {
		if _, err := tx.ExecContext(ctx, `CREATE TABLE `+gooseTable+` (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		version_id INTEGER NOT NULL,
		is_applied INTEGER NOT NULL,
		tstamp TIMESTAMP DEFAULT (datetime('now')))`); err != nil {
			return fmt.Errorf("create goose table: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+gooseTable+` (version_id, is_applied) VALUES (0, 1)`); err != nil {
			return fmt.Errorf("seed goose table: %w", err)
		}
	}

	if hasDiesel {
		// Diesel recorded versions with 18 digits (e.g. YYYYMMDDHHMMSS0000) or hyphens.
		// Normalize to Goose's 14-digit (YYYYMMDDHHMMSS) integer format.
		if _, err := tx.ExecContext(ctx, `INSERT INTO `+gooseTable+` (version_id, is_applied)
		SELECT DISTINCT CAST(SUBSTR(REPLACE(d.version, '-', ''), 1, 14) AS INTEGER), 1
		FROM `+dieselTable+` d
		WHERE CAST(SUBSTR(REPLACE(d.version, '-', ''), 1, 14) AS INTEGER) > 0
		  AND NOT EXISTS (
		      SELECT 1 FROM `+gooseTable+` g
		      WHERE g.version_id = CAST(SUBSTR(REPLACE(d.version, '-', ''), 1, 14) AS INTEGER)
		  )
		ORDER BY d.version`); err != nil {
			return fmt.Errorf("adopt diesel versions: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS `+dieselTable); err != nil {
			return fmt.Errorf("drop diesel table: %w", err)
		}
	}

	// Fix any previously-adopted 18-digit versions in gooseTable (e.g. from an earlier migration attempt).
	if _, err := tx.ExecContext(ctx, `UPDATE `+gooseTable+`
		SET version_id = CAST(SUBSTR(CAST(version_id AS TEXT), 1, 14) AS INTEGER)
		WHERE version_id > 100000000000000`); err != nil {
		return fmt.Errorf("normalize legacy goose version ids: %w", err)
	}

	// Remove any duplicate version_id rows if present.
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+gooseTable+`
		WHERE id NOT IN (
		    SELECT MIN(id) FROM `+gooseTable+` GROUP BY version_id
		)`); err != nil {
		return fmt.Errorf("deduplicate goose version ids: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit adoption: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, handle *sql.DB, name string) (bool, error) {
	var count int
	err := handle.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check table %s: %w", name, err)
	}
	return count > 0, nil
}
