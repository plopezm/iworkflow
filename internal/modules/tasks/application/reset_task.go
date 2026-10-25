//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// ResetTask sends a task back to backlog: it stops the agent when it is alive, removes its worktree and branch,
// deletes its history (events) and clears the rest of its execution state. The status event is recorded after the
// history is deleted, so the new history starts with it.
type ResetTask struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
	runtime   outbound.AgentRuntime
}

// NewResetTask builds the use case on its ports.
func NewResetTask(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime) *ResetTask {
	return &ResetTask{lifecycle: lifecycle, worktrees: worktrees, runtime: runtime}
}

// Execute resets the task. It fails, changing nothing, when the task is missing or its status does not allow a reset.
// When the worktree cannot be removed, the history is left untouched.
func (u *ResetTask) Execute(ctx context.Context, id models.TaskID) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.CanReset() {
		return invalidState("restablecer", task.Status)
	}
	if u.runtime.IsRunning(ctx, id) {
		if err := u.runtime.Stop(ctx, id); err != nil {
			return err
		}
	}
	workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
	if err != nil {
		return err
	}
	// Remove tolerates a worktree or a branch that no longer exist (for example after a discard).
	if err := u.worktrees.Remove(ctx, workspace.Path, task.WorktreePath, task.Branch); err != nil {
		return err
	}
	reset, err := u.lifecycle.Repository().ResetToBacklog(ctx, id)
	if err != nil {
		return err
	}
	if !reset {
		return u.lifecycle.Conflict(ctx, id, "restablecer")
	}
	// The events are already deleted: the status event is recorded after so open clients refresh and the new
	// history starts with it.
	return u.lifecycle.NotifyStatus(ctx, id, models.StatusBacklog, nil)
}
