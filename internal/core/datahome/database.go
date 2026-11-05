package datahome

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"iworkflow/internal/core/db"
)

// ErrDatabaseBusy is returned when another process still holds the database open.
var ErrDatabaseBusy = errors.New("the database is in use by another process")

// Prefix maps the absolute paths below From to the same paths below To.
type Prefix struct {
	From string
	To   string
}

// RewriteReport counts what RewritePaths changed.
type RewriteReport struct {
	Rewritten int
}

// storedPath is a column that holds an absolute path inside the data folder. Attachments are stored
// relative to their folder and the workspaces point at the user's repositories, so neither is listed.
type storedPath struct{ table, column string }

var storedPaths = []storedPath{
	{table: "tasks", column: "worktree_path"},
	{table: "agents_files", column: "path"},
}

// Quiesce folds the write-ahead log into the database file and closes it, so the folder holds one
// self-contained workspaces.db that can be copied as a plain file. It fails when another process still
// has the database open.
//
// The raw SQL in this file is bootstrap maintenance that runs before the application is composed (no
// sqlc layer exists for it); every value is a bound parameter and identifiers come from fixed lists.
func Quiesce(ctx context.Context, path string) error {
	handle, err := db.Open(ctx, path)
	if err != nil {
		return fmt.Errorf("open the database: %w", err)
	}
	var busy, logFrames, checkpointed int
	err = handle.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed)
	if closeErr := handle.Close(); err == nil && closeErr != nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("checkpoint the database: %w", err)
	}
	if busy != 0 {
		return ErrDatabaseBusy
	}
	if info, statErr := os.Stat(path + "-wal"); statErr == nil && info.Size() > 0 {
		return ErrDatabaseBusy
	}
	return nil
}

// CompareDatabases checks that b is a faithful copy of a: it passes the integrity check and every table
// has the same number of rows. It opens both read-write because SQLite needs it for the WAL files, but
// changes nothing.
func CompareDatabases(ctx context.Context, a string, b string) error {
	left, right, err := openBoth(ctx, a, b)
	if err != nil {
		return err
	}
	defer func() { _ = left.Close() }()  // read-only use: nothing to recover from a close error
	defer func() { _ = right.Close() }() // idem
	if err := integrityOK(ctx, right); err != nil {
		return err
	}
	tables, err := userTables(ctx, left)
	if err != nil {
		return err
	}
	for _, table := range tables {
		want, err := countRows(ctx, left, table)
		if err != nil {
			return err
		}
		got, err := countRows(ctx, right, table)
		if err != nil {
			return err
		}
		if want != got {
			return fmt.Errorf("%w: table %s has %d rows, the original %d", ErrVerify, table, got, want)
		}
	}
	return nil
}

func openBoth(ctx context.Context, a string, b string) (*sql.DB, *sql.DB, error) {
	left, err := db.Open(ctx, a)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", a, err)
	}
	right, err := db.Open(ctx, b)
	if err != nil {
		_ = left.Close() // the second open already failed: report that error
		return nil, nil, fmt.Errorf("open %s: %w", b, err)
	}
	return left, right, nil
}

func integrityOK(ctx context.Context, handle *sql.DB) error {
	var verdict string
	if err := handle.QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&verdict); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if verdict != "ok" {
		return fmt.Errorf("%w: integrity check says %q", ErrVerify, verdict)
	}
	return nil
}

func userTables(ctx context.Context, handle *sql.DB) ([]string, error) {
	rows, err := handle.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	defer func() { _ = rows.Close() }() // the rows are read to the end below
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("read table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}
	return tables, nil
}

func countRows(ctx context.Context, handle *sql.DB, table string) (int, error) {
	var count int
	query := `SELECT count(*) FROM "` + strings.ReplaceAll(table, `"`, `""`) + `"`
	if err := handle.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("count rows of %s: %w", table, err)
	}
	return count, nil
}

// RewritePaths turns every stored path below a prefix's From into the same path below its To, in one
// transaction. A path only matches when From is followed by a separator (or ends there), so a sibling
// folder that merely starts with the same name is left alone, and "%" or "_" in a folder name are plain
// characters. Running it twice changes nothing the second time.
func RewritePaths(ctx context.Context, path string, prefixes []Prefix) (RewriteReport, error) {
	handle, err := db.Open(ctx, path)
	if err != nil {
		return RewriteReport{}, fmt.Errorf("open the database: %w", err)
	}
	report, err := rewriteIn(ctx, handle, prefixes)
	if closeErr := handle.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("close the database: %w", closeErr)
	}
	return report, err
}

func rewriteIn(ctx context.Context, handle *sql.DB, prefixes []Prefix) (RewriteReport, error) {
	tables, err := userTables(ctx, handle)
	if err != nil {
		return RewriteReport{}, err
	}
	tx, err := handle.BeginTx(ctx, nil)
	if err != nil {
		return RewriteReport{}, fmt.Errorf("begin the rewrite: %w", err)
	}
	var report RewriteReport
	for _, stored := range storedPaths {
		if !slices.Contains(tables, stored.table) {
			continue
		}
		for _, prefix := range prefixes {
			changed, err := rewriteColumn(ctx, tx, stored, prefix)
			if err != nil {
				_ = tx.Rollback() // the rewrite already failed: report that error
				return RewriteReport{}, err
			}
			report.Rewritten += changed
		}
	}
	if err := tx.Commit(); err != nil {
		return RewriteReport{}, fmt.Errorf("commit the rewrite: %w", err)
	}
	return report, nil
}

func rewriteColumn(ctx context.Context, tx *sql.Tx, stored storedPath, prefix Prefix) (int, error) {
	query := fmt.Sprintf(`UPDATE %[1]s SET %[2]s = ?1 || substr(%[2]s, length(?2) + 1)
		WHERE substr(%[2]s, 1, length(?2)) = ?2
		AND (length(%[2]s) = length(?2) OR substr(%[2]s, length(?2) + 1, 1) IN ('/', '\'))`, stored.table, stored.column)
	result, err := tx.ExecContext(ctx, query, prefix.To, prefix.From)
	if err != nil {
		return 0, fmt.Errorf("rewrite %s.%s: %w", stored.table, stored.column, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count rewritten rows of %s.%s: %w", stored.table, stored.column, err)
	}
	return int(changed), nil
}
