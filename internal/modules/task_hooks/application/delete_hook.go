package application

import (
	"context"
	"fmt"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// DeleteHook removes a hook of a workspace. Its run records and skips go with it, by the storage cascade.
type DeleteHook struct {
	hooks outbound.HookRepository
}

// NewDeleteHook builds the use case.
func NewDeleteHook(hooks outbound.HookRepository) *DeleteHook {
	return &DeleteHook{hooks: hooks}
}

// Execute deletes the hook when it belongs to the workspace. An unknown id is ErrHookNotFound; an id of another
// workspace is ErrInvalidHook and nothing is deleted.
func (u *DeleteHook) Execute(ctx context.Context, workspaceID string, id string) error {
	existing, err := u.hooks.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("load hook %s: %w", id, err)
	}
	if existing.WorkspaceID() != workspaceID {
		return fmt.Errorf("%w: hook %s belongs to another workspace", hookerrors.ErrInvalidHook, id)
	}
	if err := u.hooks.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete hook %s: %w", id, err)
	}
	return nil
}
