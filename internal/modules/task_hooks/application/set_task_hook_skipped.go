package application

import (
	"context"
	"fmt"
	"strings"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// SetTaskHookSkipped disables a hook for one task, or enables it again. It is in addition to the hook-level
// enabled flag: a skipped hook is recorded as skipped and does not run for that task. The hook must belong to the
// workspace of the task, so a skip cannot be stored for a hook the task would never run.
type SetTaskHookSkipped struct {
	hooks outbound.HookRepository
	skips outbound.SkipRepository
}

// NewSetTaskHookSkipped builds the use case.
func NewSetTaskHookSkipped(hooks outbound.HookRepository, skips outbound.SkipRepository) *SetTaskHookSkipped {
	return &SetTaskHookSkipped{hooks: hooks, skips: skips}
}

// Execute sets the skip of the hook for the task. Setting it twice is the same as setting it once. An unknown hook is
// ErrHookNotFound; a hook of another workspace is ErrInvalidHook, and nothing is stored in either case.
func (u *SetTaskHookSkipped) Execute(ctx context.Context, workspaceID string, taskID string, hookID string, skipped bool) error {
	if strings.TrimSpace(workspaceID) == "" || strings.TrimSpace(taskID) == "" || strings.TrimSpace(hookID) == "" {
		return fmt.Errorf("%w: the workspace, the task and the hook are required", hookerrors.ErrInvalidHook)
	}
	hook, err := u.hooks.Get(ctx, hookID)
	if err != nil {
		return fmt.Errorf("load hook %s: %w", hookID, err)
	}
	if hook.WorkspaceID() != workspaceID {
		return fmt.Errorf("%w: hook %s belongs to another workspace", hookerrors.ErrInvalidHook, hookID)
	}
	if err := u.skips.SetSkipped(ctx, taskID, hookID, skipped); err != nil {
		return fmt.Errorf("set skip of hook %s for task %s: %w", hookID, taskID, err)
	}
	return nil
}
