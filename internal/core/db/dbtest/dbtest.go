// Package dbtest builds throw-away migrated SQLite databases for integration tests.
package dbtest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"iworkflow/internal/core/db"
)

// New returns a migrated SQLite database in a temporary folder, closed when the test ends.
func New(t testing.TB) *sql.DB {
	t.Helper()
	ctx := context.Background()
	handle, err := db.Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = handle.Close() })
	if err := db.Migrate(ctx, handle); err != nil {
		t.Fatalf("migrate test database: %v", err)
	}
	return handle
}
