package background

import (
	"context"
	"log/slog"
	"time"

	"iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// ResumeTick is how often the rate-limited tasks are checked for a due resume (30 seconds, as the Rust loop).
const ResumeTick = 30 * time.Second

// ResumeKind is the result of one resume sweep.
type ResumeKind int

// The outcomes of a sweep.
const (
	// ResumeIdle means no task was resumed.
	ResumeIdle ResumeKind = iota + 1
	// ResumeResumed means at least one task was resumed.
	ResumeResumed
	// ResumeFailed means the sweep failed; the loop goes on at the next tick.
	ResumeFailed
)

// ResumeOutcome is what one sweep produced: Resumed counts the resumed tasks and Err is set on failure.
type ResumeOutcome struct {
	Kind    ResumeKind
	Resumed int
	Err     error
}

// EvaluateResumeOnce runs one sweep at now and classifies the result.
func EvaluateResumeOnce(ctx context.Context, resumer inbound.RateLimitedResumer, now time.Time) ResumeOutcome {
	resumed, err := resumer.Execute(ctx, now)
	switch {
	case err != nil:
		return ResumeOutcome{Kind: ResumeFailed, Err: err}
	case resumed == 0:
		return ResumeOutcome{Kind: ResumeIdle}
	default:
		return ResumeOutcome{Kind: ResumeResumed, Resumed: resumed}
	}
}

// RunResumeLoop sweeps the rate-limited tasks until ctx ends, then returns nil. The first sweep runs at once,
// as the Rust tokio interval does. Each later sweep waits a full interval after the previous one finished, so
// a slow sweep delays the next one instead of queueing behind it (the Rust MissedTickBehavior::Delay). The
// instant of each sweep comes from the injected clock. The composition root runs it in a goroutine it owns.
func RunResumeLoop(ctx context.Context, resumer inbound.RateLimitedResumer, clock func() time.Time, interval time.Duration) error {
	if interval <= 0 {
		return ErrNonPositiveInterval
	}
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
		}
		logResumeOutcome(EvaluateResumeOnce(ctx, resumer, clock()))
		timer.Reset(interval)
	}
}

func logResumeOutcome(outcome ResumeOutcome) {
	switch outcome.Kind {
	case ResumeResumed:
		slog.Info("tasks resumed after the usage limit", "resumed", outcome.Resumed)
	case ResumeFailed:
		slog.Warn("could not resume the tasks after the usage limit", "error", outcome.Err)
	case ResumeIdle:
	}
}
