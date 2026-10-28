package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db"
	"iworkflow/internal/core/db/dbtest"
)

const legacyMigrationsDir = "../../../migrations"

func TestOpenAppliesConnectionPragmas(t *testing.T) {
	handle := dbtest.New(t)

	var journal string
	require.NoError(t, handle.QueryRow("PRAGMA journal_mode").Scan(&journal))
	require.Equal(t, "wal", journal)

	var synchronous, tempStore, foreignKeys, busy int
	require.NoError(t, handle.QueryRow("PRAGMA synchronous").Scan(&synchronous))
	require.NoError(t, handle.QueryRow("PRAGMA temp_store").Scan(&tempStore))
	require.NoError(t, handle.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys))
	require.NoError(t, handle.QueryRow("PRAGMA busy_timeout").Scan(&busy))
	require.Equal(t, 1, synchronous, "NORMAL")
	require.Equal(t, 2, tempStore, "MEMORY")
	require.Equal(t, 1, foreignKeys)
	require.Equal(t, 5000, busy)
}

func TestOpenCreatesMissingParentFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "app.db")

	handle, err := db.Open(context.Background(), path)
	require.NoError(t, err)
	require.NoError(t, handle.Close())
	require.FileExists(t, path)
}

func TestMigrateIsIdempotent(t *testing.T) {
	handle := dbtest.New(t)

	require.NoError(t, db.Migrate(context.Background(), handle))
}

func TestMigrationsRevertCleanly(t *testing.T) {
	ctx := context.Background()
	handle := dbtest.New(t)
	files, err := db.MigrationsFS()
	require.NoError(t, err)
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle, files)
	require.NoError(t, err)

	_, err = provider.DownTo(ctx, 0)
	require.NoError(t, err)
	remaining := schema(t, handle)
	for _, name := range remaining {
		require.True(t, strings.HasPrefix(name, "goose_db_version") || strings.HasPrefix(name, "sqlite_"),
			"object left after full down: %s", name)
	}
	_, err = provider.Up(ctx)
	require.NoError(t, err)
}

// The Go schema must equal the schema the Rust (Diesel) migrations produce (spec S5).
func TestSchemaMatchesLegacyDieselMigrations(t *testing.T) {
	legacy := legacyDatabase(t)
	current := dbtest.New(t)

	require.Equal(t, schemaSQL(t, legacy), schemaSQL(t, current))
}

// A database created by the Rust version is adopted without re-running migrations (spec S5).
func TestMigrateAdoptsDieselManagedDatabase(t *testing.T) {
	ctx := context.Background()
	legacy := legacyDatabase(t)
	_, err := legacy.Exec(`CREATE TABLE __diesel_schema_migrations (version VARCHAR(50) PRIMARY KEY NOT NULL, run_on TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
	require.NoError(t, err)
	for _, dir := range legacyMigrationDirs(t) {
		// Real Diesel version: folder prefix without hyphens (18 digits, e.g. 202609281634570000)
		version := strings.ReplaceAll(strings.SplitN(dir, "_", 2)[0], "-", "")
		_, err := legacy.Exec(`INSERT INTO __diesel_schema_migrations (version) VALUES (?)`, version)
		require.NoError(t, err)
	}
	_, err = legacy.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1','n','/p','main','2026-01-01')`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(ctx, legacy))

	var count int
	require.NoError(t, legacy.QueryRow(`SELECT COUNT(*) FROM workspaces`).Scan(&count))
	require.Equal(t, 1, count, "existing data must survive adoption")
	var applied int
	require.NoError(t, legacy.QueryRow(`SELECT COUNT(*) FROM goose_db_version WHERE version_id > 0`).Scan(&applied))
	require.Equal(t, len(legacyMigrationDirs(t)), applied)

	var dieselTables int
	require.NoError(t, legacy.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = '__diesel_schema_migrations'`).Scan(&dieselTables))
	require.Equal(t, 0, dieselTables, "diesel table must be dropped after adoption")
}

func TestMigrateFixesAlreadyAdopted18DigitVersions(t *testing.T) {
	ctx := context.Background()
	legacy := legacyDatabase(t)
	_, err := legacy.Exec(`CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY AUTOINCREMENT, version_id INTEGER NOT NULL, is_applied INTEGER NOT NULL, tstamp TIMESTAMP DEFAULT (datetime('now')))`)
	require.NoError(t, err)
	_, err = legacy.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, 1)`)
	require.NoError(t, err)
	for _, dir := range legacyMigrationDirs(t) {
		version := strings.ReplaceAll(strings.SplitN(dir, "_", 2)[0], "-", "")
		_, err := legacy.Exec(`INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, 1)`, version)
		require.NoError(t, err)
	}

	require.NoError(t, db.Migrate(ctx, legacy))

	var maxVersion int64
	require.NoError(t, legacy.QueryRow(`SELECT MAX(version_id) FROM goose_db_version`).Scan(&maxVersion))
	require.Less(t, maxVersion, int64(100000000000000), "all versions must be 14 digits or less")
}

func legacyMigrationDirs(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(legacyMigrationsDir)
	if err != nil {
		t.Skipf("legacy Rust migrations not available: %v", err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

func legacyDatabase(t *testing.T) *sql.DB {
	t.Helper()
	handle, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "legacy.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })
	for _, dir := range legacyMigrationDirs(t) {
		body, err := os.ReadFile(filepath.Join(legacyMigrationsDir, dir, "up.sql"))
		require.NoError(t, err)
		_, err = handle.Exec(string(body))
		require.NoError(t, err, dir)
	}
	return handle
}

func schema(t *testing.T, handle *sql.DB) []string {
	t.Helper()
	rows, err := handle.Query(`SELECT name FROM sqlite_master ORDER BY name`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		names = append(names, name)
	}
	require.NoError(t, rows.Err())
	return names
}

func schemaSQL(t *testing.T, handle *sql.DB) []string {
	t.Helper()
	rows, err := handle.Query(`SELECT type || ' ' || name || ' ' || COALESCE(sql, '') FROM sqlite_master
		WHERE name NOT LIKE 'sqlite_%' AND name NOT LIKE 'goose_%' AND name NOT LIKE '__diesel%' ORDER BY type, name`)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		out = append(out, strings.Join(strings.Fields(line), " "))
	}
	require.NoError(t, rows.Err())
	return out
}
