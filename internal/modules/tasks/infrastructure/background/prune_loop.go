package background

import (
	"context"
	"log/slog"
	"time"

	"iworkflow/internal/modules/tasks/domain/ports/inbound"
)

// PruneTick is how often the merged tasks past their retention are deleted.
const PruneTick = time.Hour

// RunPruneLoop prunes the old merged tasks until ctx ends, then returns nil. The first prune runs at once, so a
// server that restarts often still cleans up; each later one waits a full interval after the previous finished.
// The instant of each prune comes from the injected clock. The composition root runs it in a goroutine it owns.
func RunPruneLoop(ctx context.Context, pruner inbound.MergedTasksPruner, clock func() time.Time, interval time.Duration) error {
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
		pruned, err := pruner.Execute(ctx, clock())
		switch {
		case err != nil:
			slog.Warn("could not prune the old merged tasks", "error", err)
		case pruned > 0:
			slog.Info("old merged tasks deleted", "count", pruned)
		}
		timer.Reset(interval)
	}
}
