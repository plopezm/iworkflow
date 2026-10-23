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
// database was created by the Rust version and goose has not run yet.
func adoptDieselHistory(ctx context.Context, handle *sql.DB) error {
	hasDiesel, err := tableExists(ctx, handle, dieselTable)
	if err != nil {
		return err
	}
	hasGoose, err := tableExists(ctx, handle, gooseTable)
	if err != nil {
		return err
	}
	if !hasDiesel || hasGoose {
		return nil
	}
	tx, err := handle.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin adoption: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
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
	if _, err := tx.ExecContext(ctx, `INSERT INTO `+gooseTable+` (version_id, is_applied)
		SELECT CAST(version AS INTEGER), 1 FROM `+dieselTable+` ORDER BY version`); err != nil {
		return fmt.Errorf("adopt diesel versions: %w", err)
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
