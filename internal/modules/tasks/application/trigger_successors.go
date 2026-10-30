//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

const (
	// baseBackoff is the delay after the first failed start of a successor.
	baseBackoff = 15 * time.Second
	// maxBackoff caps the delay between two attempts.
	maxBackoff = 10 * time.Minute
	// maxBackoffExponent keeps the shift of baseBackoff far from overflowing; the cap applies long before it.
	maxBackoffExponent = 16
	// blockedByDiscardedMessage is logged once for every task that a discarded predecessor blocks.
	blockedByDiscardedMessage = "task blocked: a predecessor was discarded; remove the dependency to unblock it"
)

// TriggerSuccessors starts the backlog tasks whose predecessors have met their trigger (merged, by default).
// It is idempotent: StartTask claims each task atomically, so concurrent evaluations do not start a task
// twice. A failed start does not stop the other successors; the failed one is retried with an exponential
// backoff, in memory, until it starts or stops being a candidate. A backlog task blocked by a discarded
// predecessor never starts by itself: it is reported once and the user must remove the edge.
type TriggerSuccessors struct {
	repository outbound.TaskRepository
	graph      outbound.TaskGraphRepository
	starter    inbound.StartTaskUseCase
	clock      Clock

	// mu guards both maps. It is never held while a task is started.
	mu            sync.Mutex
	backoff       map[models.TaskID]startBackoff
	warnedBlocked map[models.TaskID]struct{}
}

// startBackoff is the retry state of a successor whose start failed.
type startBackoff struct {
	attempts    uint32
	nextAllowed time.Time
}

// NewTriggerSuccessors builds the trigger. The clock is injected so that the backoff can be tested.
func NewTriggerSuccessors(repository outbound.TaskRepository, graph outbound.TaskGraphRepository, starter inbound.StartTaskUseCase, clock Clock) *TriggerSuccessors {
	return &TriggerSuccessors{
		repository:    repository,
		graph:         graph,
		starter:       starter,
		clock:         clock,
		backoff:       make(map[models.TaskID]startBackoff),
		warnedBlocked: make(map[models.TaskID]struct{}),
	}
}

// Trigger evaluates the graph once and returns how many tasks it started. It is the SuccessorTrigger port.
func (u *TriggerSuccessors) Trigger(ctx context.Context) (int, error) {
	now := u.clock()
	edges, err := u.graph.ListDependencies(ctx, nil)
	if err != nil {
		return 0, err
	}
	if len(edges) == 0 {
		return 0, nil
	}
	tasks, err := u.loadTasks(ctx, edges)
	if err != nil {
		return 0, err
	}
	u.warnBlocked(tasks, edges)
	startable := models.StartableTasks(tasks, edges)
	// A task that is no longer a candidate (started by hand, discarded...) forgets its backoff.
	u.forgetBackoffExcept(startable)

	started := 0
	for _, id := range startable {
		if u.isBackingOff(id, now) {
			continue
		}
		if _, err := u.starter.Start(ctx, id); err != nil {
			attempts := u.recordFailure(id, now)
			slog.Warn("could not start the task triggered by the graph", "task_id", id.String(), "attempts", attempts, "error", err)
			continue
		}
		started++
		u.clearBackoff(id)
	}
	return started, nil
}

// loadTasks reads every task that appears in an edge, once, in the order the edges list them. A task that no
// longer exists is skipped.
func (u *TriggerSuccessors) loadTasks(ctx context.Context, edges []models.TaskDependency) ([]models.Task, error) {
	seen := make(map[models.TaskID]struct{}, len(edges)*2)
	var ids []models.TaskID
	for _, edge := range edges {
		for _, id := range [2]models.TaskID{edge.Predecessor, edge.Successor} {
			if _, known := seen[id]; !known {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	tasks := make([]models.Task, 0, len(ids))
	for _, id := range ids {
		found, err := u.repository.Find(ctx, id)
		if err != nil {
			return nil, err
		}
		if found != nil {
			tasks = append(tasks, *found)
		}
	}
	return tasks, nil
}

// warnBlocked logs once for every task that a discarded predecessor blocks. A task that stops being blocked
// is forgotten, so that a new block warns again.
func (u *TriggerSuccessors) warnBlocked(tasks []models.Task, edges []models.TaskDependency) {
	blocked := models.BlockedByDiscarded(tasks, edges)
	stillBlocked := make(map[models.TaskID]struct{}, len(blocked))
	for _, id := range blocked {
		stillBlocked[id] = struct{}{}
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	for id := range u.warnedBlocked {
		if _, ok := stillBlocked[id]; !ok {
			delete(u.warnedBlocked, id)
		}
	}
	for _, id := range blocked {
		if _, warned := u.warnedBlocked[id]; warned {
			continue
		}
		u.warnedBlocked[id] = struct{}{}
		slog.Warn(blockedByDiscardedMessage, "task_id", id.String())
	}
}

// isBackingOff reports whether the successor failed recently and its delay has not elapsed yet.
func (u *TriggerSuccessors) isBackingOff(id models.TaskID, now time.Time) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	state, failed := u.backoff[id]
	return failed && now.Before(state.nextAllowed)
}

// recordFailure counts one more failed start and schedules the next attempt. It returns the new count.
func (u *TriggerSuccessors) recordFailure(id models.TaskID, now time.Time) uint32 {
	u.mu.Lock()
	defer u.mu.Unlock()
	attempts := uint32(1)
	if state, failed := u.backoff[id]; failed {
		attempts = state.attempts
		if attempts < math.MaxUint32 {
			attempts++
		}
	}
	u.backoff[id] = startBackoff{attempts: attempts, nextAllowed: now.Add(backoffDelay(attempts))}
	return attempts
}

// clearBackoff forgets the failures of a successor that started.
func (u *TriggerSuccessors) clearBackoff(id models.TaskID) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.backoff, id)
}

// forgetBackoffExcept drops the backoff of every task that is not in keep.
func (u *TriggerSuccessors) forgetBackoffExcept(keep []models.TaskID) {
	kept := make(map[models.TaskID]struct{}, len(keep))
	for _, id := range keep {
		kept[id] = struct{}{}
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	for id := range u.backoff {
		if _, ok := kept[id]; !ok {
			delete(u.backoff, id)
		}
	}
}

// backoffDelay is the wait after the given number of consecutive failures: 15 s, 30 s, 60 s... capped at 10 min.
func backoffDelay(attempts uint32) time.Duration {
	exponent := uint32(0)
	if attempts > 0 {
		exponent = min(attempts-1, maxBackoffExponent)
	}
	return min(baseBackoff<<exponent, maxBackoff)
}
