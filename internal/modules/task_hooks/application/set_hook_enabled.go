package application

import (
	"context"
	"fmt"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// SetHookEnabledInput names the hook and the flag it gets.
type SetHookEnabledInput struct {
	WorkspaceID string
	HookID      string
	Enabled     bool
}

// SetHookEnabled turns a hook of a workspace on or off. Only the flag changes: the hook is rebuilt from its own
// fields and validated again, and the position is kept by the storage.
type SetHookEnabled struct {
	hooks outbound.HookRepository
}

// NewSetHookEnabled builds the use case.
func NewSetHookEnabled(hooks outbound.HookRepository) *SetHookEnabled {
	return &SetHookEnabled{hooks: hooks}
}

// Execute stores the flag. An unknown hook is ErrHookNotFound; a hook of another workspace is ErrInvalidHook, and
// nothing changes.
func (u *SetHookEnabled) Execute(ctx context.Context, in SetHookEnabledInput) error {
	existing, err := u.hooks.Get(ctx, in.HookID)
	if err != nil {
		return fmt.Errorf("load hook %s: %w", in.HookID, err)
	}
	if existing.WorkspaceID() != in.WorkspaceID {
		return fmt.Errorf("%w: hook %s belongs to another workspace", hookerrors.ErrInvalidHook, in.HookID)
	}
	params := existing.Params()
	params.Enabled = in.Enabled
	hook, err := models.NewHook(params)
	if err != nil {
		return fmt.Errorf("build hook: %w", err)
	}
	if err := u.hooks.Save(ctx, hook); err != nil {
		return fmt.Errorf("save hook %s: %w", in.HookID, err)
	}
	return nil
}
