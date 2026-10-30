//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// reconcileMessage is the error text of a task interrupted by a restart. Rust wording, kept byte-for-byte.
const reconcileMessage = "el servidor se reinició mientras la tarea estaba activa"

// ReconcileTasks runs at startup: no agent survives a restart, so every task that was active is marked interrupted.
// Rate-limited tasks are not active; the resume loop owns them.
type ReconcileTasks struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
}

// NewReconcileTasks builds the use case on its ports.
func NewReconcileTasks(lifecycle *TaskLifecycle, worktrees outbound.Worktree) *ReconcileTasks {
	return &ReconcileTasks{lifecycle: lifecycle, worktrees: worktrees}
}

// Execute marks the active tasks as interrupted and returns how many there were.
func (u *ReconcileTasks) Execute(ctx context.Context) (int, error) {
	active, err := u.lifecycle.Repository().ListByStatus(ctx, []models.TaskStatus{
		models.StatusStarting,
		models.StatusRunning,
		models.StatusIdle,
		models.StatusFinishing,
	})
	if err != nil {
		return 0, err
	}
	for _, task := range active {
		message := u.messageFor(ctx, task)
		if err = u.lifecycle.Transition(ctx, task.ID, models.StatusInterrupted, &message); err != nil {
			return 0, err
		}
	}
	return len(active), nil
}

// messageFor adds to the restart message when the worktree is gone: the task can then only be discarded. A task
// without a worktree path has nothing to check.
func (u *ReconcileTasks) messageFor(ctx context.Context, task models.Task) string {
	if task.WorktreePath == "" || u.worktrees.Exists(ctx, task.WorktreePath) {
		return reconcileMessage
	}
	return fmt.Sprintf("%s; el worktree ya no existe en %s y la tarea solo se puede descartar", reconcileMessage, task.WorktreePath)
}
