//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"errors"
	"fmt"
	"sync"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
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
	// hooks are the optional lifecycle hooks around the finish. Nil runs none.
	hooks outbound.LifecycleHooks

	// mu guards inFlight. It is never held while the hooks run or the agent is stopped.
	mu       sync.Mutex
	inFlight map[models.TaskID]struct{}
}

// NewFinishTask builds the use case on its ports.
func NewFinishTask(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime, cleanup *AttachmentCleanup) *FinishTask {
	return &FinishTask{
		lifecycle: lifecycle,
		worktrees: worktrees,
		runtime:   runtime,
		cleanup:   cleanup,
		inFlight:  make(map[models.TaskID]struct{}),
	}
}

// SetLifecycleHooks sets the hooks run before and after the finish. Nil, the default, runs none.
func (u *FinishTask) SetLifecycleHooks(hooks outbound.LifecycleHooks) {
	u.hooks = hooks
}

// Execute finishes the task. It fails, changing nothing, when the task is missing, its status does not allow
// finishing, or a pre hook blocks it (the error wraps errors.ErrHookBlocked). A consolidation failure is returned
// after the task is marked failed. The finish is the task_finish action of v1: it has no merge, which is the
// separate MergeTask use case.
//
// Two finishes of one task never overlap: the second one is refused while the first is in flight, so the pre hooks
// run once. The claim is only in memory, which is enough for the single server process. A finish that ends in a
// blocked pre hook changes nothing, so it releases the claim with the status as it was.
func (u *FinishTask) Execute(ctx context.Context, id models.TaskID) error {
	if !u.begin(id) {
		task, err := u.lifecycle.Load(ctx, id)
		if err != nil {
			return err
		}
		return invalidState("finalizar", task.Status)
	}
	defer u.end(id)

	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.CanFinish() {
		return invalidState("finalizar", task.Status)
	}
	workspacePath, err := u.hookWorkspacePath(ctx, task)
	if err != nil {
		return err
	}
	// The pre hooks run before anything changes, so a block leaves the task as it was, and before the commit, so
	// the edits the hooks make are committed too.
	preReport, preErr := runLifecycleHooks(ctx, u.hooks, models.HookRequest{
		Event:         models.HookEventFinish,
		Phase:         models.HookPhasePre,
		Task:          task,
		WorkspacePath: workspacePath,
	})
	logHookWarnings(id, preReport)
	if preErr != nil {
		if !errors.Is(preErr, werrors.ErrHookBlocked) {
			preErr = fmt.Errorf("%w: %w", werrors.ErrHookBlocked, preErr)
		}
		return preErr
	}
	// From here the finish is detached from the request: a closed tab or an aborted htmx call must not leave the task
	// in finishing, because the status writes of a cancelled context would fail and nothing would ever leave it.
	detached := context.WithoutCancel(ctx)
	if err := u.lifecycle.Transition(detached, id, models.StatusFinishing, nil); err != nil {
		return err
	}
	if err := u.consolidate(detached, task); err != nil {
		message := err.Error()
		// Intentional improvement over Rust: when marking the task failed fails too, both errors are returned, so the
		// consolidation error is not masked. errors.Join keeps errors.Is and errors.As working on each of them.
		if transitionErr := u.lifecycle.Transition(detached, id, models.StatusFailed, &message); transitionErr != nil {
			return errors.Join(err, transitionErr)
		}
		return err
	}
	if err := u.lifecycle.Transition(detached, id, models.StatusAwaitingReview, nil); err != nil {
		return err
	}
	// The post hooks run once the task awaits review; their failure does not fail the finish.
	task.Status = models.StatusAwaitingReview
	runPostLifecycleHooks(ctx, u.hooks, models.HookEventFinish, task, workspacePath)
	u.cleanup.CleanupTask(detached, id)
	return nil
}

func (u *FinishTask) begin(id models.TaskID) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, busy := u.inFlight[id]; busy {
		return false
	}
	u.inFlight[id] = struct{}{}
	return true
}

func (u *FinishTask) end(id models.TaskID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.inFlight, id)
}

// hookWorkspacePath returns the folder of the task's workspace for the hooks. Without hooks it returns "" and makes
// no lookup, so the finish is unchanged when no hooks are set.
func (u *FinishTask) hookWorkspacePath(ctx context.Context, task models.Task) (string, error) {
	if u.hooks == nil {
		return "", nil
	}
	workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
	if err != nil {
		return "", err
	}
	return workspace.Path, nil
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
