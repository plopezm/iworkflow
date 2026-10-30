package workspaces_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/infrastructure/di"
)

// noTasks is a WorkspaceTasks port with no tasks to delete.
type noTasks struct{}

func (noTasks) DeleteAll(_ context.Context, _ models.WorkspaceID) error { return nil }

// noAgentsFiles is an AgentsFiles port with no stored files.
type noAgentsFiles struct{}

func (noAgentsFiles) ListOptions(_ context.Context) ([]models.AgentsFileOption, error) {
	return nil, nil
}

func (noAgentsFiles) Install(_ context.Context, _ string, _ string, _ bool) error {
	return werrors.AgentsFileNotFound("unused")
}

func TestModuleRegistersListsUpdatesAndDeletesRealRepository(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	repo := newGitRepo(t)
	module := di.NewModule(di.Deps{
		DB:          dbtest.New(t),
		HomeDir:     home,
		Tasks:       noTasks{},
		AgentsFiles: noAgentsFiles{},
	})

	registered, err := module.Register.Execute(ctx, models.NewWorkspace{Name: "Demo", Path: repo})
	require.NoError(t, err)
	assert.Equal(t, evalSymlinks(t, repo), registered.Path)
	assert.Equal(t, "main", registered.BaseBranch)

	listed, err := module.List.Execute(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)

	fetched, err := module.Get.Execute(ctx, registered.ID)
	require.NoError(t, err)
	assert.Equal(t, registered.ID, fetched.ID)

	updated, err := module.Update.Execute(ctx, registered.ID, models.WorkspaceUpdate{Name: "Renamed"})
	require.NoError(t, err)
	assert.Equal(t, "Renamed", updated.Name)

	require.NoError(t, module.Delete.Execute(ctx, registered.ID))
	_, err = module.Get.Execute(ctx, registered.ID)
	require.ErrorIs(t, err, werrors.NotFound(""))
}

func TestModuleReportsInitializationRequiredForMissingFolderWithoutInitialize(t *testing.T) {
	module := di.NewModule(di.Deps{
		DB:          dbtest.New(t),
		HomeDir:     t.TempDir(),
		Tasks:       noTasks{},
		AgentsFiles: noAgentsFiles{},
	})
	missing := filepath.Join(t.TempDir(), "not-yet")

	_, err := module.Register.Execute(context.Background(), models.NewWorkspace{Name: "Demo", Path: missing})

	path, reason, ok := werrors.AsInitializationRequired(err)
	require.True(t, ok, "unexpected error: %v", err)
	assert.Equal(t, missing, path)
	assert.Equal(t, werrors.ReasonMissingFolder, reason)
}

func TestModuleCreatesDirectoryOnlyInsideHome(t *testing.T) {
	home := t.TempDir()
	module := di.NewModule(di.Deps{
		DB:          dbtest.New(t),
		HomeDir:     home,
		Tasks:       noTasks{},
		AgentsFiles: noAgentsFiles{},
	})

	created, err := module.CreateDirectory.Execute(context.Background(), home, "proyecto")
	require.NoError(t, err)
	assert.DirExists(t, created)

	_, err = module.CreateDirectory.Execute(context.Background(), filepath.Dir(home), "escape")
	require.ErrorIs(t, err, werrors.PathOutsideAllowedRoot(""))
}
