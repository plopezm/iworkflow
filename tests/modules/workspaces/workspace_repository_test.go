package workspaces_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/persistence"
)

// newStoredWorkspace builds a workspace whose timestamp survives the round trip (microsecond precision).
func newStoredWorkspace(t *testing.T, name string, path string) models.Workspace {
	t.Helper()
	id, err := models.NewWorkspaceID()
	require.NoError(t, err)
	return models.Workspace{
		ID:         id,
		Name:       name,
		Path:       path,
		BaseBranch: "main",
		CreatedAt:  time.Now().UTC().Truncate(time.Microsecond),
	}
}

func requireSameWorkspace(t *testing.T, expected models.Workspace, actual *models.Workspace) {
	t.Helper()
	require.NotNil(t, actual)
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Name, actual.Name)
	assert.Equal(t, expected.Path, actual.Path)
	assert.Equal(t, expected.BaseBranch, actual.BaseBranch)
	assert.True(t, expected.CreatedAt.Equal(actual.CreatedAt), "created_at %v != %v", expected.CreatedAt, actual.CreatedAt)
}

func TestWorkspaceRepositorySavesAndFindsByIDAndPath(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	original := newStoredWorkspace(t, "demo", "/repos/demo")

	require.NoError(t, repository.Save(ctx, original))

	byID, err := repository.FindByID(ctx, original.ID)
	require.NoError(t, err)
	requireSameWorkspace(t, original, byID)

	byPath, err := repository.FindByPath(ctx, "/repos/demo")
	require.NoError(t, err)
	requireSameWorkspace(t, original, byPath)
}

func TestWorkspaceRepositoryReturnsNilWhenNotFound(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	missing, err := models.NewWorkspaceID()
	require.NoError(t, err)

	byID, err := repository.FindByID(ctx, missing)
	require.NoError(t, err)
	assert.Nil(t, byID)

	byPath, err := repository.FindByPath(ctx, "/repos/nowhere")
	require.NoError(t, err)
	assert.Nil(t, byPath)
}

func TestWorkspaceRepositoryListsAllWorkspaces(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	require.NoError(t, repository.Save(ctx, newStoredWorkspace(t, "a", "/repos/a")))
	require.NoError(t, repository.Save(ctx, newStoredWorkspace(t, "b", "/repos/b")))

	all, err := repository.List(ctx)

	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestWorkspaceRepositoryListsOldestFirst(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	older := newStoredWorkspace(t, "older", "/repos/older")
	newer := newStoredWorkspace(t, "newer", "/repos/newer")
	newer.CreatedAt = older.CreatedAt.Add(time.Hour)
	require.NoError(t, repository.Save(ctx, newer))
	require.NoError(t, repository.Save(ctx, older))

	all, err := repository.List(ctx)

	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "older", all[0].Name)
	assert.Equal(t, "newer", all[1].Name)
}

func TestWorkspaceRepositoryReportsDuplicatePathAsAlreadyRegistered(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	require.NoError(t, repository.Save(ctx, newStoredWorkspace(t, "a", "/repos/same")))

	err := repository.Save(ctx, newStoredWorkspace(t, "b", "/repos/same"))

	require.ErrorIs(t, err, werrors.AlreadyRegistered(""))
	assert.EqualError(t, err, "ya existe un workspace para la ruta /repos/same")
}

func TestWorkspaceRepositoryDeleteRemovesAndReportsMissing(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	saved := newStoredWorkspace(t, "a", "/repos/a")
	require.NoError(t, repository.Save(ctx, saved))

	require.NoError(t, repository.Delete(ctx, saved.ID))
	again := repository.Delete(ctx, saved.ID)

	found, err := repository.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	assert.Nil(t, found)
	require.ErrorIs(t, again, werrors.NotFound(""))
	assert.EqualError(t, again, "workspace no encontrado: "+saved.ID.String())
}

func TestWorkspaceRepositoryUpdateChangesNameAndBranchButNotPath(t *testing.T) {
	ctx := context.Background()
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	saved := newStoredWorkspace(t, "a", "/repos/a")
	require.NoError(t, repository.Save(ctx, saved))

	edited := saved
	edited.Name = "renombrado"
	edited.BaseBranch = "develop"
	edited.Path = "/repos/otra"
	require.NoError(t, repository.Update(ctx, edited))

	found, err := repository.FindByID(ctx, saved.ID)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "renombrado", found.Name)
	assert.Equal(t, "develop", found.BaseBranch)
	assert.Equal(t, saved.Path, found.Path)
	assert.True(t, saved.CreatedAt.Equal(found.CreatedAt))
}

func TestWorkspaceRepositoryUpdateReportsMissingWorkspace(t *testing.T) {
	repository := persistence.NewWorkspaceRepository(dbtest.New(t))
	missing := newStoredWorkspace(t, "a", "/repos/a")

	err := repository.Update(context.Background(), missing)

	require.ErrorIs(t, err, werrors.NotFound(""))
	assert.EqualError(t, err, "workspace no encontrado: "+missing.ID.String())
}

func TestWorkspaceRepositoryDeleteReportsInUseWhenTasksReferenceIt(t *testing.T) {
	ctx := context.Background()
	handle := dbtest.New(t)
	repository := persistence.NewWorkspaceRepository(handle)
	saved := newStoredWorkspace(t, "a", "/repos/a")
	require.NoError(t, repository.Save(ctx, saved))
	insertTaskReferencing(t, handle, saved.ID)

	err := repository.Delete(ctx, saved.ID)

	require.ErrorIs(t, err, werrors.InUse())
	assert.EqualError(t, err, "el workspace tiene tareas activas")
}
