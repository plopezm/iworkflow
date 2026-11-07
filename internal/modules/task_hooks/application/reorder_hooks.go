package application

import (
	"context"
	"fmt"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// ReorderHooks sets the position of every hook of a workspace from a new order. The order must name each hook of
// the workspace exactly once: a partial list would leave the positions of the rest ambiguous.
type ReorderHooks struct {
	hooks outbound.HookRepository
}

// NewReorderHooks builds the use case.
func NewReorderHooks(hooks outbound.HookRepository) *ReorderHooks {
	return &ReorderHooks{hooks: hooks}
}

// Execute stores the order. A list that is incomplete, repeats an id or names a hook of another workspace is
// ErrInvalidHook, and nothing changes.
func (u *ReorderHooks) Execute(ctx context.Context, workspaceID string, ids []string) error {
	current, err := u.hooks.List(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("list hooks: %w", err)
	}
	if len(ids) != len(current) {
		return fmt.Errorf("%w: the order must name the %d hooks of the workspace", hookerrors.ErrInvalidHook, len(current))
	}
	known := make(map[string]bool, len(current))
	for _, hook := range current {
		known[hook.ID()] = true
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !known[id] {
			return fmt.Errorf("%w: hook %s does not belong to the workspace", hookerrors.ErrInvalidHook, id)
		}
		if seen[id] {
			return fmt.Errorf("%w: hook %s is listed twice", hookerrors.ErrInvalidHook, id)
		}
		seen[id] = true
	}
	if err := u.hooks.Reorder(ctx, workspaceID, ids); err != nil {
		return fmt.Errorf("reorder hooks: %w", err)
	}
	return nil
}
