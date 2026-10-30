package application_test

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	inbound "iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// movableClock is the clock of the trigger tests; the test moves it forward by hand.
type movableClock struct {
	now time.Time
}

// Now returns the instant the test set.
func (c *movableClock) Now() time.Time {
	return c.now
}

// startRecorder records every start attempt and answers with fail(id) (nil means success). It is safe for
// concurrent use.
type startRecorder struct {
	mu       sync.Mutex
	attempts []models.TaskID
	fail     func(id models.TaskID) error
}

func (s *startRecorder) Start(_ context.Context, id models.TaskID) (models.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attempts = append(s.attempts, id)
	if s.fail != nil {
		if err := s.fail(id); err != nil {
			return models.Task{}, err
		}
	}
	return newTask(models.StatusStarting), nil
}

// claimOnce starts a task the first time and refuses the later starts, as the atomic claim of StartTask does.
type claimOnce struct {
	mu      sync.Mutex
	claimed map[models.TaskID]bool
}

func (c *claimOnce) Start(_ context.Context, id models.TaskID) (models.Task, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.claimed[id] {
		return models.Task{}, werrors.InvalidState("arrancar", "starting")
	}
	c.claimed[id] = true
	return newTask(models.StatusStarting), nil
}

// warningCapture is a slog handler that keeps the messages of the warnings it receives.
type warningCapture struct {
	mu       sync.Mutex
	messages []string
}

func (c *warningCapture) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (c *warningCapture) Handle(_ context.Context, record slog.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, record.Message)
	return nil
}

func (c *warningCapture) WithAttrs(_ []slog.Attr) slog.Handler {
	return c
}

func (c *warningCapture) WithGroup(_ string) slog.Handler {
	return c
}

// countOf returns how many captured warnings have the message.
func (c *warningCapture) countOf(message string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := 0
	for _, captured := range c.messages {
		if captured == message {
			count++
		}
	}
	return count
}

// triggerUseCase wires TriggerSuccessors over the given repository, graph, starter and clock.
func triggerUseCase(repo *fakeRepository, graph *fakeGraph, starter inbound.StartTaskUseCase, clock *movableClock) *application.TriggerSuccessors {
	return application.NewTriggerSuccessors(repo, graph, starter, clock.Now)
}

// Rust: trigger_successors::starts_only_successors_whose_predecessors_are_merged
func TestTriggerSuccessorsStartsOnlySuccessorsWhosePredecessorsAreMerged(t *testing.T) {
	t.Parallel()
	merged := newTask(models.StatusMerged)
	reviewing := newTask(models.StatusAwaitingReview)
	ready := newTask(models.StatusBacklog)
	blocked := newTask(models.StatusBacklog)
	repo := newFakeRepository(merged, reviewing, ready, blocked)
	edges := []models.TaskDependency{edgeOf(merged.ID, ready.ID), edgeOf(reviewing.ID, blocked.ID)}
	graph := &fakeGraph{edges: edges}
	starter := &startRecorder{}

	started, err := triggerUseCase(repo, graph, starter, &movableClock{now: fixedNow}).Trigger(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, started)
	assert.Equal(t, []models.TaskID{ready.ID}, starter.attempts)
	assert.Equal(t, edges, graph.edges)
}

// Rust: trigger_successors::without_edges_nothing_is_loaded_or_started
func TestTriggerSuccessorsWithoutEdgesNothingIsLoadedOrStarted(t *testing.T) {
	t.Parallel()
	repo := newFakeRepository(newTask(models.StatusBacklog))
	starter := &startRecorder{}

	started, err := triggerUseCase(repo, &fakeGraph{}, starter, &movableClock{now: fixedNow}).Trigger(context.Background())

	require.NoError(t, err)
	assert.Zero(t, started)
	assert.Zero(t, repo.finds.Load())
	assert.Empty(t, starter.attempts)
}

// Rust: trigger_successors::a_failing_start_does_not_stop_the_other_successors
func TestTriggerSuccessorsAFailingStartDoesNotStopTheOtherSuccessors(t *testing.T) {
	t.Parallel()
	merged := newTask(models.StatusMerged)
	first := newTask(models.StatusBacklog)
	second := newTask(models.StatusBacklog)
	repo := newFakeRepository(merged, first, second)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(merged.ID, first.ID), edgeOf(merged.ID, second.ID)}}
	starter := &startRecorder{fail: func(id models.TaskID) error {
		if id == first.ID {
			return werrors.Git("boom")
		}
		return nil
	}}

	started, err := triggerUseCase(repo, graph, starter, &movableClock{now: fixedNow}).Trigger(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, started)
	assert.ElementsMatch(t, []models.TaskID{first.ID, second.ID}, starter.attempts)
	assert.Len(t, graph.edges, 2)
}

// Rust: trigger_successors::a_failing_start_is_skipped_until_the_delay_elapses
func TestTriggerSuccessorsAFailingStartIsSkippedUntilTheDelayElapses(t *testing.T) {
	t.Parallel()
	merged := newTask(models.StatusMerged)
	next := newTask(models.StatusBacklog)
	repo := newFakeRepository(merged, next)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(merged.ID, next.ID)}}
	starter := &startRecorder{fail: func(models.TaskID) error { return werrors.Git("boom") }}
	clock := &movableClock{now: fixedNow}
	trigger := triggerUseCase(repo, graph, starter, clock)

	// Attempt at 0 s fails and waits 15 s; 10 s is skipped; 15 s retries and fails, waiting 30 s;
	// 44 s is skipped; 45 s retries.
	results := make([]int, 0, 5)
	for _, offset := range []int64{0, 10, 15, 44, 45} {
		clock.now = fixedNow.Add(time.Duration(offset) * time.Second)
		started, err := trigger.Trigger(context.Background())
		require.NoError(t, err)
		results = append(results, started)
	}

	assert.Equal(t, []int{0, 0, 0, 0, 0}, results)
	assert.Len(t, starter.attempts, 3)
}

// Rust: trigger_successors::success_clears_the_backoff_state
// The state is observed through behaviour: after a success the failure count starts again, so the next
// failure waits the base delay (15 s) and not the accumulated one.
func TestTriggerSuccessorsSuccessClearsTheBackoffState(t *testing.T) {
	t.Parallel()
	merged := newTask(models.StatusMerged)
	next := newTask(models.StatusBacklog)
	repo := newFakeRepository(merged, next)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(merged.ID, next.ID)}}
	failuresLeft := 1
	starter := &startRecorder{fail: func(models.TaskID) error {
		if failuresLeft > 0 {
			failuresLeft--
			return werrors.Git("boom")
		}
		return nil
	}}
	clock := &movableClock{now: fixedNow}
	trigger := triggerUseCase(repo, graph, starter, clock)

	started, err := trigger.Trigger(context.Background())
	require.NoError(t, err)
	assert.Zero(t, started)

	clock.now = fixedNow.Add(15 * time.Second)
	started, err = trigger.Trigger(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, started)

	// The task is back in backlog and fails once more: the new failure waits 15 s from now.
	repo.tasks[next.ID] = next
	failuresLeft = 1
	clock.now = fixedNow.Add(15 * time.Second)
	_, err = trigger.Trigger(context.Background())
	require.NoError(t, err)
	attemptsAfterFailure := len(starter.attempts)

	clock.now = fixedNow.Add(29 * time.Second)
	_, err = trigger.Trigger(context.Background())
	require.NoError(t, err)
	assert.Len(t, starter.attempts, attemptsAfterFailure, "still waiting the base delay")

	clock.now = fixedNow.Add(30 * time.Second)
	_, err = trigger.Trigger(context.Background())
	require.NoError(t, err)
	assert.Len(t, starter.attempts, attemptsAfterFailure+1, "retried once the base delay elapsed")
}

// Rust: trigger_successors::backoff_is_dropped_when_the_task_leaves_backlog
// Observed through behaviour: once the task leaves backlog its backoff is forgotten, so when it is
// backlog again it is started at once instead of waiting for the old delay.
func TestTriggerSuccessorsBackoffIsDroppedWhenTheTaskLeavesBacklog(t *testing.T) {
	t.Parallel()
	merged := newTask(models.StatusMerged)
	next := newTask(models.StatusBacklog)
	repo := newFakeRepository(merged, next)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(merged.ID, next.ID)}}
	starter := &startRecorder{fail: func(models.TaskID) error { return werrors.Git("boom") }}
	trigger := triggerUseCase(repo, graph, starter, &movableClock{now: fixedNow})

	_, err := trigger.Trigger(context.Background())
	require.NoError(t, err)
	require.Len(t, starter.attempts, 1)

	running := next
	running.Status = models.StatusRunning
	repo.tasks[next.ID] = running
	_, err = trigger.Trigger(context.Background())
	require.NoError(t, err)
	assert.Len(t, starter.attempts, 1, "a running task is not a candidate")

	repo.tasks[next.ID] = next
	_, err = trigger.Trigger(context.Background())
	require.NoError(t, err)
	assert.Len(t, starter.attempts, 2, "the old backoff is gone, so the task is attempted at once")
}

// Rust: trigger_successors::blocked_by_discarded_is_not_started_and_warned_once
// It replaces the process logger to count the warnings, so it must not run in parallel.
func TestTriggerSuccessorsBlockedByDiscardedIsNotStartedAndWarnedOnce(t *testing.T) {
	capture := &warningCapture{}
	previous := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(previous) })
	const blockedWarning string = "task blocked: a predecessor was discarded; remove the dependency to unblock it"

	discarded := newTask(models.StatusDiscarded)
	stuck := newTask(models.StatusBacklog)
	repo := newFakeRepository(discarded, stuck)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(discarded.ID, stuck.ID)}}
	starter := &startRecorder{}
	trigger := triggerUseCase(repo, graph, starter, &movableClock{now: fixedNow})

	first, err := trigger.Trigger(context.Background())
	require.NoError(t, err)
	second, err := trigger.Trigger(context.Background())
	require.NoError(t, err)

	assert.Zero(t, first)
	assert.Zero(t, second)
	assert.Empty(t, starter.attempts)
	assert.Equal(t, 1, capture.countOf(blockedWarning))
	assert.Equal(t, models.StatusBacklog, repo.tasks[stuck.ID].Status)
}

// Concurrency: evaluations run from several goroutines (graph loop, merges, graph edits). The starter
// claims each task once, so exactly one evaluation starts each successor, whatever the interleaving.
func TestTriggerSuccessorsConcurrentEvaluationsStartEachTaskOnce(t *testing.T) {
	t.Parallel()
	merged := newTask(models.StatusMerged)
	first := newTask(models.StatusBacklog)
	second := newTask(models.StatusBacklog)
	repo := newFakeRepository(merged, first, second)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(merged.ID, first.ID), edgeOf(merged.ID, second.ID)}}
	starter := &claimOnce{claimed: map[models.TaskID]bool{}}
	trigger := triggerUseCase(repo, graph, starter, &movableClock{now: fixedNow})

	const evaluations int = 8
	results := make([]int, evaluations)
	errs := make([]error, evaluations)
	var wg sync.WaitGroup
	for index := range evaluations {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[index], errs[index] = trigger.Trigger(context.Background())
		}()
	}
	wg.Wait()

	total := 0
	for index := range evaluations {
		require.NoError(t, errs[index])
		total += results[index]
	}
	assert.Equal(t, 2, total)
	assert.Len(t, starter.claimed, 2)
}
