package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// LifecycleHooks runs the hooks around the start and the finish of a task. It is optional and consumer-side:
// the composition root implements it with the task_hooks module, so tasks never imports task_hooks. When it is
// not set, the use cases run no hooks.
type LifecycleHooks interface {
	// Run executes the hooks of the request's event and phase. An error means the action must not proceed:
	// a blocking hook failed, and the error wraps errors.ErrHookBlocked. Warnings come back in the report.
	Run(ctx context.Context, req models.HookRequest) (models.HookReport, error)
}
