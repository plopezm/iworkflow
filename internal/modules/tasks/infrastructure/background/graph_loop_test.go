package background_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/infrastructure/background"
)

// countingTrigger counts its evaluations and answers the call-th one (calls start at 1) with answer. It is
// safe for concurrent use, because the loop runs in its own goroutine.
type countingTrigger struct {
	calls  atomic.Int64
	answer func(call int64) (int, error)
}

func (c *countingTrigger) Trigger(_ context.Context) (int, error) {
	return c.answer(c.calls.Add(1))
}

// fixedAnswer answers every evaluation with the same result.
func fixedAnswer(started int, err error) func(call int64) (int, error) {
	return func(int64) (int, error) {
		return started, err
	}
}

// Rust: graph_loop::no_started_tasks_is_idle
func TestEvaluateOnceWithoutStartedTasksIsIdle(t *testing.T) {
	t.Parallel()
	trigger := &countingTrigger{answer: fixedAnswer(0, nil)}

	outcome := background.EvaluateOnce(context.Background(), trigger)

	assert.Equal(t, background.TickOutcome{Kind: background.TickIdle}, outcome)
	assert.Equal(t, int64(1), trigger.calls.Load())
}

// Rust: graph_loop::started_tasks_are_reported
func TestEvaluateOnceReportsTheStartedTasks(t *testing.T) {
	t.Parallel()
	trigger := &countingTrigger{answer: fixedAnswer(3, nil)}

	outcome := background.EvaluateOnce(context.Background(), trigger)

	assert.Equal(t, background.TickOutcome{Kind: background.TickStarted, Started: 3}, outcome)
	assert.Equal(t, int64(1), trigger.calls.Load())
}

// Rust: graph_loop::errors_are_reported_not_propagated
func TestEvaluateOnceReportsErrorsWithoutPropagatingThem(t *testing.T) {
	t.Parallel()
	trigger := &countingTrigger{answer: fixedAnswer(0, werrors.Git("boom"))}

	outcome := background.EvaluateOnce(context.Background(), trigger)

	assert.Equal(t, background.TickOutcome{Kind: background.TickFailed, Err: werrors.Git("boom")}, outcome)
}

// Triangulation (no Rust counterpart): the production tick is 15 s, as in the Rust loop.
func TestGraphTickIsFifteenSeconds(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 15*time.Second, background.GraphTick)
}

// Triangulation (no Rust counterpart): the loop evaluates on every tick and returns, with no error, once the
// context ends. It is the owner of its goroutine's end: no evaluation runs after it returns.
func TestRunGraphLoopEvaluatesEveryTickUntilTheContextEnds(t *testing.T) {
	t.Parallel()
	trigger := &countingTrigger{answer: fixedAnswer(0, nil)}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- background.RunGraphLoop(ctx, trigger, time.Millisecond) }()

	require.Eventually(t, func() bool { return trigger.calls.Load() >= 3 }, 5*time.Second, time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the loop did not stop after the context was cancelled")
	}
	stopped := trigger.calls.Load()
	assert.GreaterOrEqual(t, stopped, int64(3))
	assert.Equal(t, stopped, trigger.calls.Load(), "no evaluation runs after the loop returned")
}

// Triangulation (no Rust counterpart): a failed evaluation is only reported; the loop keeps ticking.
func TestRunGraphLoopKeepsTickingAfterAFailedEvaluation(t *testing.T) {
	t.Parallel()
	trigger := &countingTrigger{answer: func(call int64) (int, error) {
		if call == 1 {
			return 0, werrors.Repository("boom")
		}
		return 0, nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	done := make(chan error, 1)
	go func() { done <- background.RunGraphLoop(ctx, trigger, time.Millisecond) }()

	require.Eventually(t, func() bool { return trigger.calls.Load() >= 3 }, 5*time.Second, time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the loop did not stop after the context was cancelled")
	}
}

// Triangulation (no Rust counterpart): a non-positive interval is refused before the loop starts. A ticker
// cannot be built with it, and the loop must not panic.
func TestRunGraphLoopRejectsANonPositiveInterval(t *testing.T) {
	t.Parallel()
	trigger := &countingTrigger{answer: fixedAnswer(0, nil)}

	err := background.RunGraphLoop(context.Background(), trigger, 0)

	require.ErrorIs(t, err, background.ErrNonPositiveInterval)
	assert.Zero(t, trigger.calls.Load())
}
