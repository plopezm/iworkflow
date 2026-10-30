//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// continueMessage is sent after a resume so the agent carries on where it stopped.
const continueMessage = "Continúa con la tarea donde la dejaste."

// ResumeRateLimitedTasks resumes the tasks stopped by a provider usage limit once their retry time has come.
// Each execution looks at every rate-limited task in the order the repository lists them; a failure in one
// task does not stop the others. Overlapping executions run one after another, so the second one finds the
// tasks already resumed or rescheduled and resumes nothing.
type ResumeRateLimitedTasks struct {
	lifecycle *TaskLifecycle
	runtime   outbound.AgentRuntime
	resume    *ResumeTask
	send      *SendMessage

	// mu serializes the sweeps. It is held for the whole sweep.
	mu sync.Mutex
}

// NewResumeRateLimitedTasks builds the sweep on its ports, on the manual resume it reuses and on the
// message use case that sends the continuation.
func NewResumeRateLimitedTasks(lifecycle *TaskLifecycle, runtime outbound.AgentRuntime, resume *ResumeTask, send *SendMessage) *ResumeRateLimitedTasks {
	return &ResumeRateLimitedTasks{lifecycle: lifecycle, runtime: runtime, resume: resume, send: send}
}

// Execute resumes the due tasks and returns how many resumed. It is the RateLimitedResumer port.
func (u *ResumeRateLimitedTasks) Execute(ctx context.Context, now time.Time) (int, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	parked, err := u.lifecycle.Repository().ListByStatus(ctx, []models.TaskStatus{models.StatusRateLimited})
	if err != nil {
		return 0, err
	}
	resumed := 0
	for _, task := range parked {
		// A stopped task has no active process: its agent is stopped whether or not it is due yet.
		if u.runtime.IsRunning(ctx, task.ID) {
			if err := u.runtime.Stop(ctx, task.ID); err != nil {
				slog.Warn("could not stop the idle agent", "task_id", task.ID.String(), "error", err)
			}
		}
		// Without a schedule (the server stopped between recording the limit and scheduling it) the task
		// retries now; otherwise it would stay stopped forever.
		if task.UsageLimit.ResumeAt != nil && !task.UsageLimit.IsDue(now) {
			continue
		}
		if err := u.resumeOne(ctx, task.ID); err != nil {
			slog.Warn("could not resume the task after the usage limit", "task_id", task.ID.String(), "error", err)
			u.reschedule(ctx, task, now)
			continue
		}
		resumed++
	}
	return resumed, nil
}

// resumeOne restarts the agent and then sends the continuation. The continuation is only attempted once the
// restart succeeded.
func (u *ResumeRateLimitedTasks) resumeOne(ctx context.Context, id models.TaskID) error {
	if err := u.resume.Execute(ctx, id); err != nil {
		return err
	}
	return u.sendContinuation(ctx, id)
}

// sendContinuation sends the continuation through SendMessage in follow-up mode, as the Rust version does. The
// task is idle after a resume, so the agent receives it as a prompt.
func (u *ResumeRateLimitedTasks) sendContinuation(ctx context.Context, id models.TaskID) error {
	return u.send.Execute(ctx, id, continueMessage, nil, models.MessageModeFollowUp)
}

// reschedule delays the next attempt with the probe backoff. A failed write is logged, never returned, so one
// task cannot fail the whole sweep.
func (u *ResumeRateLimitedTasks) reschedule(ctx context.Context, task models.Task, now time.Time) {
	next := task.UsageLimit.AfterHit(nil, now)
	if err := u.lifecycle.Repository().SetUsageLimit(ctx, task.ID, next); err != nil {
		slog.Warn("could not reschedule the usage limit resume", "task_id", task.ID.String(), "error", err)
	}
}
