package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: resume_task::rejects_missing_worktree_without_starting_agent
func TestResumeTaskRejectsMissingWorktreeWithoutStartingAgent(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)
	fx.worktrees.exists = false

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.WorktreeMissing(task.WorktreePath))
	assert.Empty(t, fx.settings.asked)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.repo.statuses)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rust: resume_task::propagates_launch_settings_failure_without_starting_agent
func TestResumeTaskPropagatesLaunchSettingsFailureWithoutStartingAgent(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)
	fx.settings.err = werrors.Runtime("ajustes no disponibles")

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.Equal(t, werrors.Runtime("ajustes no disponibles"), err)
	assert.Equal(t, []string{task.WorktreePath}, fx.settings.asked)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.repo.statuses)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rust: resume_task::resumes_with_the_configured_runtime
func TestResumeTaskResumesWithTheConfiguredRuntime(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)
	fx.settings.options = models.LaunchOptions{Runtime: runtimeID(t, "pi")}

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.runtime.started, 1)
	assert.True(t, fx.runtime.started[0].resume)
	require.NotNil(t, fx.runtime.started[0].options.Runtime)
	assert.Equal(t, "pi", fx.runtime.started[0].options.Runtime.String())
}

// Rust: resume_task::restarts_agent_with_resume_and_goes_idle
func TestResumeTaskRestartsTheAgentWithResumeAndGoesIdle(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.runtime.started, 1)
	call := fx.runtime.started[0]
	assert.Equal(t, task.ID, call.id)
	assert.Equal(t, task.WorktreePath, call.cwd)
	assert.True(t, call.resume)
	assert.Equal(t, fx.sink, call.sink)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
	assert.Equal(t, []*string{nil}, fx.repo.errorTexts)
	assert.Len(t, fx.publisher.published, 1)
	assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
	assert.Empty(t, fx.runtime.sent)
	assert.Empty(t, fx.repo.usageWrites)
}

// Rust: resume_task::rejects_resume_when_runtime_lacks_the_capability
func TestResumeTaskRejectsResumeWhenTheRuntimeLacksTheCapability(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)
	fx.runtime.capabilities = models.MinimalAgentCapabilities()

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.Equal(t, werrors.Unsupported("el agente no permite reanudar la sesión anterior"), err)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.repo.statuses)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rust: resume_task::strips_model_and_thinking_the_runtime_does_not_support
func TestResumeTaskStripsModelAndThinkingTheRuntimeDoesNotSupport(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)
	fx.runtime.capabilities = models.AgentCapabilities{Resume: true}
	thinking := "high"
	fx.settings.options = models.LaunchOptions{
		Model:    &models.LaunchModel{Provider: "anthropic", Model: "claude-5"},
		Thinking: &thinking,
	}

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.runtime.started, 1)
	assert.Nil(t, fx.runtime.started[0].options.Model)
	assert.Nil(t, fx.runtime.started[0].options.Thinking)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
}

// Rust: resume_task::rejects_running_task
func TestResumeTaskRejectsRunningTask(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	fx := newResumeFixture(task)

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.InvalidState("reanudar", "running"))
	assert.Empty(t, fx.settings.asked)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.repo.statuses)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rule from source: resume_task.rs::execute reads the task first, so an unknown id is NotFound and nothing else runs.
func TestResumeTaskFailsWithNotFoundForUnknownTask(t *testing.T) {
	t.Parallel()
	fx := newResumeFixture()
	task := newTask(models.StatusInterrupted)

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.NotFound(task.ID.String()))
	assert.Empty(t, fx.settings.asked)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.repo.statuses)
}

// Rule from source: task_status.rs::can_resume. Only interrupted, rate-limited, failed, conflict and
// awaiting-review tasks resume; every other status is rejected with the same InvalidState and changes nothing.
func TestResumeTaskAcceptsExactlyTheStatusesThatCanResume(t *testing.T) {
	t.Parallel()
	resumable := map[models.TaskStatus]bool{
		models.StatusInterrupted:    true,
		models.StatusRateLimited:    true,
		models.StatusFailed:         true,
		models.StatusConflict:       true,
		models.StatusAwaitingReview: true,
	}
	for _, status := range models.AllTaskStatuses {
		t.Run(status.String(), func(t *testing.T) {
			t.Parallel()
			task := newTask(status)
			fx := newResumeFixture(task)

			err := fx.resumeTask().Execute(context.Background(), task.ID)

			if resumable[status] {
				require.NoError(t, err)
				assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
				return
			}
			require.ErrorIs(t, err, werrors.InvalidState("reanudar", status.String()))
			assert.Empty(t, fx.runtime.started)
			assert.Empty(t, fx.repo.statuses)
			assert.Equal(t, task, fx.repo.tasks[task.ID])
		})
	}
}

// Rule from source: resume_task.rs::execute applies prepared.with_task_limits(task.limits): the task's own
// limit wins per resource, the global one fills the rest.
func TestResumeTaskAppliesTheTaskLimitsOverTheGlobalOnes(t *testing.T) {
	t.Parallel()
	taskMemory := uint32(2048)
	globalMemory := uint32(1024)
	globalCPU := uint32(50)
	task := newTask(models.StatusInterrupted)
	task.Limits = models.ResourceLimits{MemoryMB: &taskMemory}
	fx := newResumeFixture(task)
	fx.settings.options = models.LaunchOptions{
		Limits: models.ResourceLimits{MemoryMB: &globalMemory, CPUPercent: &globalCPU},
	}

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, fx.runtime.started, 1)
	limits := fx.runtime.started[0].options.Limits
	require.NotNil(t, limits.MemoryMB)
	require.NotNil(t, limits.CPUPercent)
	assert.Equal(t, taskMemory, *limits.MemoryMB)
	assert.Equal(t, globalCPU, *limits.CPUPercent)
}

// New behavior (not in Rust): a second resume of the same task while the first one is still starting the
// agent is rejected, so two overlapping calls never start two agents for one task.
func TestResumeTaskRejectsASecondResumeWhileTheFirstIsInFlight(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)
	fx.runtime.startEntered = make(chan struct{}, 1)
	fx.runtime.startGate = make(chan struct{})
	use := fx.resumeTask()

	var wg sync.WaitGroup
	var firstErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		firstErr = use.Execute(context.Background(), task.ID)
	}()
	waitForSignal(t, fx.runtime.startEntered)

	secondErr := use.Execute(context.Background(), task.ID)
	close(fx.runtime.startGate)
	wg.Wait()

	require.ErrorIs(t, secondErr, werrors.InvalidState("reanudar", "interrupted"))
	require.NoError(t, firstErr)
	assert.Len(t, fx.runtime.started, 1)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
}

// Rule from source: resume_task.rs::execute propagates a runtime start failure and does not go idle; the
// task stays stopped so a later tick can retry it.
func TestResumeTaskKeepsTheTaskStoppedWhenTheStartFails(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusInterrupted)
	fx := newResumeFixture(task)
	startErr := errors.New("spawn failed")
	fx.runtime.startErrs[task.ID] = startErr

	err := fx.resumeTask().Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, startErr)
	assert.Empty(t, fx.repo.statuses)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}
