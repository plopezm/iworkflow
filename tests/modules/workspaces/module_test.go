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

// noTasks is a WorkspaceTasks port with no tasks: nothing to delete and nothing that blocks a path change.
type noTasks struct{}

func (noTasks) DeleteAll(_ context.Context, _ models.WorkspaceID) error { return nil }

func (noTasks) HasBlocking(_ context.Context, _ models.WorkspaceID) (bool, error) { return false, nil }

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

func TestModulePersistsTheWorkspaceColorAndKeepsItWhenTheUpdateOmitsIt(t *testing.T) {
	ctx := context.Background()
	module := di.NewModule(di.Deps{DB: dbtest.New(t), HomeDir: t.TempDir(), Tasks: noTasks{}, AgentsFiles: noAgentsFiles{}})

	registered, err := module.Register.Execute(ctx, models.NewWorkspace{Name: "Demo", Path: newGitRepo(t), Color: "blue"})
	require.NoError(t, err)
	fetched, err := module.Get.Execute(ctx, registered.ID)
	require.NoError(t, err)
	require.NotNil(t, fetched.Color)
	assert.Equal(t, "blue", *fetched.Color)

	// Absent keeps the saved color.
	kept, err := module.Update.Execute(ctx, registered.ID, models.WorkspaceUpdate{Name: "Demo"})
	require.NoError(t, err)
	require.NotNil(t, kept.Color)
	assert.Equal(t, "blue", *kept.Color)

	// Present changes it; present and empty removes it.
	green := "green"
	_, err = module.Update.Execute(ctx, registered.ID, models.WorkspaceUpdate{Name: "Demo", Color: &green})
	require.NoError(t, err)
	fetched, err = module.Get.Execute(ctx, registered.ID)
	require.NoError(t, err)
	assert.Equal(t, &green, fetched.Color)

	none := ""
	_, err = module.Update.Execute(ctx, registered.ID, models.WorkspaceUpdate{Name: "Demo", Color: &none})
	require.NoError(t, err)
	fetched, err = module.Get.Execute(ctx, registered.ID)
	require.NoError(t, err)
	assert.Nil(t, fetched.Color)
}

func TestModuleRejectsAnUnknownColorWithoutSavingAnything(t *testing.T) {
	ctx := context.Background()
	module := di.NewModule(di.Deps{DB: dbtest.New(t), HomeDir: t.TempDir(), Tasks: noTasks{}, AgentsFiles: noAgentsFiles{}})

	_, err := module.Register.Execute(ctx, models.NewWorkspace{Name: "Demo", Path: newGitRepo(t), Color: "chartreuse"})

	require.ErrorIs(t, err, werrors.InvalidColor(""))
	listed, listErr := module.List.Execute(ctx)
	require.NoError(t, listErr)
	assert.Empty(t, listed)
}
