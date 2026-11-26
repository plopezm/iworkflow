package tasks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// A workspace is blocked by every task that is neither merged nor discarded, backlog included: a backlog task
// may already hold a branch or a worktree. Merged and discarded tasks never block it.
func TestWorkspaceUnfinishedTasksBlocksOnEveryStatusButMergedAndDiscarded(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	unfinished := application.NewWorkspaceUnfinishedTasks(repo.Tasks)

	for _, status := range models.AllTaskStatuses {
		t.Run(string(status), func(t *testing.T) {
			workspaceID := insertWorkspace(t, repo.DB, "/repos/"+string(status))
			insertTask(t, repo, newTask(t, workspaceID, "task", status))

			blocked, err := unfinished.HasUnfinished(bg(), workspaceID)

			require.NoError(t, err)
			assert.Equal(t, !status.IsClosed(), blocked)
		})
	}
}

func TestWorkspaceUnfinishedTasksIgnoresTasksOfOtherWorkspaces(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	mine := insertWorkspace(t, repo.DB, "/repos/mine")
	other := insertWorkspace(t, repo.DB, "/repos/other")
	insertTask(t, repo, newTask(t, other, "foreign", models.StatusRunning))

	blocked, err := application.NewWorkspaceUnfinishedTasks(repo.Tasks).HasUnfinished(bg(), mine)

	require.NoError(t, err)
	assert.False(t, blocked)
}

func TestWorkspaceUnfinishedTasksIsFalseForAWorkspaceWithoutTasks(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	empty := insertWorkspace(t, repo.DB, "/repos/empty")

	blocked, err := application.NewWorkspaceUnfinishedTasks(repo.Tasks).HasUnfinished(bg(), empty)

	require.NoError(t, err)
	assert.False(t, blocked)
}
