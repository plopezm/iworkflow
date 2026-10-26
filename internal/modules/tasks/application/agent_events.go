//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// HandleAgentEvent records every event of an agent and applies the status transitions and the usage
// limit schedule that the event provokes. It is the receiver of the runtime events.
type HandleAgentEvent struct {
	lifecycle *TaskLifecycle
	clock     Clock
}

// NewHandleAgentEvent builds the use case.
func NewHandleAgentEvent(lifecycle *TaskLifecycle, clock Clock) *HandleAgentEvent {
	return &HandleAgentEvent{lifecycle: lifecycle, clock: clock}
}

// Execute records the event and applies the changes it provokes. Non-persistent events are published
// without loading the task.
func (u *HandleAgentEvent) Execute(ctx context.Context, id models.TaskID, event models.AgentEvent) error {
	var task *models.Task
	if event.IsPersistent() {
		loaded, err := u.lifecycle.Load(ctx, id)
		if err != nil {
			return err
		}
		task = &loaded
	}
	var next models.TaskStatus
	var nextError *string
	hasNext := false
	if task != nil {
		next, nextError, hasNext = task.Status.AfterAgentEvent(event)
	}
	var usageLimit *models.UsageLimitState
	if task != nil {
		usageLimit = u.usageLimitAfter(task, event)
	}
	if err := u.lifecycle.Record(ctx, id, event); err != nil {
		return err
	}
	if hasNext {
		if err := u.lifecycle.Transition(ctx, id, next, nextError); err != nil {
			return err
		}
	}
	if usageLimit != nil {
		if err := u.lifecycle.Repository().SetUsageLimit(ctx, id, *usageLimit); err != nil {
			return err
		}
	}
	return nil
}

// usageLimitAfter returns the new resume schedule the event provokes, if any: a usage limit computes it
// with backoff, and a normal turn that completes resets the attempts.
func (u *HandleAgentEvent) usageLimitAfter(task *models.Task, event models.AgentEvent) *models.UsageLimitState {
	now := u.clock()
	switch typed := event.(type) {
	case models.UsageLimitReached:
		if !task.Status.HasLiveAgent() {
			return nil
		}
		var resetsAt *time.Time
		if typed.ResetsAt != nil {
			if parsed, ok := models.FromEpochSecs(*typed.ResetsAt); ok {
				resetsAt = &parsed
			}
		}
		state := task.UsageLimit.AfterHit(resetsAt, now)
		return &state
	case models.RunSettled:
		// A late settle of the turn that hit the limit does not count as a success.
		if task.UsageLimit.Attempts > 0 && task.Status != models.StatusRateLimited {
			state := models.UsageLimitState{}
			return &state
		}
		return nil
	default:
		return nil
	}
}

// Handle implements the runtime sink: a failure is logged, never returned to the runtime.
func (u *HandleAgentEvent) Handle(ctx context.Context, taskID models.TaskID, event models.AgentEvent) {
	if err := u.Execute(ctx, taskID, event); err != nil {
		slog.Warn("could not record the agent event", "task_id", taskID.String(), "error", err)
	}
}

// AnswerDialog answers a dialog the agent opened and records it as resolved.
type AnswerDialog struct {
	lifecycle *TaskLifecycle
	runtime   outbound.AgentRuntime
}

// NewAnswerDialog builds the use case.
func NewAnswerDialog(lifecycle *TaskLifecycle, runtime outbound.AgentRuntime) *AnswerDialog {
	return &AnswerDialog{lifecycle: lifecycle, runtime: runtime}
}

// Execute forwards the answer to the agent, then records the resolution. It needs a live agent that
// declares dialogs.
func (u *AnswerDialog) Execute(ctx context.Context, id models.TaskID, dialogID string, answer models.DialogAnswer) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.HasLiveAgent() {
		return invalidState("responder diálogo", task.Status)
	}
	if !u.runtime.IsRunning(ctx, id) {
		return werrors.AgentNotRunning()
	}
	if !u.runtime.Capabilities(id).Dialogs {
		return werrors.Unsupported("el agente no soporta diálogos interactivos")
	}
	if err := u.runtime.Send(ctx, id, models.AnswerDialogCommand{DialogID: dialogID, Answer: answer}); err != nil {
		return err
	}
	if err := u.lifecycle.Record(ctx, id, models.DialogResolved{DialogID: dialogID}); err != nil {
		return fmt.Errorf("%w: %w", werrors.AnswerNotRecorded(), err)
	}
	return nil
}

// GetTaskSubagents lists the subagents of a task (last known state); NotFound when the task does not exist.
type GetTaskSubagents struct {
	lifecycle *TaskLifecycle
	runtime   outbound.AgentRuntime
}

// NewGetTaskSubagents builds the use case.
func NewGetTaskSubagents(lifecycle *TaskLifecycle, runtime outbound.AgentRuntime) *GetTaskSubagents {
	return &GetTaskSubagents{lifecycle: lifecycle, runtime: runtime}
}

// Execute returns the subagents the runtime knows for the task.
func (u *GetTaskSubagents) Execute(ctx context.Context, id models.TaskID) ([]models.SubagentTask, error) {
	if _, err := u.lifecycle.Load(ctx, id); err != nil {
		return nil, err
	}
	return u.runtime.Subagents(ctx, id), nil
}
