//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// DeleteTask physically deletes a task in backlog (its events and attachments cascade). Its image
// files are deleted best effort: a failure there does not block the deletion.
type DeleteTask struct {
	lifecycle *TaskLifecycle
	storage   outbound.AttachmentStorage
}

// NewDeleteTask builds the use case.
func NewDeleteTask(lifecycle *TaskLifecycle, storage outbound.AttachmentStorage) *DeleteTask {
	return &DeleteTask{lifecycle: lifecycle, storage: storage}
}

// Execute deletes the task when it is in backlog.
func (u *DeleteTask) Execute(ctx context.Context, id models.TaskID) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.CanDelete() {
		return invalidState("eliminar", task.Status)
	}
	deleted, err := u.lifecycle.Repository().DeleteIfBacklog(ctx, id)
	if err != nil {
		return err
	}
	if !deleted {
		return u.lifecycle.Conflict(ctx, id, "eliminar")
	}
	if err := u.storage.DeleteTaskDir(ctx, id); err != nil {
		slog.Warn("could not delete the attachments of the task", "task_id", id.String(), "error", err)
	}
	return u.lifecycle.NotifyRemoved(ctx, id)
}
