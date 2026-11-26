//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// WorkspaceUnfinishedTasks answers whether a workspace still has unfinished tasks. It is the tasks side of
// the rule that blocks a change of the workspace path. It only reads, so it never changes a task.
type WorkspaceUnfinishedTasks struct {
	repository outbound.TaskRepository
}

// NewWorkspaceUnfinishedTasks builds the use case.
func NewWorkspaceUnfinishedTasks(repository outbound.TaskRepository) *WorkspaceUnfinishedTasks {
	return &WorkspaceUnfinishedTasks{repository: repository}
}

// HasUnfinished reports whether the workspace has at least one task that is neither merged nor discarded.
// Backlog tasks count: they may already hold a branch or a worktree.
func (u *WorkspaceUnfinishedTasks) HasUnfinished(ctx context.Context, workspaceID string) (bool, error) {
	tasks, err := u.repository.ListActive(ctx, &workspaceID)
	if err != nil {
		return false, err
	}
	return len(tasks) > 0, nil
}
