package app

import (
	"context"
	"errors"
	"fmt"

	hookapp "iworkflow/internal/modules/task_hooks/application"
	hookmodels "iworkflow/internal/modules/task_hooks/domain/models"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// errTaskHooksUnbound answers a skip request before the task hooks are bound. It is a wiring fault, not a user error.
var errTaskHooksUnbound = errors.New("task hooks are not available")

// TaskHookLister lists the hooks of a workspace as one task sees them. *hookapp.ListTaskHookState satisfies it.
type TaskHookLister interface {
	Execute(ctx context.Context, workspaceID string, taskID string) ([]hookapp.TaskHookState, error)
}

// TaskHookSkipper sets the skip of an assignment of a workspace for one task. *hookapp.SetTaskHookSkipped satisfies it.
type TaskHookSkipper interface {
	Execute(ctx context.Context, workspaceID string, taskID string, assignmentID string, skipped bool) error
}

// TaskHooksStrip implements the hook strip of the actions tab of a task over the task_hooks use cases.
//
// The tasks module is built before the task_hooks module, because the hooks run through the agent runtime of tasks.
// The strip is therefore created first, handed to the tasks module, and bound to the hook use cases once they exist.
// Until then it shows no hooks and refuses skips.
type TaskHooksStrip struct {
	lister  TaskHookLister
	skipper TaskHookSkipper
}

var _ tasksoutbound.HookStrip = (*TaskHooksStrip)(nil)

// NewTaskHooksStrip builds an unbound strip. Bind it before the first request is served.
func NewTaskHooksStrip() *TaskHooksStrip {
	return &TaskHooksStrip{}
}

// Bind connects the strip to the hook use cases.
func (b *TaskHooksStrip) Bind(lister TaskHookLister, skipper TaskHookSkipper) {
	b.lister = lister
	b.skipper = skipper
}

// State returns every assignment of the workspace with its status for the task.
func (b *TaskHooksStrip) State(ctx context.Context, workspaceID string, taskID string) ([]tasksoutbound.StripHook, error) {
	if b.lister == nil {
		return nil, nil
	}
	states, err := b.lister.Execute(ctx, workspaceID, taskID)
	if err != nil {
		return nil, fmt.Errorf("list hooks of task %s: %w", taskID, err)
	}
	hooks := make([]tasksoutbound.StripHook, 0, len(states))
	for _, state := range states {
		hooks = append(hooks, stripHookFromState(state))
	}
	return hooks, nil
}

// SetSkipped disables an assignment of the workspace for one task, or enables it again. An assignment of another
// workspace is refused.
func (b *TaskHooksStrip) SetSkipped(ctx context.Context, workspaceID string, taskID string, assignmentID string, skipped bool) error {
	if b.skipper == nil {
		return errTaskHooksUnbound
	}
	if err := b.skipper.Execute(ctx, workspaceID, taskID, assignmentID, skipped); err != nil {
		return fmt.Errorf("set skip of assignment %s for task %s: %w", assignmentID, taskID, err)
	}
	return nil
}

// stripHookFromState maps one assignment of a task to the row the strip shows. An assignment without a run reports the
// status none.
func stripHookFromState(state hookapp.TaskHookState) tasksoutbound.StripHook {
	hook := state.Hook
	assignment := state.Assignment
	row := tasksoutbound.StripHook{
		AssignmentID: assignment.ID(),
		HookID:       hook.ID(),
		Kind:         string(hook.Kind()),
		Phase:        string(assignment.Phase()),
		Event:        string(assignment.Event()),
		Enabled:      assignment.Enabled(),
		Skipped:      state.Skipped,
		LastStatus:   tasksoutbound.StripStatusNone,
	}
	if hook.Kind() == hookmodels.KindAgent {
		row.Description = hook.Agent().Prompt
	} else {
		row.Description = hook.Script().Command
	}
	if state.LastRun == nil {
		return row
	}
	row.LastStatus = string(state.LastRun.Status)
	row.LastOutput = state.LastRun.Output
	row.LastExitCode = state.LastRun.ExitCode
	row.LastStartedAt = state.LastRun.StartedAt
	return row
}
