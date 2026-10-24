package outbound

import (
	"context"

	"github.com/google/uuid"

	"iworkflow/internal/modules/terminal/domain/models"
)

// WorkspacePathPort resolves the name and directory of a workspace. It returns
// terminalerrors.ErrWorkspaceNotFound when the workspace does not exist.
type WorkspacePathPort interface {
	Resolve(ctx context.Context, workspaceID uuid.UUID) (models.WorkspaceTarget, error)
}

// TaskPathPort resolves the workspace, title and worktree of a task. It returns
// terminalerrors.ErrTaskNotFound when the task does not exist or has no worktree.
type TaskPathPort interface {
	Resolve(ctx context.Context, taskID uuid.UUID) (models.TaskTarget, error)
}
