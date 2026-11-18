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

// Rollback (T5, S3): the finish stopped a live agent before the commit failed. The agent is gone, so the task is
// interrupted, with the failure as its error text. The agent is checked once, before the commit.
func TestFinishTaskCommitFailureWithLiveAgentMarksInterrupted(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusRunning)
	fixture.runtime.running = true
	fixture.worktrees.commitErr = werrors.Git("index.lock")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("index.lock"))
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusInterrupted}, fixture.repo.statuses)
	require.Len(t, fixture.repo.errorTexts, 2)
	require.NotNil(t, fixture.repo.errorTexts[1])
	assert.Equal(t, err.Error(), *fixture.repo.errorTexts[1])
	assert.Equal(t, []models.TaskID{task.ID}, fixture.runtime.stopped)
	assert.Equal(t, 1, fixture.runtime.runningCalls, "the agent is checked once, before the commit")
}

// Rollback (T5, S3): a finish that fails before it touches a live agent puts the task back to the status it had when
// the finish started, with the failure as its error text.
func TestFinishTaskCommitFailureRestoresThePreviousStatus(t *testing.T) {
	for _, previous := range []models.TaskStatus{models.StatusIdle, models.StatusInterrupted, models.StatusFailed, models.StatusConflict} {
		task, fixture := newFinishFixture(previous)
		fixture.worktrees.commitErr = werrors.Git("index.lock")

		err := fixture.useCase().Execute(context.Background(), task.ID)

		assert.ErrorIs(t, err, werrors.Git("index.lock"), "status %s", previous)
		assert.Equal(t, []models.TaskStatus{models.StatusFinishing, previous}, fixture.repo.statuses, "status %s", previous)
		require.Len(t, fixture.repo.errorTexts, 2, "status %s", previous)
		require.NotNil(t, fixture.repo.errorTexts[1], "status %s", previous)
		assert.Equal(t, err.Error(), *fixture.repo.errorTexts[1], "status %s", previous)
	}
}

// Rollback (T5, S3): a finish that died halfway (previous status finishing) ends failed, not finishing.
func TestFinishTaskFinishingPreviousStatusEndsFailed(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusFinishing)
	fixture.worktrees.commitErr = werrors.Git("index.lock")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("index.lock"))
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusFailed}, fixture.repo.statuses)
	require.Len(t, fixture.repo.errorTexts, 2)
	require.NotNil(t, fixture.repo.errorTexts[1])
	assert.Equal(t, err.Error(), *fixture.repo.errorTexts[1])
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
		models.StatusFinishing, // a finish that died leaves it here; retrying is the way out (the in-flight claim guards live ones)
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

// Rollback (T5, S3): when the live agent cannot be stopped, the finish stops before the commit and restores the previous
// status with the stop error. The agent was not stopped, so the status stays as it was (not interrupted).
func TestFinishTaskStopFailureRestoresThePreviousStatusWithTheAgentAlive(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusRunning)
	fixture.runtime.running = true
	fixture.runtime.stopErr = werrors.Git("no se pudo detener")

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("no se pudo detener"))
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusRunning}, fixture.repo.statuses)
	require.Len(t, fixture.repo.errorTexts, 2)
	require.NotNil(t, fixture.repo.errorTexts[1])
	assert.Equal(t, err.Error(), *fixture.repo.errorTexts[1])
	assert.Empty(t, fixture.worktrees.committed)
}

// Rollback (T5, S3): the restored status keeps the consolidation error text, so the review shows why the finish failed.
func TestFinishTaskCommitFailureStoresTheErrorTextOnTheRestoredStatus(t *testing.T) {
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

// Rollback (T5, S3): when the restore itself fails, both errors are returned, so the consolidation error that explains
// the failure is not masked by the restore error. errors.Join keeps errors.Is working on each of them.
func TestFinishTaskReturnsBothErrorsWhenTheRestoreAlsoFails(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.worktrees.commitErr = werrors.Git("index.lock")
	fixture.repo.updateStatusErrs = map[models.TaskStatus]error{models.StatusIdle: werrors.Repository("db bloqueada")}

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git("index.lock"))
	assert.ErrorIs(t, err, werrors.Repository("db bloqueada"))
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

// cancellingWorktree cancels the request while the commit runs, as a closed tab or an aborted htmx request does.
type cancellingWorktree struct {
	*fakeWorktree
	cancel context.CancelFunc
}

func (w cancellingWorktree) CommitAll(ctx context.Context, path string, message string) (bool, error) {
	w.cancel()
	return w.fakeWorktree.CommitAll(ctx, path, message)
}

// Bug: a request cancelled mid finish must not leave the task in finishing. The finish runs detached from the request,
// so it completes and the task reaches awaiting review.
func TestFinishTaskRequestCancelledMidFinishStillCompletes(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	lifecycle, _ := newLifecycle(cancelAwareRepository{fakeRepository: fixture.repo})
	fixture.lifecycle = lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fixture.worktrees = &fakeWorktree{}
	useCase := application.NewFinishTask(lifecycle, cancellingWorktree{fakeWorktree: fixture.worktrees, cancel: cancel}, fixture.runtime, application.NewAttachmentCleanup(fixture.attachments, fixture.storage))

	require.NoError(t, useCase.Execute(ctx, task.ID))

	assert.Equal(t, models.StatusAwaitingReview, fixture.repo.tasks[task.ID].Status)
}

// Bug: a failure after the request was cancelled still restores the previous status instead of leaving it in finishing.
func TestFinishTaskCancelledRequestWithCommitFailureStillRestoresThePreviousStatus(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	lifecycle, _ := newLifecycle(cancelAwareRepository{fakeRepository: fixture.repo})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fixture.worktrees = &fakeWorktree{commitErr: werrors.Git("index.lock")}
	useCase := application.NewFinishTask(lifecycle, cancellingWorktree{fakeWorktree: fixture.worktrees, cancel: cancel}, fixture.runtime, application.NewAttachmentCleanup(fixture.attachments, fixture.storage))

	require.ErrorIs(t, useCase.Execute(ctx, task.ID), werrors.Git("index.lock"))

	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusIdle}, fixture.repo.statuses)
	require.Len(t, fixture.repo.errorTexts, 2)
	require.NotNil(t, fixture.repo.errorTexts[1])
}

// Rollback (T5, S3): an AwaitingReview transition that fails after a good commit restores the previous status.
func TestFinishTaskAwaitingReviewFailureRestoresThePreviousStatus(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.repo.updateStatusErrs = map[models.TaskStatus]error{models.StatusAwaitingReview: werrors.Repository("db bloqueada")}

	err := fixture.useCase().Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Repository("db bloqueada"))
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusIdle}, fixture.repo.statuses)
	require.Len(t, fixture.repo.errorTexts, 2)
	require.NotNil(t, fixture.repo.errorTexts[1])
	assert.Equal(t, err.Error(), *fixture.repo.errorTexts[1])
}

// fakeProcessCloser records what finish asked it, and what had already happened by then.
type fakeProcessCloser struct {
	fixture          *finishFixture
	listed           []models.WorktreeProcess
	listErr          error
	listCalls        []string
	closeCalls       [][]models.WorktreeProcess
	closedPaths      []string
	stoppedAtClose   int
	committedAtClose int
	closeErr         error
}

func (f *fakeProcessCloser) ListUsers(_ context.Context, id models.TaskID, worktreePath string) ([]models.WorktreeProcess, error) {
	f.listCalls = append(f.listCalls, id.String()+"|"+worktreePath)
	return f.listed, f.listErr
}

func (f *fakeProcessCloser) CloseUsers(_ context.Context, worktreePath string, selected []models.WorktreeProcess) (int, error) {
	f.closedPaths = append(f.closedPaths, worktreePath)
	f.closeCalls = append(f.closeCalls, selected)
	f.stoppedAtClose = len(f.fixture.runtime.stopped)
	f.committedAtClose = len(f.fixture.worktrees.committed)
	return len(selected), f.closeErr
}

var devServer = models.WorktreeProcess{PID: 4321, Name: "node", Path: "/worktrees/demo/abc-task/web", Reason: models.ProcessReasonWorkingDirectory, Started: "t1"}

func newClosingUseCase(fixture *finishFixture, closer *fakeProcessCloser) *application.FinishTask {
	useCase := fixture.useCase()
	useCase.SetProcessCloser(closer)
	return useCase
}

func TestFinishTaskPreviewListsTheProcessesAndChangesNothing(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.runtime.running = true
	closer := &fakeProcessCloser{fixture: fixture, listed: []models.WorktreeProcess{devServer}}

	users, err := newClosingUseCase(fixture, closer).Preview(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.WorktreeProcess{devServer}, users)
	assert.Equal(t, []string{task.ID.String() + "|/worktrees/demo/abc-task"}, closer.listCalls)
	assert.Empty(t, fixture.repo.statuses, "previewing never changes the status")
	assert.Empty(t, fixture.runtime.stopped, "previewing never stops the agent")
	assert.Empty(t, fixture.worktrees.committed, "previewing never commits")
	assert.Empty(t, closer.closeCalls)
}

func TestFinishTaskPreviewWithoutACloserListsNothing(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)

	users, err := fixture.useCase().Preview(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, users)
}

func TestFinishTaskPreviewRejectsAStatusThatCannotFinish(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusMerged)
	closer := &fakeProcessCloser{fixture: fixture, listed: []models.WorktreeProcess{devServer}}

	_, err := newClosingUseCase(fixture, closer).Preview(context.Background(), task.ID)

	assert.Error(t, err)
	assert.Empty(t, closer.listCalls)
}

func TestFinishTaskPreviewTreatsAListingFailureAsNoProcesses(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	closer := &fakeProcessCloser{fixture: fixture, listErr: assert.AnError}

	users, err := newClosingUseCase(fixture, closer).Preview(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, users, "a failed scan must not block the finish")
}

func TestFinishTaskClosesTheSelectedProcessesBetweenStopAndCommit(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	fixture.runtime.running = true
	closer := &fakeProcessCloser{fixture: fixture}

	err := newClosingUseCase(fixture, closer).ExecuteClosing(context.Background(), task.ID, []models.WorktreeProcess{devServer})

	require.NoError(t, err)
	assert.Equal(t, [][]models.WorktreeProcess{{devServer}}, closer.closeCalls)
	assert.Equal(t, []string{"/worktrees/demo/abc-task"}, closer.closedPaths)
	assert.Equal(t, 1, closer.stoppedAtClose, "the agent is stopped first")
	assert.Equal(t, 0, closer.committedAtClose, "the processes are closed before the commit")
	assert.Len(t, fixture.worktrees.committed, 1)
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusAwaitingReview}, fixture.repo.statuses)
}

func TestFinishTaskExecuteClosesNothing(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	closer := &fakeProcessCloser{fixture: fixture, listed: []models.WorktreeProcess{devServer}}

	err := newClosingUseCase(fixture, closer).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, closer.closeCalls, "without a selection no process is touched")
	assert.Len(t, fixture.worktrees.committed, 1)
}

func TestFinishTaskContinuesWhenClosingTheProcessesFails(t *testing.T) {
	task, fixture := newFinishFixture(models.StatusIdle)
	closer := &fakeProcessCloser{fixture: fixture, closeErr: assert.AnError}

	err := newClosingUseCase(fixture, closer).ExecuteClosing(context.Background(), task.ID, []models.WorktreeProcess{devServer})

	require.NoError(t, err)
	assert.Len(t, fixture.worktrees.committed, 1)
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusAwaitingReview}, fixture.repo.statuses)
}
