package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: delete_workspace_tasks::removes_worktree_of_closed_tasks_with_branch. Closed tasks with a branch
// have their worktree and branch removed, in the workspace repository; open ones never are.
func TestDeleteWorkspaceTasksRemovesTheWorktreeOfClosedTasksWithABranch(t *testing.T) {
	merged := newTask(models.StatusMerged)
	discarded := newTask(models.StatusDiscarded)
	backlog := newTask(models.StatusBacklog)
	repo := newFakeRepository(merged, discarded, backlog)
	remover := &fakeRemover{}
	storage := &fakeStorage{}
	lifecycle, publisher := newLifecycle(repo)

	require.NoError(t, application.NewDeleteWorkspaceTasks(lifecycle, remover, storage).DeleteAll(context.Background(), "ws-1"))

	require.Len(t, remover.removed, 2)
	assert.Len(t, storage.deletedDirs, 3, "every deleted task folder is removed")
	assert.Len(t, publisher.published, 3, "every deleted task is announced")
	assert.Contains(t, remover.removed, "/repos/demo|"+merged.WorktreePath+"|"+merged.Branch)
	assert.Contains(t, remover.removed, "/repos/demo|"+discarded.WorktreePath+"|"+discarded.Branch)
	assert.Empty(t, repo.tasks)
}

// Rust: delete_workspace_tasks::cleanup_failure_does_not_block_deletion
func TestDeleteWorkspaceTasksWorktreeCleanupFailureDoesNotBlockDeletion(t *testing.T) {
	merged := newTask(models.StatusMerged)
	repo := newFakeRepository(merged)
	remover := &fakeRemover{err: werrors.Git("bloqueado")}
	lifecycle, publisher := newLifecycle(repo)

	err := application.NewDeleteWorkspaceTasks(lifecycle, remover, &fakeStorage{}).DeleteAll(context.Background(), "ws-1")

	require.NoError(t, err)
	assert.Empty(t, repo.tasks)
	assert.Len(t, publisher.published, 1)
}

// Rust: delete_workspace_tasks::rejects_when_any_task_is_active_without_cleaning
// A refused deletion (an active task) cleans nothing: no worktree is removed for a workspace that stays.
func TestDeleteWorkspaceTasksRefusalRemovesNoWorktree(t *testing.T) {
	merged := newTask(models.StatusMerged)
	running := newTask(models.StatusRunning)
	repo := newFakeRepository(merged, running)
	remover := &fakeRemover{}
	storage := &fakeStorage{}
	lifecycle, publisher := newLifecycle(repo)

	err := application.NewDeleteWorkspaceTasks(lifecycle, remover, storage).DeleteAll(context.Background(), "ws-1")

	assert.ErrorIs(t, err, werrors.WorkspaceHasActiveTasks())
	assert.Empty(t, remover.removed)
	assert.Empty(t, storage.deletedDirs)
	assert.Empty(t, publisher.published)
	assert.Len(t, repo.tasks, 2)
}
