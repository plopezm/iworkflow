//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible.
package application

import (
	"context"
	"log/slog"
	"sync"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// NewSession restarts the agent of a task with a clean context, like /new in pi: the live agent is stopped (when
// there is one) and started again in the same worktree without resume, so every runtime opens a fresh session. The
// initial prompt is not sent again. It only talks to the AgentRuntime port, so it works with any agent.
//
// The history (events) of the task is deleted, so the task starts over from the next prompt; the worktree and the
// branch are kept. A stopped task (interrupted, failed...) gets a fresh session instead
// of continuing the previous one. Two restarts of one task never overlap.
type NewSession struct {
	lifecycle      *TaskLifecycle
	worktrees      outbound.Worktree
	runtime        outbound.AgentRuntime
	launchSettings outbound.LaunchSettings
	sink           inbound.AgentEventSink
	related        *PrepareRelatedLaunch
	flusher        EventFlusher
	guard          *RestartGuard

	// mu guards inFlight. It is never held while the agent is stopped or started.
	mu       sync.Mutex
	inFlight map[models.TaskID]struct{}
}

// NewNewSession builds the use case on its ports. The guard keeps the exit of the stopped agent from being recorded as
// a crash, and the flusher waits until that exit was processed.
func NewNewSession(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime, launchSettings outbound.LaunchSettings, sink inbound.AgentEventSink, related *PrepareRelatedLaunch, flusher EventFlusher, guard *RestartGuard) *NewSession {
	return &NewSession{
		lifecycle:      lifecycle,
		worktrees:      worktrees,
		runtime:        runtime,
		launchSettings: launchSettings,
		sink:           sink,
		related:        related,
		flusher:        flusher,
		guard:          guard,
		inFlight:       make(map[models.TaskID]struct{}),
	}
}

// Execute restarts the agent with a clean context and leaves the task idle. It fails, changing nothing, when the task is
// missing, its status does not allow it, its worktree is gone, or another restart of the task is under way. When the
// agent was stopped and cannot start again, the task is marked failed.
func (u *NewSession) Execute(ctx context.Context, id models.TaskID) error {
	if !u.begin(id) {
		task, err := u.lifecycle.Load(ctx, id)
		if err != nil {
			return err
		}
		return invalidState("iniciar una sesión nueva", task.Status)
	}
	defer u.end(id)

	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.CanStartNewSession() {
		return invalidState("iniciar una sesión nueva", task.Status)
	}
	if !u.worktrees.Exists(ctx, task.WorktreePath) {
		return werrors.WorktreeMissing(task.WorktreePath)
	}
	prepared, err := u.launchSettings.Prepare(ctx, task.WorktreePath, task.Agent)
	if err != nil {
		return err
	}
	options := prepared.WithTaskLimits(task.Limits).RestrictedTo(u.runtime.RuntimeCapabilities(prepared.Runtime))
	options.AddDirs = u.related.Execute(ctx, id).AddDirs

	if u.runtime.IsRunning(ctx, id) {
		if err := u.stop(ctx, id); err != nil {
			return err
		}
	}
	if err := u.runtime.Start(ctx, id, task.WorktreePath, false, options, u.sink); err != nil {
		u.fail(ctx, id, err)
		return err
	}
	if err := u.clearHistory(ctx, id); err != nil {
		u.fail(ctx, id, err)
		return err
	}
	// The status event is recorded after the history is deleted, so the new history starts with it.
	return u.lifecycle.Transition(ctx, id, models.StatusIdle, nil)
}

// clearHistory deletes the events of the task and what belonged to the old session: its pending dialogs and its
// subagent counters.
func (u *NewSession) clearHistory(ctx context.Context, id models.TaskID) error {
	repository := u.lifecycle.Repository()
	if err := repository.DeleteEvents(ctx, id); err != nil {
		return err
	}
	if err := repository.ClearPendingDialogs(ctx, id); err != nil {
		return err
	}
	return repository.SetSubagentActivity(ctx, id, models.NewSubagentActivity(0, 0))
}

// stop stops the agent and waits until the events of the stop were processed, holding the guard so its exit is dropped.
func (u *NewSession) stop(ctx context.Context, id models.TaskID) error {
	u.guard.Mark(id)
	defer u.guard.Unmark(id)
	err := u.runtime.Stop(ctx, id)
	u.flusher.Flush()
	return err
}

// fail marks the task failed with the text of the cause, because its agent was stopped and could not start again.
func (u *NewSession) fail(ctx context.Context, id models.TaskID, cause error) {
	text := cause.Error()
	if err := u.lifecycle.Transition(ctx, id, models.StatusFailed, &text); err != nil {
		slog.Warn("could not mark the task failed after its new session failed to start", "task_id", id.String(), "error", err)
	}
}

func (u *NewSession) begin(id models.TaskID) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, busy := u.inFlight[id]; busy {
		return false
	}
	u.inFlight[id] = struct{}{}
	return true
}

func (u *NewSession) end(id models.TaskID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.inFlight, id)
}
