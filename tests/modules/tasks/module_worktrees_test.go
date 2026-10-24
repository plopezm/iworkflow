package tasks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// End to end through the module: a task is planned, its worktree is created, the agent's work is committed,
// the diff is read, and the merge integrates it into main and removes the worktree.
// It runs without t.Parallel because newGitRepository isolates the git configuration.
func TestModuleMergesAReviewedTaskIntoItsBase(t *testing.T) {
	repo := newRepo(t)
	repoPath := newGitRepository(t)
	workspaceID := insertWorkspace(t, repo.DB, repoPath)
	module := newModule(t, repo)
	ctx := context.Background()

	created, err := module.Create.Execute(ctx, models.NewTask{WorkspaceID: workspaceID, Title: "nueva funcion", Prompt: "añade el fichero"})
	require.NoError(t, err)
	task := created.Task
	// The start use case (slice F) creates the worktree in production; here the port does it directly.
	require.NoError(t, module.Worktrees.Create(ctx, repoPath, "main", task.Branch, task.WorktreePath))
	require.NoError(t, os.WriteFile(filepath.Join(task.WorktreePath, "nuevo.txt"), []byte("hola\n"), 0o644))
	_, err = module.Worktrees.CommitAll(ctx, task.WorktreePath, "pi: nuevo")
	require.NoError(t, err)
	require.NoError(t, module.Lifecycle.Transition(ctx, task.ID, models.StatusAwaitingReview, nil))

	diff, err := module.GetDiff.Execute(ctx, task.ID)
	require.NoError(t, err)
	assert.Contains(t, diff.Stat, "nuevo.txt")

	outcome, err := module.Merge.Execute(ctx, task.ID)

	require.NoError(t, err)
	assert.False(t, outcome.Conflict)
	assert.FileExists(t, filepath.Join(repoPath, "nuevo.txt"))
	assert.Equal(t, "Merge tarea pi: nueva funcion", gitCmd(t, repoPath, "log", "-1", "--format=%s"))
	merged, err := module.Get.Execute(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusMerged, merged.Status)
	require.NotNil(t, merged.DiffRange, "the commit range is recorded when the task is integrated")
	assert.NoDirExists(t, task.WorktreePath)
}
