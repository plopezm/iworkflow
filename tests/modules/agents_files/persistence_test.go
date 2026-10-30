package agents_files_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/persistence"
)

func newStoredFile(t *testing.T, description string, path string) models.AgentsFile {
	t.Helper()
	id, err := models.NewAgentsFileID()
	require.NoError(t, err)
	return models.AgentsFile{
		ID:          id,
		Description: description,
		Path:        path,
		CreatedAt:   time.Now().UTC(),
	}
}

func TestRepositorySavesAndFindsById(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewAgentsFileRepository(dbtest.New(t))
	original := newStoredFile(t, "reglas", "/data/a.md")

	require.NoError(t, repository.Save(ctx, original))

	found, err := repository.FindByID(ctx, original.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	require.Equal(t, original.ID, found.ID)
	require.Equal(t, original.Description, found.Description)
	require.Equal(t, original.Path, found.Path)
	require.True(t, original.CreatedAt.Equal(found.CreatedAt), "created_at %v != %v", original.CreatedAt, found.CreatedAt)

	missing, err := repository.FindByID(ctx, mustNewID(t))
	require.NoError(t, err)
	require.Nil(t, missing)
}

func TestRepositoryListsAllFilesOldestFirst(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewAgentsFileRepository(dbtest.New(t))
	older := newStoredFile(t, "a", "/data/a.md")
	newer := newStoredFile(t, "b", "/data/b.md")
	newer.CreatedAt = older.CreatedAt.Add(time.Minute)
	require.NoError(t, repository.Save(ctx, newer))
	require.NoError(t, repository.Save(ctx, older))

	listed, err := repository.List(ctx)

	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, older.ID, listed[0].ID)
	require.Equal(t, newer.ID, listed[1].ID)
}

func TestRepositoryRejectsDuplicatePath(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewAgentsFileRepository(dbtest.New(t))
	require.NoError(t, repository.Save(ctx, newStoredFile(t, "a", "/data/same.md")))

	err := repository.Save(ctx, newStoredFile(t, "b", "/data/same.md"))

	require.ErrorIs(t, err, agentserrors.Repository(""))
}

func TestRepositoryDeleteRemovesAndReportsMissing(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewAgentsFileRepository(dbtest.New(t))
	saved := newStoredFile(t, "a", "/data/a.md")
	require.NoError(t, repository.Save(ctx, saved))

	require.NoError(t, repository.Delete(ctx, saved.ID))
	again := repository.Delete(ctx, saved.ID)

	found, err := repository.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	require.Nil(t, found)
	require.Equal(t, agentserrors.NotFound(saved.ID.String()), again)
}

// Rows written by the Rust (Diesel) version must read correctly, and rows written here must use the
// same text format, so both versions can share one database during the migration.
func TestRepositoryReadsAndWritesTheRustTimestampFormat(t *testing.T) {
	ctx := context.Background()
	handle := dbtest.New(t)
	repository := persistence.NewAgentsFileRepository(handle)
	const rustRowID = "6f1c2a3e-0000-4000-8000-0000000000aa"
	mustExec(t, handle, `INSERT INTO agents_files (id, description, path, created_at) VALUES (?, 'legacy', '/data/legacy.md', '2026-01-02 03:04:05')`, rustRowID)

	legacy, err := repository.FindByID(ctx, mustParseID(t, rustRowID))
	require.NoError(t, err)
	require.NotNil(t, legacy)
	require.Equal(t, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), legacy.CreatedAt)

	written := newStoredFile(t, "new", "/data/new.md")
	written.CreatedAt = time.Date(2026, 1, 2, 3, 4, 5, 123456789, time.UTC)
	require.NoError(t, repository.Save(ctx, written))
	// CAST to TEXT reads the raw stored text; a plain column read would let the driver convert it.
	var stored string
	require.NoError(t, handle.QueryRowContext(ctx, `SELECT CAST(created_at AS TEXT) FROM agents_files WHERE id = ?`, written.ID.String()).Scan(&stored))
	require.Equal(t, "2026-01-02 03:04:05.123456789", stored)
}

func TestRepositoryReportsMalformedStoredIdAsRepositoryError(t *testing.T) {
	ctx := context.Background()
	handle := dbtest.New(t)
	repository := persistence.NewAgentsFileRepository(handle)
	mustExec(t, handle, `INSERT INTO agents_files (id, description, path, created_at) VALUES ('not-a-uuid', 'bad', '/data/bad.md', '2026-01-02 03:04:05')`)

	_, err := repository.List(ctx)

	require.ErrorIs(t, err, agentserrors.Repository(""))
	require.ErrorContains(t, err, "id inválido not-a-uuid")
}

func mustNewID(t *testing.T) models.AgentsFileID {
	t.Helper()
	id, err := models.NewAgentsFileID()
	require.NoError(t, err)
	return id
}

func mustParseID(t *testing.T, value string) models.AgentsFileID {
	t.Helper()
	id, err := models.ParseAgentsFileID(value)
	require.NoError(t, err)
	return id
}

func mustExec(t *testing.T, handle *sql.DB, query string, args ...any) {
	t.Helper()
	_, err := handle.ExecContext(context.Background(), query, args...)
	require.NoError(t, err)
}
