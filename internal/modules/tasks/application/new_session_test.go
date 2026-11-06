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

func (f *resumeFixture) newSession(flusher application.EventFlusher, guard *application.RestartGuard) *application.NewSession {
	related := application.NewPrepareRelatedLaunch(application.NewListTaskRelatedProjects(f.related, relatedWorkspaces()), f.probe)
	return application.NewNewSession(f.lifecycle, f.worktrees, f.runtime, f.settings, f.sink, related, flusher, guard)
}

func TestNewSessionStopsTheLiveAgentAndStartsItWithoutResumeThenGoesIdle(t *testing.T) {
	t.Parallel()
	for _, status := range []models.TaskStatus{models.StatusIdle, models.StatusRunning} {
		task := newTask(status)
		fx := newResumeFixture(task)
		fx.runtime.running[task.ID] = true

		err := fx.newSession(flushRecorder{runtime: fx.runtime}, application.NewRestartGuard()).Execute(context.Background(), task.ID)

		require.NoError(t, err, status)
		assert.Equal(t, []string{"stop", "flush", "start"}, fx.runtime.calls, status)
		require.Len(t, fx.runtime.started, 1, status)
		assert.False(t, fx.runtime.started[0].resume, "a new session never continues the previous one")
		assert.Equal(t, task.WorktreePath, fx.runtime.started[0].cwd)
		assert.Empty(t, fx.runtime.sent, "the initial prompt is not sent again")
		assert.Equal(t, 1, fx.repo.eventDeletes, "the history is deleted")
		assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
	}
}

func TestNewSessionOfAStoppedTaskStartsAFreshSessionWithoutStopping(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)

	err := fx.newSession(flushRecorder{runtime: fx.runtime}, application.NewRestartGuard()).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []string{"start"}, fx.runtime.calls)
	assert.False(t, fx.runtime.started[0].resume)
	assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
}

func TestNewSessionIsRefusedForStatusesWithoutAnAgentToRestart(t *testing.T) {
	t.Parallel()
	for _, status := range []models.TaskStatus{models.StatusBacklog, models.StatusStarting, models.StatusMerged, models.StatusDiscarded} {
		task := newTask(status)
		fx := newResumeFixture(task)

		err := fx.newSession(flushRecorder{runtime: fx.runtime}, application.NewRestartGuard()).Execute(context.Background(), task.ID)

		require.Error(t, err, status)
		assert.Empty(t, fx.runtime.calls, status)
		assert.Zero(t, fx.repo.eventDeletes, status)
		assert.Equal(t, task, fx.repo.tasks[task.ID], status)
	}
}

func TestNewSessionWithAMissingWorktreeChangesNothing(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.worktrees.exists = false

	err := fx.newSession(flushRecorder{runtime: fx.runtime}, application.NewRestartGuard()).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.WorktreeMissing(task.WorktreePath))
	assert.Empty(t, fx.runtime.calls, "the live agent is not stopped when the restart cannot happen")
}

func TestNewSessionMarksTheTaskFailedWhenTheAgentDoesNotStart(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.runtime.startErrs[task.ID] = werrors.Runtime("no arranca")

	err := fx.newSession(flushRecorder{runtime: fx.runtime}, application.NewRestartGuard()).Execute(context.Background(), task.ID)

	require.Error(t, err)
	assert.Equal(t, models.StatusFailed, fx.repo.tasks[task.ID].Status)
	assert.Zero(t, fx.repo.eventDeletes, "the history is kept when the new session did not start")
}

func TestNewSessionMarksTheTaskFailedWhenTheHistoryCannotBeDeleted(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.repo.deleteEventsErr = werrors.Repository("disco lleno")

	err := fx.newSession(flushRecorder{runtime: fx.runtime}, application.NewRestartGuard()).Execute(context.Background(), task.ID)

	require.Equal(t, werrors.Repository("disco lleno"), err)
	assert.Equal(t, models.StatusFailed, fx.repo.tasks[task.ID].Status)
}

func TestNewSessionDoesNotRecordTheStopAsACrash(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	guard := application.NewRestartGuard()
	agentEvents := application.NewHandleAgentEvent(fx.lifecycle, clock)
	agentEvents.SetRestartGuard(guard)
	queue := application.NewQueuedAgentEventSink(agentEvents, 16)
	defer queue.Shutdown()
	fx.runtime.exitTo = queue

	err := fx.newSession(queue, guard).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.NotContains(t, fx.repo.statuses, models.StatusInterrupted)
	assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
}
