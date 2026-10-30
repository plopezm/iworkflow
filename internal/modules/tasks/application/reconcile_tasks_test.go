package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// reconcileRestartMessage is the restart text of an interrupted task, as Rust words it.
const reconcileRestartMessage = "el servidor se reinició mientras la tarea estaba activa"

// reconcileFixture runs ReconcileTasks over the given tasks; exists tells whether their worktrees are still on disk.
func reconcileFixture(t *testing.T, exists bool, tasks ...models.Task) (*application.ReconcileTasks, *fakeRepository, *fakeWorktree) {
	t.Helper()
	repo := newFakeRepository(tasks...)
	lifecycle, _ := newLifecycle(repo)
	worktrees := &fakeWorktree{exists: exists}
	return application.NewReconcileTasks(lifecycle, worktrees), repo, worktrees
}

// Rust: reconcile_tasks::missing_worktree_gets_discard_only_message
func TestReconcileTasksMissingWorktreeGetsDiscardOnlyMessage(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	useCase, repo, _ := reconcileFixture(t, false, task)

	count, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	require.NotNil(t, repo.errorTexts[0])
	assert.Contains(t, *repo.errorTexts[0], "ya no existe")
	assert.Contains(t, *repo.errorTexts[0], task.WorktreePath)
}

// Rust: reconcile_tasks::existing_worktree_keeps_restart_message
func TestReconcileTasksExistingWorktreeKeepsRestartMessage(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	useCase, repo, _ := reconcileFixture(t, true, task)

	_, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, repo.errorTexts[0])
	assert.Equal(t, reconcileRestartMessage, *repo.errorTexts[0])
}

// Rust: reconcile_tasks::marks_active_tasks_as_interrupted
func TestReconcileTasksMarksActiveTasksAsInterrupted(t *testing.T) {
	t.Parallel()
	useCase, repo, _ := reconcileFixture(t, true, newTask(models.StatusRunning), newTask(models.StatusIdle))

	count, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []models.TaskStatus{models.StatusInterrupted, models.StatusInterrupted}, repo.statuses)
}

// Rust: reconcile_tasks::leaves_rate_limited_tasks_untouched
func TestReconcileTasksLeavesRateLimitedTasksUntouched(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRateLimited)
	useCase, repo, _ := reconcileFixture(t, true, task)

	count, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, repo.statuses)
	assert.Equal(t, models.StatusRateLimited, repo.tasks[task.ID].Status)
}

// Rule from source: a task that was finishing when the server stopped is active too, so it is interrupted. Rust
// lists the statuses but only tests the exclusion of rate-limited tasks.
func TestReconcileTasksInterruptsFinishingTasks(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusFinishing)
	useCase, repo, _ := reconcileFixture(t, true, task)

	count, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Equal(t, []models.TaskStatus{models.StatusInterrupted}, repo.statuses)
}

// Rule from source: a task without a worktree path has nothing on disk to check, so the restart message stands and
// the filesystem is not asked.
func TestReconcileTasksWithoutWorktreePathSkipsTheFilesystemCheck(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusStarting)
	task.WorktreePath = ""
	useCase, repo, worktrees := reconcileFixture(t, false, task)

	_, err := useCase.Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 0, worktrees.existsCalls)
	require.NotNil(t, repo.errorTexts[0])
	assert.Equal(t, reconcileRestartMessage, *repo.errorTexts[0])
}
