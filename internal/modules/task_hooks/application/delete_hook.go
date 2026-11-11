package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// DeleteHook removes a hook of the library with its assignments. Its run records stay in the history.
type DeleteHook struct {
	hooks outbound.HookRepository
}

// NewDeleteHook builds the use case.
func NewDeleteHook(hooks outbound.HookRepository) *DeleteHook {
	return &DeleteHook{hooks: hooks}
}

// Execute deletes the hook. An unknown id is ErrHookNotFound.
func (u *DeleteHook) Execute(ctx context.Context, id string) error {
	if err := u.hooks.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete hook %s: %w", id, err)
	}
	return nil
}
