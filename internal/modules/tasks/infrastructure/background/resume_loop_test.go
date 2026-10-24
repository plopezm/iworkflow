package background_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/infrastructure/background"
)

// resumerFake records every evaluation and answers the call-th one (calls start at 1). It is safe for
// concurrent use, because the loop runs in its own goroutine.
type resumerFake struct {
	mu       sync.Mutex
	calls    int
	instants []time.Time
	answer   func(call int) (int, error)
	// called receives the instant of every evaluation; the send never blocks the loop.
	called chan time.Time
}

func newResumerFake(answer func(call int) (int, error)) *resumerFake {
	return &resumerFake{answer: answer, called: make(chan time.Time, 1000)}
}

func (r *resumerFake) Execute(_ context.Context, now time.Time) (int, error) {
	r.mu.Lock()
	r.calls++
	call := r.calls
	r.instants = append(r.instants, now)
	r.mu.Unlock()
	select {
	case r.called <- now:
	default:
	}
	return r.answer(call)
}

func (r *resumerFake) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func (r *resumerFake) instantsSeen() []time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]time.Time(nil), r.instants...)
}

var loopNow = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

func loopClock() time.Time { return loopNow }

// runLoopAsync starts the loop in a goroutine the test owns and returns the channel of its result.
func runLoopAsync(ctx context.Context, resumer inbound.RateLimitedResumer, interval time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- background.RunResumeLoop(ctx, resumer, loopClock, interval)
	}()
	return done
}

// waitForEvaluation fails the test when no evaluation happens in time, instead of hanging.
func waitForEvaluation(t *testing.T, resumer *resumerFake) {
	t.Helper()
	select {
	case <-resumer.called:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for an evaluation")
	}
}

// Rule from source: resume_loop.rs::RESUME_TICK is 30 seconds.
func TestResumeTickIsThirtySeconds(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 30*time.Second, background.ResumeTick)
}

// Rule from source: resume_loop.rs::run_resume_loop uses a tokio interval, whose first tick fires at once. The
// first evaluation therefore runs at startup, with the injected clock's instant, before any interval elapses.
func TestRunResumeLoopRunsTheFirstEvaluationImmediately(t *testing.T) {
	t.Parallel()
	resumer := newResumerFake(func(int) (int, error) { return 0, nil })
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoopAsync(ctx, resumer, time.Hour)

	waitForEvaluation(t, resumer)
	cancel()

	require.NoError(t, <-done)
	require.Equal(t, 1, resumer.callCount())
	seen := resumer.instantsSeen()
	require.Len(t, seen, 1)
	assert.True(t, seen[0].Equal(loopNow))
}

// Rule from source: resume_loop.rs::run_resume_loop evaluates once per tick until it is aborted. The loop returns
// nil when its context ends and makes no further evaluation after that.
func TestRunResumeLoopEvaluatesOncePerIntervalUntilItsContextEnds(t *testing.T) {
	t.Parallel()
	resumer := newResumerFake(func(int) (int, error) { return 0, nil })
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoopAsync(ctx, resumer, 5*time.Millisecond)

	for range 3 {
		waitForEvaluation(t, resumer)
	}
	cancel()

	require.NoError(t, <-done)
	stoppedAt := resumer.callCount()
	assert.GreaterOrEqual(t, stoppedAt, 3)
	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, stoppedAt, resumer.callCount())
}

// Rule from source: resume_loop.rs::run_resume_loop keeps ticking after a failed evaluation (it logs and goes on).
func TestRunResumeLoopKeepsGoingAfterAFailedEvaluation(t *testing.T) {
	t.Parallel()
	resumer := newResumerFake(func(call int) (int, error) {
		if call == 1 {
			return 0, errors.New("database is locked")
		}
		return 1, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := runLoopAsync(ctx, resumer, 5*time.Millisecond)

	waitForEvaluation(t, resumer)
	waitForEvaluation(t, resumer)
	cancel()

	require.NoError(t, <-done)
	assert.GreaterOrEqual(t, resumer.callCount(), 2)
}

// Rule from source: a loop without a positive interval cannot drive a ticker; it is rejected before any evaluation.
func TestRunResumeLoopRejectsANonPositiveInterval(t *testing.T) {
	t.Parallel()
	for _, interval := range []time.Duration{0, -time.Second} {
		resumer := newResumerFake(func(int) (int, error) { return 0, nil })

		err := background.RunResumeLoop(context.Background(), resumer, loopClock, interval)

		require.ErrorIs(t, err, background.ErrNonPositiveInterval)
		assert.Equal(t, 0, resumer.callCount())
	}
}

// Rule from source: resume_loop.rs::run_resume_loop logs a non-zero count as info, zero silently, and a failure as a
// warning. EvaluateResumeOnce returns the classification the loop logs.
func TestEvaluateResumeOnceClassifiesTheResult(t *testing.T) {
	t.Parallel()
	failure := errors.New("database is locked")
	cases := []struct {
		name    string
		answer  int
		err     error
		kind    background.ResumeKind
		resumed int
	}{
		{name: "nothing due", answer: 0, kind: background.ResumeIdle},
		{name: "some tasks resumed", answer: 3, kind: background.ResumeResumed, resumed: 3},
		{name: "evaluation failed", err: failure, kind: background.ResumeFailed},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			resumer := newResumerFake(func(int) (int, error) { return testCase.answer, testCase.err })

			outcome := background.EvaluateResumeOnce(context.Background(), resumer, loopNow)

			assert.Equal(t, testCase.kind, outcome.Kind)
			assert.Equal(t, testCase.resumed, outcome.Resumed)
			assert.Equal(t, testCase.err, outcome.Err)
			assert.True(t, resumer.instantsSeen()[0].Equal(loopNow))
		})
	}
}
