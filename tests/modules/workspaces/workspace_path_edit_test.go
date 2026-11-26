package workspaces_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
	"iworkflow/internal/modules/workspaces/infrastructure/di"
)

// blockedTasks is a WorkspaceTasks port in which every workspace has an unfinished task.
type blockedTasks struct{ noTasks }

func (blockedTasks) HasBlocking(_ context.Context, _ models.WorkspaceID) (bool, error) {
	return true, nil
}

// newUnrelatedRepo creates a repository whose first commit is not the one of newGitRepo.
func newUnrelatedRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "commit", "-q", "--allow-empty", "-m", "unrelated project")
	return root
}

func TestPathEditFollowsAMovedFolderOfTheSameRepository(t *testing.T) {
	ctx := context.Background()
	repo := newGitRepo(t)
	module := newPathEditModule(t, noTasks{})
	registered, err := module.Register.Execute(ctx, models.NewWorkspace{Name: "Demo", Path: repo})
	require.NoError(t, err)
	moved := filepath.Join(t.TempDir(), "moved-repo")
	require.NoError(t, os.Rename(repo, moved))

	updated, err := module.Update.Execute(ctx, registered.ID, models.WorkspaceUpdate{Name: "Demo", Path: moved})

	require.NoError(t, err)
	assert.Equal(t, evalSymlinks(t, moved), updated.Path)
	fetched, err := module.Get.Execute(ctx, registered.ID)
	require.NoError(t, err)
	assert.Equal(t, evalSymlinks(t, moved), fetched.Path, "the new path is stored")
}

func TestPathEditRejectsAnotherProjectAndKeepsTheStoredPath(t *testing.T) {
	ctx := context.Background()
	repo := newGitRepo(t)
	other := newUnrelatedRepo(t)
	module := newPathEditModule(t, noTasks{})
	registered, err := module.Register.Execute(ctx, models.NewWorkspace{Name: "Demo", Path: repo})
	require.NoError(t, err)

	_, err = module.Update.Execute(ctx, registered.ID, models.WorkspaceUpdate{Name: "Renamed", Path: evalSymlinks(t, other)})

	require.ErrorIs(t, err, werrors.PathNotSameProject(""))
	fetched, err := module.Get.Execute(ctx, registered.ID)
	require.NoError(t, err)
	assert.Equal(t, evalSymlinks(t, repo), fetched.Path)
	assert.Equal(t, "Demo", fetched.Name, "the rejected change does not rename the workspace either")
}

func TestPathEditIsLockedWhileTheWorkspaceHasUnfinishedTasks(t *testing.T) {
	ctx := context.Background()
	repo := newGitRepo(t)
	module := newPathEditModule(t, blockedTasks{})
	registered, err := module.Register.Execute(ctx, models.NewWorkspace{Name: "Demo", Path: repo})
	require.NoError(t, err)
	stored := evalSymlinks(t, repo)
	moved := filepath.Join(t.TempDir(), "moved-repo")
	require.NoError(t, os.Rename(repo, moved))

	locked, err := module.Update.PathEditBlocker(ctx, registered.ID)
	require.NoError(t, err)
	_, err = module.Update.Execute(ctx, registered.ID, models.WorkspaceUpdate{Name: "Demo", Path: moved})

	assert.True(t, locked)
	require.ErrorIs(t, err, werrors.PathBlockedByTasks())
	fetched, err := module.Get.Execute(ctx, registered.ID)
	require.NoError(t, err)
	assert.Equal(t, stored, fetched.Path)
}

// newPathEditModule composes the workspaces module over a migrated database, with the given tasks port.
func newPathEditModule(t *testing.T, tasks outbound.WorkspaceTasks) *di.Module {
	t.Helper()
	return di.NewModule(di.Deps{
		DB:          dbtest.New(t),
		HomeDir:     t.TempDir(),
		Tasks:       tasks,
		AgentsFiles: noAgentsFiles{},
	})
}
