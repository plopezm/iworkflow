package application

import (
	"context"
	"fmt"
	"log/slog"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// runLifecycleHooks runs the hooks of one phase of a task action. A nil port means no hooks: it returns an empty
// report and no error, so the use case behaves as it did before the hooks existed.
func runLifecycleHooks(ctx context.Context, hooks outbound.LifecycleHooks, req models.HookRequest) (models.HookReport, error) {
	if hooks == nil {
		return models.HookReport{}, nil
	}
	report, err := hooks.Run(ctx, req)
	if err != nil {
		return report, fmt.Errorf("lifecycle hooks of %s (%s): %w", req.Event, req.Phase, err)
	}
	return report, nil
}

// runPostLifecycleHooks runs the post hooks of an action that already succeeded. Their failure never fails the
// action, which is done: the error and the warnings are only logged, because the hooks module records the outcome
// in the task log.
func runPostLifecycleHooks(ctx context.Context, hooks outbound.LifecycleHooks, event models.HookEvent, task models.Task, workspacePath string) {
	report, err := runLifecycleHooks(ctx, hooks, models.HookRequest{
		Event:         event,
		Phase:         models.HookPhasePost,
		Result:        models.HookResultOK,
		Task:          task,
		WorkspacePath: workspacePath,
	})
	logHookWarnings(task.ID, report)
	if err != nil {
		slog.Warn("a post lifecycle hook failed; the task action stays done", "task_id", task.ID.String(), "event", event, "error", err)
	}
}

// logHookWarnings logs the warnings of a hook run that did not block the action.
func logHookWarnings(id models.TaskID, report models.HookReport) {
	for _, warning := range report.Warnings {
		slog.Warn("lifecycle hook warning", "task_id", id.String(), "warning", warning)
	}
}
