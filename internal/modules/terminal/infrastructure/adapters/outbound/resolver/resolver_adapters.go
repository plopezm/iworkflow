// Package resolver adapts the terminal path ports to lookups injected by the composition root.
//
// The terminal module cannot import the workspaces or tasks modules. The composition root (internal/app)
// builds these functions over the other modules' use cases, which is the only place where both meet.
package resolver

import (
	"context"

	"github.com/google/uuid"

	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
)

// WorkspaceResolver looks a workspace up. It returns (nil, nil) when the workspace does not exist.
type WorkspaceResolver func(ctx context.Context, workspaceID uuid.UUID) (*models.WorkspaceTarget, error)

// TaskResolver looks a task up. It returns (nil, nil) when the task does not exist or has no worktree.
type TaskResolver func(ctx context.Context, taskID uuid.UUID) (*models.TaskTarget, error)

// WorkspacePathAdapter implements outbound.WorkspacePathPort on top of a WorkspaceResolver.
type WorkspacePathAdapter struct {
	resolve WorkspaceResolver
}

// NewWorkspacePathAdapter builds the adapter.
func NewWorkspacePathAdapter(resolve WorkspaceResolver) *WorkspacePathAdapter {
	return &WorkspacePathAdapter{resolve: resolve}
}

// Resolve maps a missing workspace to ErrWorkspaceNotFound and a lookup failure to a pty error.
func (a *WorkspacePathAdapter) Resolve(ctx context.Context, workspaceID uuid.UUID) (models.WorkspaceTarget, error) {
	found, err := a.resolve(ctx, workspaceID)
	if err != nil {
		return models.WorkspaceTarget{}, terminalerrors.Pty(err) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	if found == nil {
		return models.WorkspaceTarget{}, terminalerrors.ErrWorkspaceNotFound
	}
	return *found, nil
}

// TaskPathAdapter implements outbound.TaskPathPort on top of a TaskResolver.
type TaskPathAdapter struct {
	resolve TaskResolver
}

// NewTaskPathAdapter builds the adapter.
func NewTaskPathAdapter(resolve TaskResolver) *TaskPathAdapter {
	return &TaskPathAdapter{resolve: resolve}
}

// Resolve maps a missing task to ErrTaskNotFound and a lookup failure to a pty error.
func (a *TaskPathAdapter) Resolve(ctx context.Context, taskID uuid.UUID) (models.TaskTarget, error) {
	found, err := a.resolve(ctx, taskID)
	if err != nil {
		return models.TaskTarget{}, terminalerrors.Pty(err) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	if found == nil {
		return models.TaskTarget{}, terminalerrors.ErrTaskNotFound
	}
	return *found, nil
}
