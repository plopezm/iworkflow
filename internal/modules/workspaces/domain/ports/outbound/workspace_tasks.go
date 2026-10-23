package outbound

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
)

// WorkspaceTasks deletes the tasks of a workspace before the workspace itself. It is implemented by
// the tasks module and wired by the composition root: this module never imports it.
type WorkspaceTasks interface {
	// DeleteAll deletes every task of the workspace. It returns InUse when any of them is still active.
	DeleteAll(ctx context.Context, id models.WorkspaceID) error
}
