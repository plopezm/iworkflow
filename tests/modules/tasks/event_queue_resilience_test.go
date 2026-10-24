package tasks_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/di"
)

// eventCount is the number of agent events each resilience test pushes through the queue.
const eventCount = 50

// lockHold is longer than the connection busy timeout (5 s): the first writes really fail with a locked
// database, and the queue must keep accepting events meanwhile.
const lockHold = 6500 * time.Millisecond

// resilienceEvent is the i-th event of the scenario. Every fifth event opens a dialog; the dialog
// opened by event i is closed by event i+1 on every other round, so some dialogs stay pending.
func resilienceEvent(index int) models.AgentEvent {
	switch {
	case index%5 == 0:
		return models.DialogRequested{Request: models.DialogRequest{
			ID:    fmt.Sprintf("dialog-%d", index),
			Title: fmt.Sprintf("diálogo %d", index),
			Kind:  models.ConfirmDialog{},
		}}
	case index%5 == 1 && (index/5)%2 == 0:
		return models.DialogResolved{DialogID: fmt.Sprintf("dialog-%d", index-1)}
	default:
		return models.UserMessage{Text: fmt.Sprintf("mensaje %d", index)}
	}
}

// expectedPendingDialogs are the dialogs still open after the whole scenario.
func expectedPendingDialogs() []string {
	var pending []string
	for index := range eventCount {
		if index%5 == 0 && (index/5)%2 != 0 {
			pending = append(pending, fmt.Sprintf("dialog-%d", index))
		}
	}
	sort.Strings(pending)
	return pending
}

// persistedAgentEvents returns the agent events of the history in order, with their sequence numbers.
func persistedAgentEvents(t *testing.T, repo *Repo, id models.TaskID) ([]int64, []models.AgentEvent) {
	t.Helper()
	history, err := repo.Tasks.Events(bg(), id, nil)
	require.NoError(t, err)
	var seqs []int64
	var events []models.AgentEvent
	for _, event := range history {
		agent, ok := event.Kind.(models.AgentEventKind)
		if !ok || event.Seq == nil {
			continue
		}
		seqs = append(seqs, *event.Seq)
		events = append(events, agent.Event)
	}
	return seqs, events
}

// runningTaskWithModule inserts a running task and builds the module over the same database.
func runningTaskWithModule(t *testing.T) (*Repo, *di.Module, models.Task) {
	t.Helper()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/queue")
	task := newTask(t, workspaceID, "viva", models.StatusRunning)
	insertTask(t, repo, task)
	return repo, newModule(t, repo), task
}

// Rust: event_queue_resilience_test::no_agent_event_is_lost_while_sqlite_stays_locked_past_busy_timeout
// The reader must not wait for the database, and no event may be lost, duplicated or reordered once the
// lock is released.
func TestNoAgentEventIsLostWhileTheDatabaseStaysLockedPastTheBusyTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("holds a database lock for longer than the busy timeout")
	}
	t.Parallel()
	repo, module, task := runningTaskWithModule(t)

	locked := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- holdWriteLock(t, repo, locked, release)
	}()
	<-locked
	releaseTimer := time.AfterFunc(lockHold, func() { close(release) })
	defer releaseTimer.Stop()

	started := time.Now()
	for index := range eventCount {
		module.EventSink.Handle(bg(), task.ID, resilienceEvent(index))
	}
	enqueueTime := time.Since(started)
	assert.Less(t, enqueueTime, 2*time.Second, "the reader was blocked for %s", enqueueTime)

	deadline := time.Now().Add(30 * time.Second)
	var seqs []int64
	var events []models.AgentEvent
	for {
		seqs, events = persistedAgentEvents(t, repo, task.ID)
		if len(events) >= eventCount || time.Now().After(deadline) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	require.NoError(t, <-holderDone)
	module.ShutdownEvents()

	want := make([]models.AgentEvent, 0, eventCount)
	for index := range eventCount {
		want = append(want, resilienceEvent(index))
	}
	assert.Equal(t, want, events, "events were lost, duplicated or reordered")
	for index := 1; index < len(seqs); index++ {
		assert.Less(t, seqs[index-1], seqs[index], "seq must strictly increase")
	}
	stored := found(t, repo, task.ID)
	var pending []string
	for _, dialog := range stored.PendingDialogs {
		pending = append(pending, dialog.ID)
	}
	sort.Strings(pending)
	assert.Equal(t, expectedPendingDialogs(), pending, "pending dialogs are inconsistent")
}

// Rust: event_queue_resilience_test::flush_events_persists_everything_accepted_before_returning
func TestFlushPersistsEverythingAcceptedBeforeReturning(t *testing.T) {
	t.Parallel()
	repo, module, task := runningTaskWithModule(t)
	t.Cleanup(module.ShutdownEvents)

	for index := range eventCount {
		module.EventSink.Handle(bg(), task.ID, resilienceEvent(index))
	}
	module.FlushEvents()

	_, events := persistedAgentEvents(t, repo, task.ID)
	want := make([]models.AgentEvent, 0, eventCount)
	for index := range eventCount {
		want = append(want, resilienceEvent(index))
	}
	assert.Equal(t, want, events, "flush returned with events still unpersisted")
}

// Events reported to a module that is shutting down are still persisted, in order.
func TestEventsReportedAfterShutdownAreStillPersisted(t *testing.T) {
	t.Parallel()
	repo, module, task := runningTaskWithModule(t)

	module.ShutdownEvents()
	module.EventSink.Handle(context.Background(), task.ID, resilienceEvent(0))
	module.EventSink.Handle(context.Background(), task.ID, resilienceEvent(1))

	_, events := persistedAgentEvents(t, repo, task.ID)
	assert.Equal(t, []models.AgentEvent{resilienceEvent(0), resilienceEvent(1)}, events)
}
