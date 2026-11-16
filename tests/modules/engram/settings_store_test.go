package engram_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db"
	"iworkflow/internal/modules/engram/infrastructure/adapters/outbound/persistence"
)

func TestSettingsStoreRoundTripsTheAddress(t *testing.T) {
	database := openDatabase(t)
	store := persistence.NewSettingsStore(database)

	got, err := store.SavedURL(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "", got, "nothing saved yet")

	require.NoError(t, store.SaveURL(t.Context(), "http://127.0.0.1:9000"))
	got, err = store.SavedURL(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "http://127.0.0.1:9000", got)

	require.NoError(t, store.SaveURL(t.Context(), ""))
	got, err = store.SavedURL(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "", got, "blank means the default again")
}

func openDatabase(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "iwf.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(t.Context(), database))
	t.Cleanup(func() { _ = database.Close() })
	return database
}
