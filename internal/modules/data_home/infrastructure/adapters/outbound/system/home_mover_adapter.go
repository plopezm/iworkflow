// Package system adapts the data folder move engine of the core (internal/core/datahome) and the
// resolution of config.DataDir to the data_home ports.
package system

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"iworkflow/internal/core/config"
	"iworkflow/internal/core/datahome"
	domainerrors "iworkflow/internal/modules/data_home/domain/errors"
	"iworkflow/internal/modules/data_home/domain/models"
)

// destinationReasons maps the validation sentinels of the move engine to the reasons of the domain.
var destinationReasons = []struct {
	sentinel error
	reason   domainerrors.DestinationReason
}{
	{sentinel: datahome.ErrNotAbsolute, reason: domainerrors.ReasonRelative},
	{sentinel: datahome.ErrSameFolder, reason: domainerrors.ReasonSame},
	{sentinel: datahome.ErrNested, reason: domainerrors.ReasonNested},
	{sentinel: datahome.ErrNotEmpty, reason: domainerrors.ReasonNotEmpty},
	{sentinel: datahome.ErrNotWritable, reason: domainerrors.ReasonNotWritable},
	{sentinel: datahome.ErrNoSpace, reason: domainerrors.ReasonNoSpace},
	{sentinel: datahome.ErrUnsafeSource, reason: domainerrors.ReasonSourceUnsafe},
	{sentinel: datahome.ErrNotDataFolder, reason: domainerrors.ReasonSourceMissing},
}

// HomeMoverAdapter implements outbound.HomeMoverPort over the move engine. The move engine keeps its
// bootstrap files next to the per-user pointer; when that folder cannot be found, every method reports
// domainerrors.ErrUnavailable instead of failing the start-up.
type HomeMoverAdapter struct {
	mover  *datahome.Mover
	err    error
	getenv func(string) string
}

// NewHomeMoverAdapter builds the adapter of this machine. goos, getenv, now and log are the inputs of
// the move engine: runtime.GOOS, os.Getenv, time.Now and the logger in production.
func NewHomeMoverAdapter(goos string, getenv func(string) string, now func() time.Time, log func(msg string, args ...any)) *HomeMoverAdapter {
	mover, err := datahome.NewMover(goos, getenv, now, log)
	return &HomeMoverAdapter{mover: mover, err: err, getenv: getenv}
}

// Status reports the folder in use, its source and the pending and last moves.
func (a *HomeMoverAdapter) Status(context.Context) (models.HomeStatus, error) {
	if a.err != nil {
		return models.HomeStatus{}, fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, a.err)
	}
	current, err := config.DataDir(a.getenv)
	if err != nil {
		return models.HomeStatus{}, fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, err)
	}
	source, err := a.source()
	if err != nil {
		return models.HomeStatus{}, err
	}
	status := models.HomeStatus{Current: current, Source: source, EnvLocked: source == models.SourceEnv}
	pending, err := a.mover.Pending()
	if err != nil {
		return models.HomeStatus{}, fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, err)
	}
	if pending != nil {
		status.Pending = &models.PendingMove{From: pending.From, To: pending.To}
	}
	last, err := a.mover.LastResult()
	if err != nil {
		return models.HomeStatus{}, fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, err)
	}
	if last != nil {
		status.Last = &models.MoveOutcome{OK: last.OK, From: last.From, To: last.To, Error: last.Error, Warnings: last.Warnings, FinishedAt: last.FinishedAt}
	}
	return status, nil
}

// source applies the precedence of config.DataDir: IWF_HOME, then the pointer, then the default folder.
func (a *HomeMoverAdapter) source() (models.HomeSource, error) {
	if strings.TrimSpace(a.getenv("IWF_HOME")) != "" {
		return models.SourceEnv, nil
	}
	pointer, err := datahome.Read(a.mover.Files.Pointer)
	if err != nil {
		return "", fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, err)
	}
	if pointer != "" {
		return models.SourcePointer, nil
	}
	return models.SourceDefault, nil
}

// Request records a move of the current folder to destination. The move engine validates the
// destination; its refusals become domain errors with their reason.
func (a *HomeMoverAdapter) Request(ctx context.Context, destination string) error {
	if a.err != nil {
		return fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, a.err)
	}
	current, err := config.DataDir(a.getenv)
	if err != nil {
		return fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, err)
	}
	if err := a.mover.Request(current, destination); err != nil {
		return mapRequestError(err)
	}
	return nil
}

// mapRequestError turns a refusal of the move engine into the domain error of the same meaning.
func mapRequestError(err error) error {
	if errors.Is(err, datahome.ErrPending) {
		return fmt.Errorf("%w: %w", domainerrors.ErrMovePending, err)
	}
	for _, candidate := range destinationReasons {
		if errors.Is(err, candidate.sentinel) {
			return &domainerrors.DestinationError{Reason: candidate.reason}
		}
	}
	return fmt.Errorf("record the move: %w", err)
}

// Cancel withdraws the pending move.
func (a *HomeMoverAdapter) Cancel(context.Context) error {
	if a.err != nil {
		return fmt.Errorf("%w: %w", domainerrors.ErrUnavailable, a.err)
	}
	if err := a.mover.Cancel(); err != nil {
		return fmt.Errorf("cancel the move: %w", err)
	}
	return nil
}
