//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// WorktreeRemover removes the worktree and the branch of a task. It is the removal part of the git port;
// the adapter satisfies it.
type WorktreeRemover interface {
	Remove(ctx context.Context, repo string, path string, branch string) error
}

// DeleteWorkspaceTasks is the tasks side of deleting a workspace. It works in this order: refuse when a
// task is active, remove the worktrees and branches of the closed tasks (best effort), delete the rows in
// one transaction, and delete the attachment folders. Nothing is removed when a task is active.
type DeleteWorkspaceTasks struct {
	lifecycle *TaskLifecycle
	worktrees WorktreeRemover
	storage   outbound.AttachmentStorage
}

// NewDeleteWorkspaceTasks builds the use case.
func NewDeleteWorkspaceTasks(lifecycle *TaskLifecycle, worktrees WorktreeRemover, storage outbound.AttachmentStorage) *DeleteWorkspaceTasks {
	return &DeleteWorkspaceTasks{lifecycle: lifecycle, worktrees: worktrees, storage: storage}
}

// DeleteAll deletes the workspace tasks. The workspace id is a plain string: the tasks module does not
// import the workspaces module.
func (u *DeleteWorkspaceTasks) DeleteAll(ctx context.Context, workspaceID string) error {
	tasks, err := u.lifecycle.Repository().List(ctx, &workspaceID)
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if !task.Status.IsInactive() {
			return werrors.WorkspaceHasActiveTasks()
		}
	}
	u.cleanupClosed(ctx, workspaceID, tasks)
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

// cleanupClosed removes the worktree and the branch of each closed task that has a branch. A failure never
// blocks the deletion. The workspace path comes from the workspace itself, never from the task.
func (u *DeleteWorkspaceTasks) cleanupClosed(ctx context.Context, workspaceID string, tasks []models.Task) {
	closed := make([]models.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.Status.IsClosed() && task.Branch != "" {
			closed = append(closed, task)
		}
	}
	if len(closed) == 0 {
		return
	}
	workspace, err := u.lifecycle.Workspace(ctx, workspaceID)
	if err != nil {
		slog.Warn("could not resolve the workspace; worktree cleanup is skipped", "workspace_id", workspaceID, "error", err)
		return
	}
	for _, task := range closed {
		if err := u.worktrees.Remove(ctx, workspace.Path, task.WorktreePath, task.Branch); err != nil {
			slog.Warn("could not clean up the task worktree", "task_id", task.ID.String(), "error", err)
		}
	}
}
