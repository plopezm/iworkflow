//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"sync"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// ResumeTask restarts the agent of a stopped task in its existing worktree, continuing its previous session,
// and leaves the task idle. It is the manual resume and the step the rate-limit sweep runs for each due task.
//
// It does not go through the start claim: a resume starts from a stopped status, while the claim only moves
// backlog tasks (Rust parity). Two resumes of the same task never overlap: a second call while the first is
// still starting the agent is rejected (new behavior, the Rust version has no such guard).
type ResumeTask struct {
	lifecycle      *TaskLifecycle
	worktrees      outbound.Worktree
	runtime        outbound.AgentRuntime
	launchSettings outbound.LaunchSettings
	sink           inbound.AgentEventSink

	// mu guards inFlight. It is never held while the agent is started.
	mu       sync.Mutex
	inFlight map[models.TaskID]struct{}
}

// NewResumeTask builds the use case on its ports. The sink receives the events of the restarted agent.
func NewResumeTask(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime, launchSettings outbound.LaunchSettings, sink inbound.AgentEventSink) *ResumeTask {
	return &ResumeTask{
		lifecycle:      lifecycle,
		worktrees:      worktrees,
		runtime:        runtime,
		launchSettings: launchSettings,
		sink:           sink,
		inFlight:       make(map[models.TaskID]struct{}),
	}
}

// Execute resumes the task. It fails, changing nothing, when the task is missing, cannot resume in its status,
// its worktree is gone, the runtime cannot continue a session, or the agent does not start. A task whose resume
// fails stays stopped.
func (u *ResumeTask) Execute(ctx context.Context, id models.TaskID) error {
	if !u.begin(id) {
		task, err := u.lifecycle.Load(ctx, id)
		if err != nil {
			return err
		}
		return invalidState("reanudar", task.Status)
	}
	defer u.end(id)

	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return err
	}
	if !task.Status.CanResume() {
		return invalidState("reanudar", task.Status)
	}
	if !u.worktrees.Exists(ctx, task.WorktreePath) {
		return werrors.WorktreeMissing(task.WorktreePath)
	}
	prepared, err := u.launchSettings.Prepare(ctx, task.WorktreePath, task.Agent)
	if err != nil {
		return err
	}
	capabilities := u.runtime.RuntimeCapabilities(prepared.Runtime)
	if !capabilities.Resume {
		return werrors.Unsupported("el agente no permite reanudar la sesión anterior")
	}
	options := prepared.WithTaskLimits(task.Limits).RestrictedTo(capabilities)
	if err := u.runtime.Start(ctx, id, task.WorktreePath, true, options, u.sink); err != nil {
		return err
	}
	return u.lifecycle.Transition(ctx, id, models.StatusIdle, nil)
}

// begin marks the task as in flight; it reports false when another resume of the task already holds it.
func (u *ResumeTask) begin(id models.TaskID) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, busy := u.inFlight[id]; busy {
		return false
	}
	u.inFlight[id] = struct{}{}
	return true
}

// end releases the task so a later resume can run.
func (u *ResumeTask) end(id models.TaskID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.inFlight, id)
}
