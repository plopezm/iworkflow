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

// discardFixture is a task with its worktree, and the fakes behind the discard.
type discardFixture struct {
	lifecycle *application.TaskLifecycle
	repo      *fakeRepository
	publisher *fakePublisher
	worktrees *fakeWorktree
	runtime   *fakeRuntime
}

// newDiscardFixture builds the fixture around a task in the given status and with the given pending dialogs.
func newDiscardFixture(status models.TaskStatus, dialogIDs ...string) (models.Task, *discardFixture) {
	task := newTask(status, dialogIDs...)
	repo := newFakeRepository(task)
	lifecycle, publisher := newLifecycle(repo)
	return task, &discardFixture{
		lifecycle: lifecycle,
		repo:      repo,
		publisher: publisher,
		worktrees: &fakeWorktree{},
		runtime:   &fakeRuntime{},
	}
}

// useCase wires DiscardTask over the fixture.
func (f *discardFixture) useCase() *application.DiscardTask {
	return application.NewDiscardTask(f.lifecycle, f.worktrees, f.runtime)
}

// Rust: discard_task::stops_live_agent_and_removes_worktree
func TestDiscardTaskStopsLiveAgentAndRemovesWorktree(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusRunning)
	fixture.runtime.running = true

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskID{task.ID}, fixture.runtime.stopped)
	assert.Len(t, fixture.worktrees.removed, 1)
	assert.Equal(t, []models.TaskStatus{models.StatusDiscarded}, fixture.repo.statuses)
}

// Rust: discard_task::rejects_already_merged_task
func TestDiscardTaskRejectsAlreadyMergedTask(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusMerged)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.InvalidState("descartar", "merged"))
}

// Rule from source: discarding records the Discarded status with no error text and clears the pending dialogs of the
// task, because a discarded task has no live agent to answer them.
func TestDiscardTaskRecordsDiscardedAndClearsPendingDialogs(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusRunning, "dlg-1")
	fixture.runtime.running = true

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []*string{nil}, fixture.repo.errorTexts)
	assert.Equal(t, 1, fixture.repo.dialogClears)
}

// Rule from source: discard removes the worktree and the branch of the task from the repository of its workspace,
// without integrating anything.
func TestDiscardTaskRemovesWorktreeAndBranchFromTheWorkspaceRepository(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusAwaitingReview)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []string{"/repos/demo|/worktrees/demo/abc-task|pi/arregla-el-login-abc"}, fixture.worktrees.removed)
	assert.Empty(t, fixture.worktrees.merged)
}

// Rule from source: discard accepts every status except discarded, merged, finishing and backlog, and refuses the
// rest before the agent is touched or git is called.
func TestDiscardTaskAcceptsOnlyTheDiscardableStatuses(t *testing.T) {
	rejected := []models.TaskStatus{models.StatusDiscarded, models.StatusMerged, models.StatusFinishing, models.StatusBacklog}
	for _, status := range models.AllTaskStatuses {
		task, fixture := newDiscardFixture(status)
		fixture.runtime.running = true

		err := fixture.useCase().Execute(context.Background(), task.ID)

		if !slices.Contains(rejected, status) {
			assert.NoError(t, err, "status %s", status)
			continue
		}
		assert.ErrorIs(t, err, werrors.InvalidState("descartar", status.String()), "status %s", status)
		assert.Empty(t, fixture.runtime.stopped, "status %s", status)
		assert.Empty(t, fixture.worktrees.removed, "status %s", status)
		assert.Empty(t, fixture.repo.statuses, "status %s", status)
	}
}

// Rule from source: when the worktree cannot be removed, the status is left as it was; the task is not marked
// discarded while its worktree still exists.
func TestDiscardTaskRemovalFailureKeepsStatus(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusAwaitingReview)
	fixture.worktrees.removeErr = werrors.Git("bloqueado")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("bloqueado"))
	assert.Empty(t, fixture.repo.statuses)
	assert.Empty(t, fixture.publisher.published)
}

// Rule from source: when the live agent cannot be stopped, discard stops before removing the worktree.
func TestDiscardTaskStopFailureLeavesWorktreeInPlace(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusRunning)
	fixture.runtime.running = true
	fixture.runtime.stopErr = werrors.Git("no se pudo detener")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("no se pudo detener"))
	assert.Empty(t, fixture.worktrees.removed)
	assert.Empty(t, fixture.repo.statuses)
}

// failingWorkspaces is a workspace lookup whose every query fails with err.
type failingWorkspaces struct {
	err error
}

func (f failingWorkspaces) Find(_ context.Context, _ string) (*models.WorkspaceRef, error) {
	return nil, f.err
}

func (f failingWorkspaces) List(_ context.Context) ([]models.WorkspaceRef, error) {
	return nil, f.err
}

// Intentional improvement over Rust: the workspace is resolved before the agent is stopped, so a failed lookup
// leaves the agent running and the task untouched.
func TestDiscardTaskWorkspaceLookupFailureDoesNotStopAgent(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusRunning)
	fixture.runtime.running = true
	lifecycle := application.NewTaskLifecycle(fixture.repo, fixture.publisher, failingWorkspaces{err: werrors.Repository("workspaces no disponibles")}, clock)
	useCase := application.NewDiscardTask(lifecycle, fixture.worktrees, fixture.runtime)

	err := useCase.Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Repository("workspaces no disponibles"))
	assert.Empty(t, fixture.runtime.stopped)
	assert.Empty(t, fixture.worktrees.removed)
	assert.Empty(t, fixture.repo.statuses)
}

// Rule from source: a successful discard publishes exactly one Discarded status event, persisted in the history.
func TestDiscardTaskPublishesOneDiscardedEvent(t *testing.T) {
	task, fixture := newDiscardFixture(models.StatusIdle)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskEventKind{models.StatusKind(models.StatusDiscarded, nil)}, fixture.repo.events)
	assert.Len(t, fixture.publisher.published, 1)
}
