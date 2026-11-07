package application

import (
	"context"
	"fmt"
	"strings"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// SetTaskHookSkipped disables a hook for one task, or enables it again. It is in addition to the hook-level
// enabled flag: a skipped hook is recorded as skipped and does not run for that task.
type SetTaskHookSkipped struct {
	skips outbound.SkipRepository
}

// NewSetTaskHookSkipped builds the use case.
func NewSetTaskHookSkipped(skips outbound.SkipRepository) *SetTaskHookSkipped {
	return &SetTaskHookSkipped{skips: skips}
}

// Execute sets the skip of the hook for the task. Setting it twice is the same as setting it once.
func (u *SetTaskHookSkipped) Execute(ctx context.Context, taskID string, hookID string, skipped bool) error {
	if strings.TrimSpace(taskID) == "" || strings.TrimSpace(hookID) == "" {
		return fmt.Errorf("%w: the task and the hook are required", hookerrors.ErrInvalidHook)
	}
	if err := u.skips.SetSkipped(ctx, taskID, hookID, skipped); err != nil {
		return fmt.Errorf("set skip of hook %s for task %s: %w", hookID, taskID, err)
	}
	return nil
}
