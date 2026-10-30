package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: resume_rate_limited_tasks::CONTINUE_MESSAGE
const resumeContinueMessage = "Continúa con la tarea donde la dejaste."

// Rust: resume_rate_limited_tasks::resumes_a_parked_task_that_has_no_schedule
func TestResumeRateLimitedResumesParkedTaskWithoutSchedule(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{})
	fx := newResumeFixture(task)

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 1, resumed)
	require.Len(t, fx.runtime.started, 1)
	assert.Equal(t, task.ID, fx.runtime.started[0].id)
	assert.True(t, fx.runtime.started[0].resume)
	assert.Empty(t, fx.runtime.stopped)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
	assert.Empty(t, fx.repo.usageWrites)
	assert.Equal(t, models.StatusIdle, fx.repo.tasks[task.ID].Status)
}

// Rust: resume_rate_limited_tasks::resumes_due_task_and_sends_continuation
func TestResumeRateLimitedResumesDueTaskAndSendsContinuation(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: 1})
	fx := newResumeFixture(task)

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 1, resumed)
	require.Len(t, fx.runtime.started, 1)
	require.Len(t, fx.runtime.sent, 1)
	assert.Equal(t, task.ID, fx.runtime.sent[0].id)
	command, isPrompt := fx.runtime.sent[0].command.(models.PromptCommand)
	require.True(t, isPrompt)
	assert.Equal(t, resumeContinueMessage, command.Message)
	assert.Empty(t, command.Images)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
	assert.Empty(t, fx.repo.usageWrites)
}

// Rust: resume_rate_limited_tasks::leaves_not_due_task_parked_but_stops_its_idle_process
func TestResumeRateLimitedLeavesNotDueTaskParkedButStopsItsIdleProcess(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(10 * time.Minute), Attempts: 1})
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	assert.Equal(t, []models.TaskID{task.ID}, fx.runtime.stopped)
	assert.False(t, fx.runtime.IsRunning(context.Background(), task.ID))
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.runtime.sent)
	assert.Empty(t, fx.repo.statuses)
	assert.Empty(t, fx.repo.usageWrites)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rust: resume_rate_limited_tasks::failed_resume_reschedules_with_backoff
func TestResumeRateLimitedFailedResumeReschedulesWithBackoff(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: 1})
	fx := newResumeFixture(task)
	fx.worktrees.exists = false

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	require.Len(t, fx.repo.usageWrites, 1)
	write := fx.repo.usageWrites[0]
	assert.Equal(t, task.ID, write.id)
	assert.Equal(t, uint32(2), write.state.Attempts)
	require.NotNil(t, write.state.ResumeAt)
	assert.True(t, write.state.ResumeAt.Equal(fixedNow.Add(10*time.Minute)))
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.repo.statuses)
	assert.Equal(t, models.StatusRateLimited, fx.repo.tasks[task.ID].Status)
}

// Rule from source: usage_limit.rs::is_due. A schedule equal to now is already due.
func TestResumeRateLimitedResumesTaskExactlyAtItsResumeInstant(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(0), Attempts: 1})
	fx := newResumeFixture(task)

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 1, resumed)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
}

// Rule from source: resume_rate_limited_tasks.rs::execute stops a running process only; a not due task that
// is not running is skipped without any runtime call.
func TestResumeRateLimitedSkipsNotDueTaskWithoutStoppingWhenNotRunning(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(time.Hour), Attempts: 1})
	fx := newResumeFixture(task)

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	assert.Empty(t, fx.runtime.stopped)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.repo.usageWrites)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rule from source: resume_rate_limited_tasks.rs::execute logs a failed stop and goes on with the schedule check.
func TestResumeRateLimitedKeepsResumingWhenStoppingTheIdleProcessFails(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: 1})
	fx := newResumeFixture(task)
	fx.runtime.running[task.ID] = true
	fx.runtime.stopErr = errors.New("stop failed")

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 1, resumed)
	assert.Equal(t, []models.TaskID{task.ID}, fx.runtime.stopped)
	require.Len(t, fx.runtime.started, 1)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
}

// Rule from source: resume_rate_limited_tasks.rs::execute: a failure in one task does not stop the others;
// only the failed task is rescheduled. Tasks are handled in the order the repository lists them.
func TestResumeRateLimitedReschedulesOnlyTheFailedTaskAndKeepsGoing(t *testing.T) {
	t.Parallel()
	first := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second)})
	second := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second)})
	fx := newResumeFixture(first, second)
	fx.runtime.startErrs[second.ID] = errors.New("spawn failed")

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 1, resumed)
	require.Len(t, fx.runtime.started, 2)
	assert.Equal(t, first.ID, fx.runtime.started[0].id)
	assert.Equal(t, second.ID, fx.runtime.started[1].id)
	require.Len(t, fx.runtime.sent, 1)
	assert.Equal(t, first.ID, fx.runtime.sent[0].id)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
	require.Len(t, fx.repo.usageWrites, 1)
	assert.Equal(t, second.ID, fx.repo.usageWrites[0].id)
	assert.Equal(t, uint32(1), fx.repo.usageWrites[0].state.Attempts)
	assert.Equal(t, models.StatusRateLimited, fx.repo.tasks[second.ID].Status)
}

// Rule from source: usage_limit.rs::after_hit probes with 5 minutes doubled per attempt, capped at 1 hour, and
// the attempt counter grows by one on every hit.
func TestResumeRateLimitedBacksOffExponentiallyAndCapsTheDelay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		attempts uint32
		wait     time.Duration
		next     uint32
	}{
		{name: "first retry waits the base delay", attempts: 0, wait: 5 * time.Minute, next: 1},
		{name: "second attempt doubles", attempts: 1, wait: 10 * time.Minute, next: 2},
		{name: "fourth attempt doubles again", attempts: 3, wait: 40 * time.Minute, next: 4},
		{name: "fifth attempt reaches the cap", attempts: 4, wait: time.Hour, next: 5},
		{name: "late attempts stay at the cap", attempts: 12, wait: time.Hour, next: 13},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: testCase.attempts})
			fx := newResumeFixture(task)
			fx.worktrees.exists = false

			resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

			require.NoError(t, err)
			assert.Equal(t, 0, resumed)
			require.Len(t, fx.repo.usageWrites, 1)
			write := fx.repo.usageWrites[0]
			require.NotNil(t, write.state.ResumeAt)
			assert.True(t, write.state.ResumeAt.Equal(fixedNow.Add(testCase.wait)))
			assert.Equal(t, testCase.next, write.state.Attempts)
		})
	}
}

// Rule from source: resume_rate_limited_tasks.rs::reschedule logs a failed write and returns the count anyway.
func TestResumeRateLimitedRescheduleFailureIsLoggedNotReturned(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: 1})
	fx := newResumeFixture(task)
	fx.worktrees.exists = false
	fx.repo.usageErr = errors.New("database is locked")

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	assert.Empty(t, fx.repo.usageWrites)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rule from source: resume_rate_limited_tasks.rs::resume runs the resume and then the continuation; a failed
// continuation counts as a failed resume and reschedules the task. The task is already idle at that point
// (Rust parity: the reschedule does not check the status).
func TestResumeRateLimitedFailedContinuationReschedulesAfterTheResume(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second)})
	fx := newResumeFixture(task)
	fx.runtime.sendErr = errors.New("send failed")

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
	require.Len(t, fx.repo.usageWrites, 1)
	assert.Equal(t, task.ID, fx.repo.usageWrites[0].id)
	assert.Equal(t, uint32(1), fx.repo.usageWrites[0].state.Attempts)
	require.NotNil(t, fx.repo.usageWrites[0].state.ResumeAt)
	assert.True(t, fx.repo.usageWrites[0].state.ResumeAt.Equal(fixedNow.Add(5*time.Minute)))
}

// Rule from source: resume_rate_limited_tasks.rs::execute returns the list failure and changes nothing.
func TestResumeRateLimitedListFailureReturnsErrorAndChangesNothing(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: 1})
	fx := newResumeFixture(task)
	listErr := errors.New("database is locked")
	fx.repo.listErr = listErr

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.ErrorIs(t, err, listErr)
	assert.Equal(t, 0, resumed)
	assert.Empty(t, fx.runtime.stopped)
	assert.Empty(t, fx.runtime.started)
	assert.Empty(t, fx.runtime.sent)
	assert.Empty(t, fx.repo.statuses)
	assert.Empty(t, fx.repo.usageWrites)
	assert.Equal(t, task, fx.repo.tasks[task.ID])
}

// Rule from source: resume_rate_limited_tasks.rs::execute lists only the rate-limited tasks; an idle task with a
// past schedule is never touched.
func TestResumeRateLimitedOnlyListsRateLimitedTasks(t *testing.T) {
	t.Parallel()
	idle := newTask(models.StatusIdle)
	idle.UsageLimit = models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: 1}
	parked := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second)})
	fx := newResumeFixture(idle, parked)

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 1, resumed)
	assert.Equal(t, [][]models.TaskStatus{{models.StatusRateLimited}}, fx.repo.listedWith)
	require.Len(t, fx.runtime.started, 1)
	assert.Equal(t, parked.ID, fx.runtime.started[0].id)
	assert.Equal(t, idle, fx.repo.tasks[idle.ID])
}

// New behavior (not in Rust, which ticks sequentially): two overlapping executions resume each task once. The
// second waits for the first and then finds nothing due, so the agent starts exactly once.
func TestResumeRateLimitedOverlappingTicksResumeEachTaskOnce(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second)})
	fx := newResumeFixture(task)
	fx.runtime.startEntered = make(chan struct{}, 1)
	fx.runtime.startGate = make(chan struct{})
	use := fx.rateLimited()

	var wg sync.WaitGroup
	var first int
	var firstErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		first, firstErr = use.Execute(context.Background(), fixedNow)
	}()
	waitForSignal(t, fx.runtime.startEntered)

	var second int
	var secondErr error
	wg.Add(1)
	go func() {
		defer wg.Done()
		second, secondErr = use.Execute(context.Background(), fixedNow)
	}()
	close(fx.runtime.startGate)
	wg.Wait()

	require.NoError(t, firstErr)
	require.NoError(t, secondErr)
	assert.Equal(t, 1, first)
	assert.Equal(t, 0, second)
	assert.Len(t, fx.runtime.started, 1)
	assert.Len(t, fx.runtime.sent, 1)
	assert.Equal(t, []models.TaskStatus{models.StatusIdle}, fx.repo.statuses)
	assert.Empty(t, fx.repo.usageWrites)
}

// Rule from source: a failed agent start is a failed resume: the task stays stopped and is rescheduled with the
// backoff, exactly like a missing worktree.
func TestResumeRateLimitedFailedStartKeepsTheTaskStopped(t *testing.T) {
	t.Parallel()
	task := parkedTask(models.UsageLimitState{ResumeAt: instantAt(-time.Second), Attempts: 0})
	fx := newResumeFixture(task)
	fx.runtime.startErrs[task.ID] = werrors.Runtime("no se pudo arrancar")

	resumed, err := fx.rateLimited().Execute(context.Background(), fixedNow)

	require.NoError(t, err)
	assert.Equal(t, 0, resumed)
	assert.Empty(t, fx.repo.statuses)
	require.Len(t, fx.repo.usageWrites, 1)
	assert.Equal(t, uint32(1), fx.repo.usageWrites[0].state.Attempts)
}
