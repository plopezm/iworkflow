//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"errors"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// FinishTask closes the agent, consolidates the pending changes into a commit and leaves the task ready for
// manual review. The worktree and the branch are kept for the review. The attachments of the task are cleaned up
// afterwards, best effort. When the consolidation fails, the task ends in failed with the error text.
type FinishTask struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
	runtime   outbound.AgentRuntime
	cleanup   *AttachmentCleanup
}

// NewFinishTask builds the use case on its ports.
func NewFinishTask(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime, cleanup *AttachmentCleanup) *FinishTask {
	return &FinishTask{lifecycle: lifecycle, worktrees: worktrees, runtime: runtime, cleanup: cleanup}
}

// Execute finishes the task. It fails, changing nothing, when the task is missing or its status does not allow
// finishing. A consolidation failure is returned after the task is marked failed.
func (u *FinishTask) Execute(ctx context.Context, id models.TaskID) error {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.CanFinish() {
		return invalidState("finalizar", task.Status)
	}
	if err := u.lifecycle.Transition(ctx, id, models.StatusFinishing, nil); err != nil {
		return err
	}
	if err := u.consolidate(ctx, task); err != nil {
		message := err.Error()
		// Intentional improvement over Rust: when marking the task failed fails too, both errors are returned, so the
		// consolidation error is not masked. errors.Join keeps errors.Is and errors.As working on each of them.
		if transitionErr := u.lifecycle.Transition(ctx, id, models.StatusFailed, &message); transitionErr != nil {
			return errors.Join(err, transitionErr)
		}
		return err
	}
	if err := u.lifecycle.Transition(ctx, id, models.StatusAwaitingReview, nil); err != nil {
		return err
	}
	u.cleanup.CleanupTask(ctx, id)
	return nil
}

// consolidate stops the agent when it is still alive and commits every pending change of the worktree.
func (u *FinishTask) consolidate(ctx context.Context, task models.Task) error {
	if u.runtime.IsRunning(ctx, task.ID) {
		if err := u.runtime.Stop(ctx, task.ID); err != nil {
			return err
		}
	}
	message := "pi: " + task.Title
	_, err := u.worktrees.CommitAll(ctx, task.WorktreePath, message)
	return err
}
