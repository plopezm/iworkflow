//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"time"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// TaskLifecycle is the shared operations of the use cases: loading tasks, changing their status and
// recording events (persisting the relevant ones and publishing them live).
type TaskLifecycle struct {
	repository outbound.TaskRepository
	publisher  outbound.TaskEventPublisher
	workspaces outbound.WorkspaceLookup
	clock      Clock
}

// NewTaskLifecycle builds the lifecycle on its ports.
func NewTaskLifecycle(repository outbound.TaskRepository, publisher outbound.TaskEventPublisher, workspaces outbound.WorkspaceLookup, clock Clock) *TaskLifecycle {
	return &TaskLifecycle{repository: repository, publisher: publisher, workspaces: workspaces, clock: clock}
}

// Repository gives the use cases the repository for their conditional writes.
func (l *TaskLifecycle) Repository() outbound.TaskRepository {
	return l.repository
}

// Load returns the task, or NotFound when it does not exist.
func (l *TaskLifecycle) Load(ctx context.Context, id models.TaskID) (models.Task, error) {
	found, err := l.repository.Find(ctx, id)
	if err != nil {
		return models.Task{}, err
	}
	if found == nil {
		return models.Task{}, werrors.NotFound(id.String())
	}
	return *found, nil
}

// Conflict is the error of a conditional operation that did not apply (the repository returned false):
// it reloads the task to tell "it no longer exists" from "its status changed".
func (l *TaskLifecycle) Conflict(ctx context.Context, id models.TaskID, operation string) error {
	task, err := l.Load(ctx, id)
	if err != nil {
		return err
	}
	return invalidState(operation, task.Status)
}

// invalidState is the error of an operation that is not allowed in the current status.
func invalidState(operation string, status models.TaskStatus) error {
	return werrors.InvalidState(operation, status.String())
}

// Workspace returns the workspace of a task, or WorkspaceNotFound.
func (l *TaskLifecycle) Workspace(ctx context.Context, workspaceID string) (models.WorkspaceRef, error) {
	found, err := l.workspaces.Find(ctx, workspaceID)
	if err != nil {
		return models.WorkspaceRef{}, err
	}
	if found == nil {
		return models.WorkspaceRef{}, werrors.WorkspaceNotFound(workspaceID)
	}
	return *found, nil
}

// Transition applies a status and records it as an event. Leaving the live statuses drops the dialogs
// that were pending.
func (l *TaskLifecycle) Transition(ctx context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error {
	if err := l.repository.UpdateStatus(ctx, id, status, errorText); err != nil {
		return err
	}
	if !status.HasLiveAgent() {
		if err := l.repository.ClearPendingDialogs(ctx, id); err != nil {
			return err
		}
	}
	return l.emit(ctx, id, models.StatusKind(status, errorText))
}

// NotifyStatus records and publishes a status change the caller already applied through a conditional
// update (for example ClaimForStart or RevertStartClaim). Unlike Transition, it does not write the status again.
func (l *TaskLifecycle) NotifyStatus(ctx context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error {
	return l.emit(ctx, id, models.StatusKind(status, errorText))
}

// Record records an event of the agent. The pending dialogs and the subagent counters are updated
// before the event is persisted and published, so a client that refetches on the event already sees
// the new state.
func (l *TaskLifecycle) Record(ctx context.Context, id models.TaskID, event models.AgentEvent) error {
	if err := l.syncPendingDialogs(ctx, id, event); err != nil {
		return err
	}
	if changed, ok := event.(models.SubagentsChanged); ok {
		if err := l.repository.SetSubagentActivity(ctx, id, models.NewSubagentActivity(changed.Active, changed.Waiting)); err != nil {
			return err
		}
	}
	return l.emit(ctx, id, models.AgentEventKind{Event: event})
}

func (l *TaskLifecycle) syncPendingDialogs(ctx context.Context, id models.TaskID, event models.AgentEvent) error {
	switch typed := event.(type) {
	case models.DialogRequested:
		var expiresAt *time.Time
		if typed.Request.TimeoutMS != nil {
			deadline := l.clock().Add(time.Duration(*typed.Request.TimeoutMS) * time.Millisecond)
			expiresAt = &deadline
		}
		return l.repository.AddPendingDialog(ctx, id, models.NewPendingDialog(typed.Request.ID, expiresAt))
	case models.DialogResolved:
		return l.repository.RemovePendingDialog(ctx, id, typed.DialogID)
	default:
		return nil
	}
}

// NotifyChanged tells the board that a backlog task was edited (ephemeral, not persisted).
func (l *TaskLifecycle) NotifyChanged(ctx context.Context, id models.TaskID) error {
	return l.emit(ctx, id, models.ChangedEventKind{})
}

// NotifyRemoved tells the board that a backlog task was deleted (ephemeral, not persisted).
func (l *TaskLifecycle) NotifyRemoved(ctx context.Context, id models.TaskID) error {
	return l.emit(ctx, id, models.RemovedEventKind{})
}

func (l *TaskLifecycle) emit(ctx context.Context, id models.TaskID, kind models.TaskEventKind) error {
	var event models.TaskEvent
	if kind.IsPersistent() {
		stored, err := l.repository.AppendEvent(ctx, id, kind)
		if err != nil {
			return err
		}
		event = stored
	} else {
		event = models.EphemeralTaskEvent(id, kind, l.clock())
	}
	l.publisher.Publish(event)
	return nil
}
