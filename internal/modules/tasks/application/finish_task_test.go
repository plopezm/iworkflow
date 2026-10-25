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

// finishFixture is a task with its worktree, and the fakes behind the finish.
type finishFixture struct {
	lifecycle   *application.TaskLifecycle
	repo        *fakeRepository
	publisher   *fakePublisher
	worktrees   *fakeWorktree
	runtime     *fakeRuntime
	attachments *fakeAttachments
	storage     *fakeStorage
}

// newFinishFixture builds the fixture around a task in the given status.
func newFinishFixture(status models.TaskStatus) (models.Task, *finishFixture) {
	task := newTask(status)
	repo := newFakeRepository(task)
	lifecycle, publisher := newLifecycle(repo)
	return task, &finishFixture{
		lifecycle:   lifecycle,
		repo:        repo,
		publisher:   publisher,
		worktrees:   &fakeWorktree{},
		runtime:     &fakeRuntime{},
		attachments: &fakeAttachments{},
		storage:     &fakeStorage{},
	}
}

// useCase wires FinishTask over the fixture; the cleanup uses the attachment and storage fakes.
func (f *finishFixture) useCase() *application.FinishTask {
	return application.NewFinishTask(f.lifecycle, f.worktrees, f.runtime, application.NewAttachmentCleanup(f.attachments, f.storage))
}

// Rust: finish_task::stops_agent_commits_and_awaits_review
func TestFinishTaskStopsAgentCommitsAndAwaitsReview(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.runtime.running = true

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []string{"/worktrees/demo/abc-task|pi: Arregla el login"}, fixture.worktrees.committed)
	assert.Equal(t, []models.TaskID{task.ID}, fixture.runtime.stopped)
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusAwaitingReview}, fixture.repo.statuses)
}

// Rust: finish_task::commit_failure_marks_task_failed
func TestFinishTaskCommitFailureMarksTaskFailed(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusInterrupted)
	fixture.worktrees.commitErr = werrors.Git("index.lock")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("index.lock"))
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusFailed}, fixture.repo.statuses)
	assert.Equal(t, 1, fixture.runtime.runningCalls, "the agent is checked once, before the commit")
}

// Rust: finish_task::rejects_closed_task
func TestFinishTaskRejectsClosedTask(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusMerged)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.InvalidState("finalizar", "merged"))
}

// Rust: finish_task::cleanup_failures_do_not_fail_the_finish
func TestFinishTaskCleanupFailuresDoNotFailTheFinish(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.attachments.failDeleteByTask = werrors.Repository("db")
	fixture.storage.failDeleteDir = true

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusAwaitingReview}, fixture.repo.statuses)
	assert.Equal(t, 1, fixture.attachments.deletedByTask, "the metadata cleanup still ran")
	assert.Len(t, fixture.storage.deletedDirs, 1, "the folder cleanup still ran after the metadata failure")
}

// Rule from source: finish keeps the worktree and its branch for the manual review; only merge and discard remove them.
func TestFinishTaskKeepsTheWorktreeAndBranch(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Len(t, fixture.worktrees.committed, 1)
	assert.Empty(t, fixture.worktrees.removed)
}

// Rule from source: only running, idle, interrupted, failed and conflict tasks can finish; any other status is refused
// before the agent is touched or git is called.
func TestFinishTaskAcceptsOnlyTheFinishableStatuses(t *testing.T) {
	finishable := []models.TaskStatus{
		models.StatusRunning,
		models.StatusIdle,
		models.StatusInterrupted,
		models.StatusFailed,
		models.StatusConflict,
	}
	for _, status := range models.AllTaskStatuses {
		task, fixture := newFinishFixture(status)
		fixture.runtime.running = true

		err := fixture.useCase().Execute(context.Background(), task.ID)

		if slices.Contains(finishable, status) {
			assert.NoError(t, err, "status %s", status)
			continue
		}
		assert.ErrorIs(t, err, werrors.InvalidState("finalizar", status.String()), "status %s", status)
		assert.Empty(t, fixture.runtime.stopped, "status %s", status)
		assert.Empty(t, fixture.worktrees.committed, "status %s", status)
		assert.Empty(t, fixture.repo.statuses, "status %s", status)
	}
}

// Rule from source: when the live agent cannot be stopped, the finish stops before the commit and marks the task
// failed with the stop error.
func TestFinishTaskStopFailureMarksFailedWithoutCommitting(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusRunning)
	fixture.runtime.running = true
	fixture.runtime.stopErr = werrors.Git("no se pudo detener")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("no se pudo detener"))
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusFailed}, fixture.repo.statuses)
	assert.Empty(t, fixture.worktrees.committed)
}

// Rule from source: the failed status keeps the consolidation error text, so the review shows why the finish failed.
func TestFinishTaskCommitFailureStoresTheErrorTextOnFailed(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.worktrees.commitErr = werrors.Git("index.lock")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.Error(t, err)
	require.NotEmpty(t, fixture.repo.errorTexts)
	last := fixture.repo.errorTexts[len(fixture.repo.errorTexts)-1]
	require.NotNil(t, last)
	assert.Equal(t, err.Error(), *last)
}

// Rule from source: the attachments of a task are cleaned up only once it reached awaiting review; a failed
// consolidation keeps them, because the task is not finished.
func TestFinishTaskKeepsAttachmentsWhenConsolidationFails(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.worktrees.commitErr = werrors.Git("index.lock")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.Error(t, err)
	assert.Zero(t, fixture.attachments.deletedByTask)
	assert.Empty(t, fixture.storage.deletedDirs)
}

// Rule from source: a successful finish publishes the Finishing and AwaitingReview status events, persisted in order.
func TestFinishTaskPublishesTheStatusEventsInOrder(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)

	err := fixture.useCase().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskEventKind{
		models.StatusKind(models.StatusFinishing, nil),
		models.StatusKind(models.StatusAwaitingReview, nil),
	}, fixture.repo.events)
	assert.Len(t, fixture.publisher.published, 2)
}
