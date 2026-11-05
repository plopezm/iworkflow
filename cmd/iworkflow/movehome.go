package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"time"

	"iworkflow/internal/core/config"
	"iworkflow/internal/core/datahome"
)

// applyPendingMove carries out a data folder move requested from the settings screen. It must run before
// the log file or the database are opened: both live in the folder being moved, and nothing may write to it
// while it is copied. A failed move is logged and the old folder stays in use; only an interruption is an error,
// and then the move stays pending for the next start.
func applyPendingMove(ctx context.Context, getenv func(string) string, logger *slog.Logger) error {
	mover, err := datahome.NewMover(runtime.GOOS, getenv, time.Now, logger.Info)
	if errors.Is(err, datahome.ErrNoConfigDir) {
		return nil // without a configuration folder no move can have been requested
	}
	if err != nil {
		return fmt.Errorf("data folder move: %w", err)
	}
	if strings.TrimSpace(getenv("IWF_HOME")) != "" {
		// IWF_HOME wins over the pointer, so the old folder would stay in use while its data was moved and deleted.
		result, err := mover.Refuse("IWF_HOME is set, so the data folder cannot be moved from the settings screen; the old folder stays in use")
		if err != nil {
			return fmt.Errorf("data folder move: %w", err)
		}
		if result != nil {
			logger.Error("data folder move: refused", "error", result.Error)
		}
		return nil
	}
	current, err := config.DataDir(getenv)
	if err != nil {
		return nil //nolint:nilerr // A broken data folder setting is reported by the normal start-up.
	}
	result, err := mover.Apply(ctx, current)
	if err != nil {
		return fmt.Errorf("data folder move: %w", err)
	}
	switch {
	case result == nil:
	case result.OK:
		logger.Info("data folder move: done", "from", result.From, "to", result.To, "warnings", len(result.Warnings))
		for _, warning := range result.Warnings {
			logger.Warn("data folder move: " + warning)
		}
	default:
		logger.Error("data folder move: failed; the old folder stays in use", "from", result.From, "to", result.To, "error", result.Error)
	}
	return nil
}
