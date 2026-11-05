//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// errRestarterClosed is the refusal of a notice that arrives after the restarter started to close.
var errRestarterClosed = errors.New("the related projects restarter is closed")

// RestartOutcome says what a notice about related projects did to the agent of its task. Restarted is true when the
// agent was restarted with resume and told the notice at once. Pending is true when the notice waits for the agent to
// become idle. Both false means nothing will happen: the agent was left as it is, or it has no live session.
type RestartOutcome struct {
	Restarted bool
	Pending   bool
}

// EventFlusher waits until every agent event reported so far has been processed. The event queue implements it.
type EventFlusher interface {
	Flush()
}

// RestartGuard marks the agents the restarter stops on purpose. The exit of such an agent is not a crash, so the event
// use case drops it. It is safe for concurrent use.
type RestartGuard struct {
	mu     sync.Mutex
	active map[models.TaskID]struct{}
}

// NewRestartGuard builds a guard with no task marked.
func NewRestartGuard() *RestartGuard {
	return &RestartGuard{active: make(map[models.TaskID]struct{})}
}

// Mark records that the agent of the task is being stopped on purpose.
func (g *RestartGuard) Mark(id models.TaskID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active[id] = struct{}{}
}

// Unmark records that the agent of the task is no longer being stopped on purpose.
func (g *RestartGuard) Unmark(id models.TaskID) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.active, id)
}

// Active reports whether the agent of the task is being stopped on purpose. A nil guard is never active.
func (g *RestartGuard) Active(id models.TaskID) bool {
	if g == nil {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	_, marked := g.active[id]
	return marked
}

// restartResult is what one attempt to restart the agent of a task did with the notices it was given.
type restartResult int

const (
	// restartRestarted: the agent was restarted with resume and told the notices.
	restartRestarted restartResult = iota
	// restartAbandoned: the notices were dropped, or the task was failed. Nothing waits for an idle agent.
	restartAbandoned
	// restartWaiting: the agent is busy. The notices stay pending until it becomes idle.
	restartWaiting
)

// RelatedProjectsRestarter restarts the agent of a task after related projects were added to it, so that the folders
// of those projects reach its launch context, and then tells the agent about them in one prompt. A running agent cannot
// take new folders, so the restart waits until the agent is idle (see TaskIdle). Notices that arrive while the agent
// works are merged into one restart.
//
// The restart stops the agent, waits until its exit is processed, which the guard drops, and resumes the session with the
// folders it may read now. When the agent cannot be restarted, the task is marked failed with the error text. The save of
// the related projects is never undone by a restart.
//
// Each restart runs under a claim on its task, so two restarts of one task never overlap. The goroutines of the restarts
// are owned by this value: Close waits for them.
type RelatedProjectsRestarter struct {
	lifecycle      *TaskLifecycle
	worktrees      outbound.Worktree
	runtime        outbound.AgentRuntime
	launchSettings outbound.LaunchSettings
	sink           inbound.AgentEventSink
	related        *PrepareRelatedLaunch
	flusher        EventFlusher
	guard          *RestartGuard
	send           *SendMessage

	// wg counts the restarts under way, including the calls of Request, so that Close waits for all of them.
	wg sync.WaitGroup

	// mu guards closed, pending and running. It is never held while the agent is stopped or started.
	mu     sync.Mutex
	closed bool
	// pending holds, per task, the notices that wait for a restart, in arrival order.
	pending map[models.TaskID][]string
	// running holds the tasks whose restart is claimed.
	running map[models.TaskID]struct{}
}

// NewRelatedProjectsRestarter builds the restarter. The sink receives the events of the restarted agents, and flusher
// waits until the events of a stopped agent were processed, so that the guard sees its exit.
func NewRelatedProjectsRestarter(lifecycle *TaskLifecycle, worktrees outbound.Worktree, runtime outbound.AgentRuntime, launchSettings outbound.LaunchSettings, sink inbound.AgentEventSink, related *PrepareRelatedLaunch, flusher EventFlusher, guard *RestartGuard) *RelatedProjectsRestarter {
	return &RelatedProjectsRestarter{
		lifecycle:      lifecycle,
		worktrees:      worktrees,
		runtime:        runtime,
		launchSettings: launchSettings,
		sink:           sink,
		related:        related,
		flusher:        flusher,
		guard:          guard,
		send:           NewSendMessage(lifecycle, runtime),
		pending:        make(map[models.TaskID][]string),
		running:        make(map[models.TaskID]struct{}),
	}
}

// Request queues a notice about related projects for the task. An idle task is restarted at once and the call returns
// after the restart. A task that is starting or running keeps the notice until TaskIdle. A task without a live agent
// drops it. A failed read of the task keeps the notice for the next time the agent is idle.
func (r *RelatedProjectsRestarter) Request(ctx context.Context, id models.TaskID, notice string) (RestartOutcome, error) {
	if !r.beginWork() {
		return RestartOutcome{}, errRestarterClosed
	}
	defer r.wg.Done()

	// The notice is queued before the status is read: an idle transition that happens after the read finds it in TaskIdle.
	r.addPending(id, notice)
	task, err := r.lifecycle.Load(ctx, id)
	if err != nil {
		return RestartOutcome{}, err
	}
	if !task.Status.HasLiveAgent() {
		r.dropPending(id)
		return RestartOutcome{}, nil
	}
	if task.Status != models.StatusIdle || !r.claim(id) {
		return RestartOutcome{Pending: true}, nil
	}
	switch r.drain(context.WithoutCancel(ctx), id) {
	case restartRestarted:
		return RestartOutcome{Restarted: true}, nil
	case restartWaiting:
		return RestartOutcome{Pending: true}, nil
	default:
		return RestartOutcome{}, nil
	}
}

// TaskIdle is the hook the event use case calls when a task becomes idle. When notices wait for the task and no restart
// of it is under way, it starts the restart in a goroutine this value owns. It never blocks, because the event worker
// calls it.
func (r *RelatedProjectsRestarter) TaskIdle(id models.TaskID) {
	if !r.beginWork() {
		return
	}
	r.mu.Lock()
	_, busy := r.running[id]
	if busy || len(r.pending[id]) == 0 {
		r.mu.Unlock()
		r.wg.Done()
		return
	}
	r.running[id] = struct{}{}
	r.mu.Unlock()
	go func() {
		defer r.wg.Done()
		r.drain(context.Background(), id)
	}()
}

// Close refuses new notices and waits for the restarts under way. Call it on shutdown, before the agents are stopped. A
// restart is not interrupted: stopping it half-way would leave the agent and the task out of step.
func (r *RelatedProjectsRestarter) Close() {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	r.wg.Wait()
}

// beginWork admits one unit of work that Close waits for; it reports false once Close has started.
func (r *RelatedProjectsRestarter) beginWork() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return false
	}
	r.wg.Add(1)
	return true
}

// addPending queues a notice for the task.
func (r *RelatedProjectsRestarter) addPending(id models.TaskID, notice string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending[id] = append(r.pending[id], notice)
}

// dropPending forgets the notices of a task that has no live agent to receive them.
func (r *RelatedProjectsRestarter) dropPending(id models.TaskID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pending, id)
}

// claim takes the restart of the task; it reports false when a restart of the task is already under way.
func (r *RelatedProjectsRestarter) claim(id models.TaskID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, busy := r.running[id]; busy {
		return false
	}
	r.running[id] = struct{}{}
	return true
}

// nextNotices takes the notices that wait for the task. When none wait, it releases the claim in the same step, so a
// notice queued after it is seen by TaskIdle or by the next Request.
func (r *RelatedProjectsRestarter) nextNotices(id models.TaskID) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	notices := r.pending[id]
	delete(r.pending, id)
	if len(notices) == 0 {
		delete(r.running, id)
	}
	return notices
}

// requeue puts back the notices of an attempt that found the agent busy, ahead of any that arrived since, and releases
// the claim.
func (r *RelatedProjectsRestarter) requeue(id models.TaskID, notices []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pending[id] = append(notices, r.pending[id]...)
	delete(r.running, id)
}

// drain restarts the task for the notices that wait for it, while the claim is held, and releases the claim when none
// wait. It returns what the first attempt did.
func (r *RelatedProjectsRestarter) drain(ctx context.Context, id models.TaskID) restartResult {
	first := restartAbandoned
	for attempt := 0; ; attempt++ {
		notices := r.nextNotices(id)
		if len(notices) == 0 {
			return first
		}
		result := r.restart(ctx, id, notices)
		if attempt == 0 {
			first = result
		}
		if result == restartWaiting {
			r.requeue(id, notices)
			return result
		}
	}
}

// restart stops the agent of the task, resumes it with the folders it may read now, and tells it the notices. The
// task must be idle: a busy agent keeps the notices, and a task without a live agent drops them.
func (r *RelatedProjectsRestarter) restart(ctx context.Context, id models.TaskID, notices []string) restartResult {
	task, err := r.lifecycle.Load(ctx, id)
	if err != nil {
		slog.Warn("could not reload the task to restart its agent; the related projects notice is dropped", "task_id", id.String(), "error", err)
		return restartAbandoned
	}
	if task.Status != models.StatusIdle {
		if task.Status.HasLiveAgent() {
			return restartWaiting
		}
		slog.Warn("the task has no live agent; the related projects notice is dropped", "task_id", id.String(), "status", task.Status.String())
		return restartAbandoned
	}
	options, err := prepareResume(ctx, r.worktrees, r.runtime, r.launchSettings, r.related, task)
	if err != nil {
		slog.Warn("the agent cannot resume with the related projects; it is left as it is", "task_id", id.String(), "error", err)
		return restartAbandoned
	}
	if r.runtime.IsRunning(ctx, id) {
		if err := r.stop(ctx, id); err != nil {
			return r.fail(ctx, id, err)
		}
	}
	if err := r.runtime.Start(ctx, id, task.WorktreePath, true, options, r.sink); err != nil {
		return r.fail(ctx, id, err)
	}
	if err := r.lifecycle.Transition(ctx, id, models.StatusIdle, nil); err != nil {
		return r.fail(ctx, id, err)
	}
	r.tell(ctx, id, notices)
	return restartRestarted
}

// stop stops the agent of the task and waits until the events the stop produced were processed. The guard is held over
// that wait, so the exit of the agent is dropped rather than recorded as a crash.
func (r *RelatedProjectsRestarter) stop(ctx context.Context, id models.TaskID) error {
	r.guard.Mark(id)
	err := r.runtime.Stop(ctx, id)
	r.flusher.Flush()
	r.guard.Unmark(id)
	return err
}

// fail marks the task failed with the text of the cause and drops the notices, because the agent cannot take them.
func (r *RelatedProjectsRestarter) fail(ctx context.Context, id models.TaskID, cause error) restartResult {
	text := cause.Error()
	slog.Warn("could not restart the agent for the related projects", "task_id", id.String(), "error", text)
	if err := r.lifecycle.Transition(ctx, id, models.StatusFailed, &text); err != nil {
		slog.Warn("could not mark the task failed after the agent restart failed", "task_id", id.String(), "error", err)
	}
	return restartAbandoned
}

// tell sends the notices to the restarted agent as one prompt. A failure is logged: the agent is already restarted.
func (r *RelatedProjectsRestarter) tell(ctx context.Context, id models.TaskID, notices []string) {
	message := relatedAddedIntro + strings.Join(notices, "\n\n")
	if err := r.send.Execute(ctx, id, message, nil, models.MessageModeFollowUp); err != nil {
		slog.Warn("could not tell the agent about the new related projects", "task_id", id.String(), "error", err)
	}
}
