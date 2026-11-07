package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// hookStep names a hook run in the runtime call log, for example "hook pre start".
func hookStep(req models.HookRequest) string {
	return "hook " + string(req.Phase) + " " + string(req.Event)
}

// Rule: the pre start hook runs once, in the worktree, before the agent starts; the post start hook runs after the
// prompt was sent. Both see the task with its branch and worktree, and the workspace folder.
func TestStartTaskHooksRunPreBeforeLaunchAndPostAfterPrompt(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	hooks := &recordingHooks{onRun: func(req models.HookRequest) { fx.runtime.note(hookStep(req)) }}
	useCase := fx.start(newUUID)
	useCase.SetLifecycleHooks(hooks)

	started, err := useCase.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	assert.Equal(t, []string{"hook pre start", "start", "send", "hook post start"}, fx.runtime.calls)
	require.Len(t, hooks.calls, 2)
	require.Len(t, fx.worktrees.created, 1)

	pre := hooks.calls[0]
	assert.Equal(t, models.HookEventStart, pre.Event)
	assert.Equal(t, models.HookPhasePre, pre.Phase)
	assert.Equal(t, models.HookResult(""), pre.Result)
	assert.Equal(t, "/repos/demo", pre.WorkspacePath)
	assert.Equal(t, task.ID, pre.Task.ID)
	assert.Equal(t, models.StatusStarting, pre.Task.Status)
	assert.Equal(t, fx.worktrees.created[0].branch, pre.Task.Branch)
	assert.Equal(t, fx.worktrees.created[0].path, pre.Task.WorktreePath)

	post := hooks.calls[1]
	assert.Equal(t, models.HookEventStart, post.Event)
	assert.Equal(t, models.HookPhasePost, post.Phase)
	assert.Equal(t, models.HookResultOK, post.Result)
	assert.Equal(t, models.StatusStarting, post.Task.Status)
}

// Rule: a pre start hook that fails, blocking or not, ends the task in failed like a failed launch: no agent starts,
// the worktree is kept, and the failure text is stored on the task. The post hook never runs.
func TestStartTaskBlockingPreHookEndsTaskFailedWithoutLaunch(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		hookErr error
	}{
		{name: "blocked", hookErr: fmt.Errorf("%w: exit 1", werrors.ErrHookBlocked)},
		{name: "other failure", hookErr: errors.New("hook timed out")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			task := newTask(models.StatusBacklog)
			fx := newStartFixture(task)
			hooks := &recordingHooks{preErr: tc.hookErr}
			useCase := fx.start(newUUID)
			useCase.SetLifecycleHooks(hooks)

			started, err := useCase.Execute(context.Background(), task.ID)

			require.NoError(t, err)
			assert.Equal(t, models.StatusFailed, started.Status)
			assert.Equal(t, []models.TaskStatus{models.StatusFailed}, fx.repo.statuses)
			require.NotEmpty(t, fx.repo.errorTexts)
			require.NotNil(t, fx.repo.errorTexts[0])
			assert.Contains(t, *fx.repo.errorTexts[0], tc.hookErr.Error())
			assert.Empty(t, fx.runtime.started, "no agent starts when a pre hook fails")
			assert.Empty(t, fx.runtime.sent)
			require.Len(t, fx.worktrees.created, 1, "the worktree was created before the hook ran")
			assert.Empty(t, fx.worktrees.removed, "the worktree is kept")
			require.Len(t, hooks.calls, 1, "the post hook never runs after a failed pre hook")
			assert.Equal(t, models.HookPhasePre, hooks.calls[0].Phase)
		})
	}
}

// Rule: a post start hook failure never fails the start: the task keeps running and no transition is recorded.
func TestStartTaskPostHookFailureDoesNotFailStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	hooks := &recordingHooks{postErr: errors.New("hook exploded"), warnings: []string{"slow hook"}}
	useCase := fx.start(newUUID)
	useCase.SetLifecycleHooks(hooks)

	started, err := useCase.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusStarting, started.Status)
	assert.Nil(t, started.Error)
	assert.Empty(t, fx.repo.statuses, "a failed post hook records no transition")
	assert.Len(t, fx.runtime.sent, 1, "the prompt was still sent")
	require.Len(t, hooks.calls, 2)
	assert.Equal(t, models.HookPhasePost, hooks.calls[1].Phase)
}

// Rule: the post start hook runs only after a successful launch; a launch that fails runs the pre hook alone.
func TestStartTaskPostHookDoesNotRunWhenAgentCannotStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	fx.runtime.startErrs[task.ID] = werrors.Runtime("no existe gentle-shell")
	hooks := &recordingHooks{}
	useCase := fx.start(newUUID)
	useCase.SetLifecycleHooks(hooks)

	started, err := useCase.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, started.Status)
	require.Len(t, hooks.calls, 1)
	assert.Equal(t, models.HookPhasePre, hooks.calls[0].Phase)
}

// Rule: a blocked pre finish changes nothing: the task keeps its status, nothing is stopped, committed or published,
// and the error wraps ErrHookBlocked. The hook sees the workspace folder and the task before the finish.
func TestFinishTaskBlockingPreHookChangesNothing(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		hookErr error
	}{
		{name: "blocked", hookErr: fmt.Errorf("%w: lint failed", werrors.ErrHookBlocked)},
		{name: "other failure", hookErr: errors.New("hook timed out")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			task, fx := newFinishFixture(models.StatusIdle)
			fx.runtime.running = true
			hooks := &recordingHooks{preErr: tc.hookErr}
			useCase := fx.useCase()
			useCase.SetLifecycleHooks(hooks)

			err := useCase.Execute(context.Background(), task.ID)

			require.ErrorIs(t, err, werrors.ErrHookBlocked)
			assert.ErrorIs(t, err, tc.hookErr)
			assert.Empty(t, fx.repo.statuses, "no transition happens")
			assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
			assert.Empty(t, fx.worktrees.committed, "the pre hook runs before the commit")
			assert.Empty(t, fx.runtime.stopped)
			assert.Zero(t, fx.runtime.runningCalls, "the agent is not even checked")
			assert.Empty(t, fx.publisher.published)
			require.Len(t, hooks.calls, 1)
			pre := hooks.calls[0]
			assert.Equal(t, models.HookEventFinish, pre.Event)
			assert.Equal(t, models.HookPhasePre, pre.Phase)
			assert.Equal(t, "/repos/demo", pre.WorkspacePath)
			assert.Equal(t, models.StatusIdle, pre.Task.Status)
		})
	}
}

// Rule: the post finish hook runs once the task awaits review, before the attachment cleanup, and its failure does
// not fail the finish.
func TestFinishTaskPostHookRunsAfterAwaitingReviewAndItsFailureDoesNotFailFinish(t *testing.T) {
	t.Parallel()
	task, fx := newFinishFixture(models.StatusIdle)
	var statusesAtPost []models.TaskStatus
	var committedAtPost []string
	hooks := &recordingHooks{postErr: errors.New("hook exploded")}
	hooks.onRun = func(req models.HookRequest) {
		if req.Phase == models.HookPhasePost {
			statusesAtPost = slices.Clone(fx.repo.statuses)
			committedAtPost = slices.Clone(fx.worktrees.committed)
		}
	}
	useCase := fx.useCase()
	useCase.SetLifecycleHooks(hooks)

	err := useCase.Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusAwaitingReview}, statusesAtPost)
	assert.Len(t, committedAtPost, 1, "the post hook runs after the commit")
	assert.Equal(t, 1, fx.attachments.deletedByTask, "the cleanup still runs after the post hook")
	require.Len(t, hooks.calls, 2)
	post := hooks.calls[1]
	assert.Equal(t, models.HookEventFinish, post.Event)
	assert.Equal(t, models.HookPhasePost, post.Phase)
	assert.Equal(t, models.HookResultOK, post.Result)
	assert.Equal(t, models.StatusAwaitingReview, post.Task.Status)
	assert.Equal(t, "/repos/demo", post.WorkspacePath)
}

// Rule: the post finish hook does not run when the consolidation fails, because the task is not finished.
func TestFinishTaskPostHookDoesNotRunWhenConsolidationFails(t *testing.T) {
	t.Parallel()
	task, fx := newFinishFixture(models.StatusIdle)
	fx.worktrees.commitErr = werrors.Git("index.lock")
	hooks := &recordingHooks{}
	useCase := fx.useCase()
	useCase.SetLifecycleHooks(hooks)

	err := useCase.Execute(context.Background(), task.ID)

	require.Error(t, err)
	require.Len(t, hooks.calls, 1)
	assert.Equal(t, models.HookPhasePre, hooks.calls[0].Phase)
}

// gatedHooks holds the first pre hook run until the test releases it, so a second action can start while the first
// one is inside its pre hook. The other runs return at once.
type gatedHooks struct {
	entered chan struct{}
	release chan struct{}
	mu      sync.Mutex
	preRuns int
}

func newGatedHooks() *gatedHooks {
	return &gatedHooks{entered: make(chan struct{}, 1), release: make(chan struct{})}
}

func (h *gatedHooks) Run(_ context.Context, req models.HookRequest) (models.HookReport, error) {
	if req.Phase != models.HookPhasePre {
		return models.HookReport{}, nil
	}
	h.mu.Lock()
	h.preRuns++
	first := h.preRuns == 1
	h.mu.Unlock()
	if first {
		h.entered <- struct{}{}
		<-h.release
	}
	return models.HookReport{}, nil
}

// Review advisory (f): two finishes of one task never run the pre hooks twice. While the first is inside its pre
// hook, the second is refused without running the hooks, and the first still finishes the task.
func TestFinishTaskConcurrentFinishRunsPreHooksOnce(t *testing.T) {
	t.Parallel()
	task, fx := newFinishFixture(models.StatusIdle)
	hooks := newGatedHooks()
	useCase := fx.useCase()
	useCase.SetLifecycleHooks(hooks)
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- useCase.Execute(context.Background(), task.ID)
	}()
	<-hooks.entered

	secondErr := useCase.Execute(context.Background(), task.ID)
	close(hooks.release)
	firstErr := <-firstDone

	require.Error(t, secondErr, "the second finish must be refused while the first one is in flight")
	require.NoError(t, firstErr)
	assert.Equal(t, 1, hooks.preRuns, "the pre hooks run once for the two finishes")
	assert.Equal(t, []models.TaskStatus{models.StatusFinishing, models.StatusAwaitingReview}, fx.repo.statuses)
}

// Review advisory (f): a blocked finish releases its claim, so the task can be finished again once the hook passes.
func TestFinishTaskBlockedPreHookReleasesTheClaim(t *testing.T) {
	t.Parallel()
	task, fx := newFinishFixture(models.StatusIdle)
	hooks := &recordingHooks{preErr: fmt.Errorf("%w: lint failed", werrors.ErrHookBlocked)}
	useCase := fx.useCase()
	useCase.SetLifecycleHooks(hooks)

	require.ErrorIs(t, useCase.Execute(context.Background(), task.ID), werrors.ErrHookBlocked)
	hooks.preErr = nil

	require.NoError(t, useCase.Execute(context.Background(), task.ID))
	assert.Equal(t, models.StatusAwaitingReview, fx.repo.tasks[task.ID].Status)
}

// cancelAwareRepository fails the status writes once the context is done, as the database does for a cancelled
// request. A transition that is not detached from the request shows up as a failed write.
type cancelAwareRepository struct {
	*fakeRepository
}

func (r cancelAwareRepository) UpdateStatus(ctx context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.fakeRepository.UpdateStatus(ctx, id, status, errorText)
}

// Review advisory (h): the request is cancelled while the pre start hook runs. The task must still end in failed with
// the hook message, not be left in starting, and the start reports that outcome like any failed launch.
func TestStartTaskCancelledPreHookStillMarksTheTaskFailed(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	fx := newStartFixture(task)
	lifecycle, _ := newLifecycle(cancelAwareRepository{fakeRepository: fx.repo})
	fx.lifecycle = lifecycle
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hooks := &recordingHooks{
		preErr: fmt.Errorf("hook interrupted: %w", context.Canceled),
		onRun:  func(models.HookRequest) { cancel() },
	}
	useCase := fx.start(newUUID)
	useCase.SetLifecycleHooks(hooks)

	started, err := useCase.Execute(ctx, task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusFailed, started.Status)
	require.NotNil(t, started.Error)
	assert.Contains(t, *started.Error, "un hook previo al inicio falló")
	assert.Contains(t, *started.Error, context.Canceled.Error())
	assert.Equal(t, models.StatusFailed, fx.repo.tasks[task.ID].Status, "the task is not left in starting")
	assert.Empty(t, fx.runtime.started, "no agent starts")
}

// Review advisory (e): a post finish hook is not started when the request was already cancelled. The action is done,
// so the hook is skipped and the finish still succeeds.
func TestFinishTaskSkipsThePostHookWhenTheRequestWasCancelled(t *testing.T) {
	t.Parallel()
	task, fx := newFinishFixture(models.StatusIdle)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hooks := &recordingHooks{onRun: func(req models.HookRequest) {
		if req.Phase == models.HookPhasePre {
			cancel()
		}
	}}
	useCase := fx.useCase()
	useCase.SetLifecycleHooks(hooks)

	require.NoError(t, useCase.Execute(ctx, task.ID))

	require.Len(t, hooks.calls, 1, "only the pre hook runs")
	assert.Equal(t, models.StatusAwaitingReview, fx.repo.tasks[task.ID].Status)
}

// Review advisory (e): a post finish hook cancelled while it runs stops at its context and never fails the finish.
func TestFinishTaskPostHookCancelledMidRunDoesNotFailFinish(t *testing.T) {
	t.Parallel()
	task, fx := newFinishFixture(models.StatusIdle)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hooks := &recordingHooks{
		postErr: fmt.Errorf("hook stopped: %w", context.Canceled),
		onRun: func(req models.HookRequest) {
			if req.Phase == models.HookPhasePost {
				cancel()
			}
		},
	}
	useCase := fx.useCase()
	useCase.SetLifecycleHooks(hooks)

	require.NoError(t, useCase.Execute(ctx, task.ID))

	assert.Equal(t, models.StatusAwaitingReview, fx.repo.tasks[task.ID].Status)
	require.Len(t, hooks.calls, 2)
}
