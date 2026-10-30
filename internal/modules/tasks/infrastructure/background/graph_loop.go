// Package background holds the long-running loops of the tasks module. Each loop stops when its context ends;
// the composition root runs it in a goroutine it owns and waits for.
package background

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// GraphTick is how often the dependency graph is evaluated again. It covers merges done outside the board and
// starts that failed and went back to backlog.
const GraphTick = 15 * time.Second

// ErrNonPositiveInterval is returned by RunGraphLoop when the interval cannot drive a ticker.
var ErrNonPositiveInterval = errors.New("graph loop: the interval must be positive")

// TickKind is the result of one evaluation of the graph.
type TickKind int

// The outcomes of an evaluation.
const (
	// TickIdle means nothing was started.
	TickIdle TickKind = iota + 1
	// TickStarted means at least one task was started.
	TickStarted
	// TickFailed means the evaluation failed; the loop goes on at the next tick.
	TickFailed
)

// TickOutcome is what one evaluation produced: Started counts the started tasks and Err is set on failure.
type TickOutcome struct {
	Kind    TickKind
	Started int
	Err     error
}

// EvaluateOnce runs the triggers once and classifies the result.
func EvaluateOnce(ctx context.Context, trigger inbound.SuccessorTrigger) TickOutcome {
	started, err := trigger.Trigger(ctx)
	switch {
	case err != nil:
		return TickOutcome{Kind: TickFailed, Err: err}
	case started == 0:
		return TickOutcome{Kind: TickIdle}
	default:
		return TickOutcome{Kind: TickStarted, Started: started}
	}
}

// RunGraphLoop evaluates the graph every interval until ctx ends, then returns nil. A tick that finds the
// previous evaluation still running is dropped, so evaluations never queue up.
func RunGraphLoop(ctx context.Context, trigger inbound.SuccessorTrigger, interval time.Duration) error {
	if interval <= 0 {
		return ErrNonPositiveInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			outcome := EvaluateOnce(ctx, trigger)
			switch outcome.Kind {
			case TickStarted:
				slog.Info("tasks started by the dependency graph", "started", outcome.Started)
			case TickFailed:
				slog.Warn("could not evaluate the dependency graph", "error", outcome.Err)
			case TickIdle:
			}
		}
	}
}
