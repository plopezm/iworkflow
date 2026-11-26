//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"errors"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// DiscardTask stops the agent when it is alive and removes the worktree and the branch without integrating
// anything. The task ends in discarded.
type DiscardTask struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
	runtime   outbound.AgentRuntime
	listener  outbound.DiscardListener
}

// NewDiscardTask builds the use case on its ports.
func NewDiscardTask(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime) *DiscardTask {
	return &DiscardTask{lifecycle: lifecycle, worktrees: worktrees, runtime: runtime}
}

// SetDiscardListener sets who is told when a task ends discarded. Optional: without one nothing is told.
func (u *DiscardTask) SetDiscardListener(listener outbound.DiscardListener) {
	u.listener = listener
}

// Execute discards the task. It fails, changing nothing, when the task is missing or its status does not allow
// discarding. When the worktree cannot be removed, the error is returned: if this discard stopped a live agent the
// task is interrupted with the error text (the agent is gone, so its live status would be false); otherwise the status
// is left as it was.
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
	agentStopped := false
	if u.runtime.IsRunning(ctx, id) {
		if err := u.runtime.Stop(ctx, id); err != nil {
			return err
		}
		agentStopped = true
	}
	if err := u.worktrees.Remove(ctx, workspace.Path, task.WorktreePath, task.Branch); err != nil {
		if agentStopped {
			// The request may be cancelled; the agent is already gone, so the interrupted status must be saved anyway.
			detached := context.WithoutCancel(ctx)
			message := err.Error()
			if transitionErr := u.lifecycle.Transition(detached, id, models.StatusInterrupted, &message); transitionErr != nil {
				return errors.Join(err, transitionErr)
			}
		}
		return err
	}
	// The worktree and the branch are already gone: the request may be cancelled now, but the task must still be recorded
	// as discarded, or it would keep its old status with nothing left to discard.
	detached := context.WithoutCancel(ctx)
	if err := u.lifecycle.Transition(detached, id, models.StatusDiscarded, nil); err != nil {
		return err
	}
	if u.listener != nil {
		u.listener.Discarded(detached, id)
	}
	return nil
}
