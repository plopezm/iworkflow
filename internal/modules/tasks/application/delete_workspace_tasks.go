//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// DeleteWorkspaceTasks is the tasks side of deleting a workspace. It deletes every task of the workspace
// in one transaction, but only when all of them are inactive (backlog, merged or discarded): otherwise
// nothing is deleted and WorkspaceHasActiveTasks is returned.
//
// Slice A covers the rows, the attachment folders and the removal notices. Removing task worktrees and
// branches is slice E and is not part of this use case yet.
type DeleteWorkspaceTasks struct {
	lifecycle *TaskLifecycle
	storage   outbound.AttachmentStorage
}

// NewDeleteWorkspaceTasks builds the use case.
func NewDeleteWorkspaceTasks(lifecycle *TaskLifecycle, storage outbound.AttachmentStorage) *DeleteWorkspaceTasks {
	return &DeleteWorkspaceTasks{lifecycle: lifecycle, storage: storage}
}

// DeleteAll deletes the workspace tasks. The workspace id is a plain string: the tasks module does not
// import the workspaces module.
func (u *DeleteWorkspaceTasks) DeleteAll(ctx context.Context, workspaceID string) error {
	ids, deleted, err := u.lifecycle.Repository().DeleteInactiveByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}
	if !deleted {
		return werrors.WorkspaceHasActiveTasks()
	}
	for _, id := range ids {
		if err := u.storage.DeleteTaskDir(ctx, id); err != nil {
			slog.Warn("could not delete the attachments of a deleted task", "task_id", id.String(), "error", err)
		}
		if err := u.lifecycle.NotifyRemoved(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
