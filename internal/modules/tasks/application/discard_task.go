//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// DiscardTask stops the agent when it is alive and removes the worktree and the branch without integrating
// anything. The task ends in discarded.
type DiscardTask struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
	runtime   outbound.AgentRuntime
}

// NewDiscardTask builds the use case on its ports.
func NewDiscardTask(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime) *DiscardTask {
	return &DiscardTask{lifecycle: lifecycle, worktrees: worktrees, runtime: runtime}
}

// Execute discards the task. It fails, changing nothing, when the task is missing or its status does not allow
// discarding. When the worktree cannot be removed, the status is left as it was.
func (u *DiscardTask) Execute(ctx context.Context, id models.TaskID) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.CanDiscard() {
		return invalidState("descartar", task.Status)
	}
	// Intentional improvement over Rust: the workspace is resolved before the agent is stopped, so a failed lookup
	// leaves the agent running and the task untouched.
	workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
	if err != nil {
		return err
	}
	if u.runtime.IsRunning(ctx, id) {
		if err := u.runtime.Stop(ctx, id); err != nil {
			return err
		}
	}
	if err := u.worktrees.Remove(ctx, workspace.Path, task.WorktreePath, task.Branch); err != nil {
		return err
	}
	return u.lifecycle.Transition(ctx, id, models.StatusDiscarded, nil)
}
