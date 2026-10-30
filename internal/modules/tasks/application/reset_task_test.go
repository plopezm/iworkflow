package application_test

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// resetFixture is a task with its worktree, and the fakes behind the reset.
type resetFixture struct {
	lifecycle *application.TaskLifecycle
	repo      *fakeRepository
	publisher *fakePublisher
	worktrees *fakeWorktree
	runtime   *fakeRuntime
}

// newResetFixture builds the fixture around a task in the given status.
func newResetFixture(status models.TaskStatus) (models.Task, *resetFixture) {
	task := newTask(status)
	repo := newFakeRepository(task)
	lifecycle, publisher := newLifecycle(repo)
	return task, &resetFixture{
		lifecycle: lifecycle,
		repo:      repo,
		publisher: publisher,
		worktrees: &fakeWorktree{},
		runtime:   &fakeRuntime{},
	}
}

// useCase wires ResetTask over the fixture.
func (f *resetFixture) useCase() *application.ResetTask {
	return application.NewResetTask(f.lifecycle, f.worktrees, f.runtime)
}

// Rust: reset_task::stops_live_agent_removes_worktree_and_resets
func TestResetTaskStopsLiveAgentRemovesWorktreeAndResets(t *testing.T) {
	task, fixture := newResetFixture(models.StatusRunning)
	fixture.runtime.running = true

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskID{task.ID}, fixture.runtime.stopped)
	assert.Len(t, fixture.worktrees.removed, 1)
	assert.Equal(t, 1, fixture.repo.resets)
	assert.Empty(t, fixture.repo.statuses, "the reset does not write a status through UpdateStatus")
	assert.Len(t, fixture.publisher.published, 1)
}

// Rust: reset_task::resets_discarded_task_without_stopping_runtime
func TestResetTaskResetsDiscardedTaskWithoutStoppingRuntime(t *testing.T) {
	task, fixture := newResetFixture(models.StatusDiscarded)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, fixture.runtime.stopped)
	assert.Equal(t, 1, fixture.repo.resets)
}

// Rust: reset_task::rejects_backlog_and_merged_tasks
func TestResetTaskRejectsBacklogAndMergedTasks(t *testing.T) {
	for _, status := range []models.TaskStatus{models.StatusBacklog, models.StatusMerged, models.StatusFinishing} {
		task, fixture := newResetFixture(status)

		err := fixture.useCase().Execute(context.Background(), task.ID)

		assert.ErrorIs(t, err, werrors.InvalidState("restablecer", status.String()), "status %s", status)
		assert.Empty(t, fixture.worktrees.removed, "status %s", status)
		assert.Zero(t, fixture.repo.resets, "status %s", status)
	}
}

// Rust: reset_task::does_not_touch_database_when_worktree_removal_fails
func TestResetTaskDoesNotTouchDatabaseWhenWorktreeRemovalFails(t *testing.T) {
	task, fixture := newResetFixture(models.StatusAwaitingReview)
	fixture.worktrees.removeErr = werrors.Git("boom")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("boom"))
	assert.Zero(t, fixture.repo.resets)
	assert.Empty(t, fixture.publisher.published)
}

// Rust: reset_task::reports_conflict_when_repository_does_not_apply_reset
func TestResetTaskReportsConflictWhenRepositoryDoesNotApplyReset(t *testing.T) {
	task, fixture := newResetFixture(models.StatusIdle)
	notApplied := false
	fixture.repo.resetResult = &notApplied

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.InvalidState("restablecer", "idle"))
	assert.Empty(t, fixture.publisher.published)
}

// Rule from source: reset accepts every status except backlog, merged and finishing, including discarded and
// rate_limited, and removes the worktree and branch for each of them.
func TestResetTaskAcceptsEveryStatusExceptBacklogMergedAndFinishing(t *testing.T) {
	rejected := []models.TaskStatus{models.StatusBacklog, models.StatusMerged, models.StatusFinishing}
	for _, status := range models.AllTaskStatuses {
		if slices.Contains(rejected, status) {
			continue
		}
		task, fixture := newResetFixture(status)

		err := fixture.useCase().Execute(context.Background(), task.ID)

		require.NoError(t, err, "status %s", status)
		assert.Equal(t, 1, fixture.repo.resets, "status %s", status)
		assert.Len(t, fixture.worktrees.removed, 1, "status %s", status)
	}
}

// Rule from source: reset removes the worktree and the branch of the task from the repository of its workspace.
func TestResetTaskRemovesWorktreeAndBranchFromTheWorkspaceRepository(t *testing.T) {
	task, fixture := newResetFixture(models.StatusIdle)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []string{"/repos/demo|/worktrees/demo/abc-task|pi/arregla-el-login-abc"}, fixture.worktrees.removed)
}

// Rule from source: when the live agent cannot be stopped, reset stops before touching the worktree or the database.
func TestResetTaskStopFailureLeavesWorktreeAndHistoryUntouched(t *testing.T) {
	task, fixture := newResetFixture(models.StatusRunning)
	fixture.runtime.running = true
	fixture.runtime.stopErr = werrors.Git("no se pudo detener")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("no se pudo detener"))
	assert.Empty(t, fixture.worktrees.removed)
	assert.Zero(t, fixture.repo.resets)
	assert.Empty(t, fixture.publisher.published)
}

// Rule from source: when the task's workspace is gone, reset fails before removing anything or touching the database.
func TestResetTaskMissingWorkspaceFailsBeforeRemovingAnything(t *testing.T) {
	task, fixture := newResetFixture(models.StatusIdle)
	task.WorkspaceID = "missing"
	fixture.repo.tasks[task.ID] = task

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.WorkspaceNotFound("missing"))
	assert.Empty(t, fixture.worktrees.removed)
	assert.Zero(t, fixture.repo.resets)
}

// Rule from source: after the history is deleted, reset records only the backlog status event, so the new history
// starts with it.
func TestResetTaskRecordsOnlyTheBacklogEventAfterReset(t *testing.T) {
	task, fixture := newResetFixture(models.StatusIdle)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskEventKind{models.StatusKind(models.StatusBacklog, nil)}, fixture.repo.events)
	assert.Len(t, fixture.publisher.published, 1)
}
