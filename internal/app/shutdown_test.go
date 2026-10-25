package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The step after a slow one still gets its own budget: the slow step runs out of its budget, and the flush step runs
// with a live context and persists.
func TestSlowStepDoesNotStarveTheFlushStep(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	flushed := make(chan error, 1)
	steps := []shutdownStep{
		{name: "stop the agents", run: func(ctx context.Context) error {
			<-ctx.Done()
			return ctx.Err()
		}},
		{name: "persist the agent events", run: func(ctx context.Context) error {
			flushed <- ctx.Err()
			return nil
		}},
	}

	err := runShutdownSteps(ctx, 50*time.Millisecond, steps)

	require.ErrorContains(t, err, "stop the agents")
	assert.NotContains(t, err.Error(), "persist the agent events")
	select {
	case flushErr := <-flushed:
		assert.NoError(t, flushErr, "the flush step must run with its own live budget")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the flush step never ran")
	}
}

// An already expired caller context does not skip the persist step: the step runs on a context that keeps the values
// but not the expired deadline.
func TestExpiredContextStillRunsThePersistStep(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := make(chan error, 1)
	steps := []shutdownStep{{name: "persist the agent events", run: func(ctx context.Context) error {
		ran <- ctx.Err()
		return nil
	}}}

	_ = runShutdownSteps(ctx, time.Second, steps)

	select {
	case stepErr := <-ran:
		assert.NoError(t, stepErr, "the persist step must not see the expired caller deadline")
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the persist step was skipped")
	}
}

// A step that finishes within its budget reports nil, and one that outlives it reports the timeout.
func TestBoundedStepReportsFinishedAndOverdueSteps(t *testing.T) {
	t.Parallel()

	require.NoError(t, boundedStep(context.Background(), "quick", time.Second, func(context.Context) error { return nil }))

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	err := boundedStep(context.Background(), "slow", 20*time.Millisecond, func(context.Context) error {
		select {
		case <-release:
		case <-time.After(2 * time.Second):
		}
		return nil
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "slow did not finish")
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// With an already expired caller context the result never reports success for a step that did not finish: nil is
// returned only when the step has returned, which the test observes through its own channel.
func TestBoundedStepNeverReportsAnUnfinishedStepAsFinished(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	finished := make(chan struct{})

	err := boundedStep(ctx, "persist", time.Second, func(context.Context) error {
		close(finished)
		return nil
	})

	if err == nil {
		select {
		case <-finished:
		default:
			assert.Fail(t, "a nil result must come from a finished step")
		}
	}
	assert.True(t, err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded))
}
