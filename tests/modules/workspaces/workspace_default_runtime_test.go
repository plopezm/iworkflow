package workspaces_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db"
	"iworkflow/internal/core/db/dbtest"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
)

// previousMigrationVersion is the version of the migration before the one that adds
// workspaces.default_runtime (20261116100000_add_workspace_default_runtime.sql).
const previousMigrationVersion int64 = 20261112110000

func stringPtr(value string) *string {
	return &value
}

func TestWorkspaceRepositoryRoundTripsDefaultRuntime(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	withRuntime := newStoredWorkspace(t, "con", "/repos/con")
	withRuntime.DefaultRuntime = stringPtr("codex")
	withoutRuntime := newStoredWorkspace(t, "sin", "/repos/sin")
	require.NoError(t, repository.Save(ctx, withRuntime))
	require.NoError(t, repository.Save(ctx, withoutRuntime))

	foundWith, err := repository.FindByID(ctx, withRuntime.ID)
	require.NoError(t, err)
	require.NotNil(t, foundWith)
	require.NotNil(t, foundWith.DefaultRuntime)
	assert.Equal(t, "codex", *foundWith.DefaultRuntime)

	foundWithout, err := repository.FindByID(ctx, withoutRuntime.ID)
	require.NoError(t, err)
	require.NotNil(t, foundWithout)
	assert.Nil(t, foundWithout.DefaultRuntime, "a workspace saved without a default runtime reads nil (system agent)")

	all, err := repository.List(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)
	for _, listed := range all {
		switch listed.ID {
		case withRuntime.ID:
			require.NotNil(t, listed.DefaultRuntime)
			assert.Equal(t, "codex", *listed.DefaultRuntime)
		case withoutRuntime.ID:
			assert.Nil(t, listed.DefaultRuntime)
		default:
			t.Fatalf("unexpected workspace %s in list", listed.ID)
		}
	}
}

func TestWorkspaceRepositorySavesBlankDefaultRuntimeAsSystemAgent(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	saved := newStoredWorkspace(t, "vacio", "/repos/vacio")
	saved.DefaultRuntime = stringPtr("")
	require.NoError(t, repository.Save(ctx, saved))

	found, err := repository.FindByID(ctx, saved.ID)

	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Nil(t, found.DefaultRuntime, "an empty runtime is stored as NULL, the system agent")
}

func TestWorkspaceRepositoryUpdateSetsAndClearsDefaultRuntime(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	saved := newStoredWorkspace(t, "a", "/repos/a")
	require.NoError(t, repository.Save(ctx, saved))

	set := saved
	set.DefaultRuntime = stringPtr("claude")
	require.NoError(t, repository.Update(ctx, set))
	afterSet, err := repository.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	require.NotNil(t, afterSet)
	require.NotNil(t, afterSet.DefaultRuntime)
	assert.Equal(t, "claude", *afterSet.DefaultRuntime)

	cleared := set
	cleared.DefaultRuntime = nil
	require.NoError(t, repository.Update(ctx, cleared))
	afterClear, err := repository.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	require.NotNil(t, afterClear)
	assert.Nil(t, afterClear.DefaultRuntime, "clearing the default runtime stores NULL again")
}

func TestMigrationAddsNullableDefaultRuntimeAndExistingRowsReadNil(t *testing.T) {
	ctx := context.Background()
	handle, err := db.Open(ctx, filepath.Join(t.TempDir(), "migrated.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })

	migrations, err := db.MigrationsFS()
	require.NoError(t, err)
	provider, err := goose.NewProvider(goose.DialectSQLite3, handle, migrations)
	require.NoError(t, err)
	_, err = provider.UpTo(ctx, previousMigrationVersion)
	require.NoError(t, err, "migrations before the default runtime column apply")
	insertWorkspaceRowBeforeMigration(t, handle)

	require.NoError(t, db.Migrate(ctx, handle), "the pending migration applies on top of existing data")

	var notNull int
	require.NoError(t, handle.QueryRowContext(ctx,
		`SELECT "notnull" FROM pragma_table_info('workspaces') WHERE name = 'default_runtime'`).Scan(&notNull))
	assert.Equal(t, 0, notNull, "workspaces.default_runtime is nullable")

	repository := persistence.NewWorkspaceRepository(handle)
	id, err := models.ParseWorkspaceID("3f2b1c4e-5d6a-4b7c-8d9e-0f1a2b3c4d5e")
	require.NoError(t, err)
	found, err := repository.FindByID(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "antiguo", found.Name)
	assert.Nil(t, found.DefaultRuntime, "a row written before the migration reads the system agent")
}

// insertWorkspaceRowBeforeMigration writes a workspace with the schema that existed before the
// default runtime column was added.
func insertWorkspaceRowBeforeMigration(t *testing.T, handle *sql.DB) {
	t.Helper()
	_, err := handle.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		"3f2b1c4e-5d6a-4b7c-8d9e-0f1a2b3c4d5e", "antiguo", "/repos/antiguo", "main", "2026-01-02 03:04:05")
	require.NoError(t, err)
}
