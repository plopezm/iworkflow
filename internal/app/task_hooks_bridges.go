package app

import (
	"context"
	"errors"
	"fmt"

	hooksapp "iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	hooksmodels "iworkflow/internal/modules/task_hooks/domain/models"
	taskserrors "iworkflow/internal/modules/tasks/domain/errors"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// hooksRunner is the part of the task_hooks RunHooks use case the lifecycle bridge needs.
type hooksRunner interface {
	Execute(ctx context.Context, in hooksapp.RunHooksInput) (hooksapp.RunHooksReport, error)
}

// lifecycleHooks gives the tasks start and finish the hooks of the task_hooks module. It lives in the composition
// root, so the tasks module never imports task_hooks: it maps the task action to the hooks of its event and phase,
// and a blocking failure to the tasks ErrHookBlocked.
type lifecycleHooks struct {
	run hooksRunner
}

var _ tasksoutbound.LifecycleHooks = (*lifecycleHooks)(nil)

func newLifecycleHooks(run hooksRunner) *lifecycleHooks {
	return &lifecycleHooks{run: run}
}

// Run executes the hooks of the request. A blocking failure is returned wrapping the tasks ErrHookBlocked and keeps
// the message of the hooks module. The warnings come back in the report even when the action is blocked.
func (l *lifecycleHooks) Run(ctx context.Context, req tasksmodels.HookRequest) (tasksmodels.HookReport, error) {
	input, err := hooksRunInput(req)
	if err != nil {
		return tasksmodels.HookReport{}, fmt.Errorf("map the lifecycle hook request: %w", err)
	}
	report, err := l.run.Execute(ctx, input)
	mapped := tasksmodels.HookReport{Warnings: report.Warnings}
	if err == nil {
		return mapped, nil
	}
	if errors.Is(err, hookerrors.ErrHookBlocked) {
		return mapped, fmt.Errorf("%w: %w", taskserrors.ErrHookBlocked, err)
	}
	return mapped, fmt.Errorf("run the hooks: %w", err)
}

// hooksRunInput maps the task action to the input of the hooks run. An unknown event, phase or result is an error,
// so a request never silently runs no hook.
func hooksRunInput(req tasksmodels.HookRequest) (hooksapp.RunHooksInput, error) {
	event, err := hookEvent(req.Event)
	if err != nil {
		return hooksapp.RunHooksInput{}, err
	}
	phase, err := hookPhase(req.Phase)
	if err != nil {
		return hooksapp.RunHooksInput{}, err
	}
	result, err := hookResult(req.Result)
	if err != nil {
		return hooksapp.RunHooksInput{}, err
	}
	return hooksapp.RunHooksInput{
		WorkspaceID: req.Task.WorkspaceID,
		Run: hooksmodels.RunContext{
			TaskID:        req.Task.ID.String(),
			TaskTitle:     req.Task.Title,
			TaskStatus:    string(req.Task.Status),
			WorkspacePath: req.WorkspacePath,
			WorktreePath:  req.Task.WorktreePath,
			Branch:        req.Task.Branch,
			Phase:         phase,
			Event:         event,
			Result:        result,
		},
	}, nil
}

func hookEvent(event tasksmodels.HookEvent) (hooksmodels.Event, error) {
	switch event {
	case tasksmodels.HookEventStart:
		return hooksmodels.EventTaskStart, nil
	case tasksmodels.HookEventFinish:
		return hooksmodels.EventTaskFinish, nil
	default:
		return "", fmt.Errorf("unknown hook event %q", event)
	}
}

func hookPhase(phase tasksmodels.HookPhase) (hooksmodels.Phase, error) {
	switch phase {
	case tasksmodels.HookPhasePre:
		return hooksmodels.PhasePre, nil
	case tasksmodels.HookPhasePost:
		return hooksmodels.PhasePost, nil
	default:
		return "", fmt.Errorf("unknown hook phase %q", phase)
	}
}

// hookResult maps the outcome of the task action. A pre hook has none, so the empty result is valid.
func hookResult(result tasksmodels.HookResult) (hooksmodels.TaskResult, error) {
	switch result {
	case "":
		return hooksmodels.TaskResultNone, nil
	case tasksmodels.HookResultOK:
		return hooksmodels.TaskResultOK, nil
	case tasksmodels.HookResultFailed:
		return hooksmodels.TaskResultFailed, nil
	case tasksmodels.HookResultAborted:
		return hooksmodels.TaskResultAborted, nil
	default:
		return "", fmt.Errorf("unknown hook result %q", result)
	}
}
