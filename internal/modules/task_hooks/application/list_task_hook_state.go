package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// TaskHookState is one hook of a workspace as one task sees it: whether the task skips it and the latest run of
// the hook for the task. LastRun is nil when the hook has not run for the task.
type TaskHookState struct {
	Hook    models.Hook
	Skipped bool
	LastRun *models.HookRun
}

// ListTaskHookState builds the status strip of a task: every hook of its workspace in position order, with the
// skip flag and the latest run of each.
type ListTaskHookState struct {
	hooks outbound.HookRepository
	skips outbound.SkipRepository
	runs  outbound.RunRepository
}

// NewListTaskHookState builds the use case.
func NewListTaskHookState(hooks outbound.HookRepository, skips outbound.SkipRepository, runs outbound.RunRepository) *ListTaskHookState {
	return &ListTaskHookState{hooks: hooks, skips: skips, runs: runs}
}

// Execute returns the state of each hook of the workspace for the task.
func (u *ListTaskHookState) Execute(ctx context.Context, workspaceID string, taskID string) ([]TaskHookState, error) {
	hooks, err := u.hooks.List(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list hooks: %w", err)
	}
	skippedIDs, err := u.skips.SkippedHookIDs(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list skipped hooks: %w", err)
	}
	runs, err := u.runs.ListByTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}

	skipped := make(map[string]bool, len(skippedIDs))
	for _, id := range skippedIDs {
		skipped[id] = true
	}
	// The runs come in start order, so the last record of a hook is its latest run.
	latest := make(map[string]models.HookRun, len(runs))
	for _, run := range runs {
		latest[run.HookID] = run
	}

	states := make([]TaskHookState, 0, len(hooks))
	for _, hook := range hooks {
		state := TaskHookState{Hook: hook, Skipped: skipped[hook.ID()]}
		if run, ok := latest[hook.ID()]; ok {
			state.LastRun = &run
		}
		states = append(states, state)
	}
	return states, nil
}
