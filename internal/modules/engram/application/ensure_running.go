package application

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	engramerrors "iworkflow/internal/modules/engram/domain/errors"
	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

const (
	startPollInterval = 500 * time.Millisecond
	startPollAttempts = 20
	// startGrace is how long after a start this process only waits for Engram instead of starting it again, so a slow
	// cold start is never launched twice.
	startGrace = 30 * time.Second
)

func trimSpace(value string) string { return strings.TrimSpace(value) }

// EnsureRunning starts Engram only when nothing answers at the configured address.
type EnsureRunning struct {
	process outbound.ServerProcess
	sleep   func(ctx context.Context, d time.Duration) error
	now     func() time.Time

	mu        sync.Mutex
	startedAt map[string]time.Time // port -> when this process last started Engram there
}

// NewEnsureRunning builds the use case. sleep waits between health checks and must return early with the context error.
func NewEnsureRunning(process outbound.ServerProcess, sleep func(ctx context.Context, d time.Duration) error, now func() time.Time) *EnsureRunning {
	return &EnsureRunning{process: process, sleep: sleep, now: now, startedAt: map[string]time.Time{}}
}

// claimStart reports whether another start on port is still inside its grace period. When none is, it claims the start
// now, so the one caller that gets false launches Engram and a concurrent caller only waits. A launch that fails must be
// released with releaseStart, so fixing the cause and applying again retries at once.
func (u *EnsureRunning) claimStart(port string) bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	now := u.now()
	if last, ok := u.startedAt[port]; ok && now.Sub(last) < startGrace {
		return true
	}
	u.startedAt[port] = now
	return false
}

// releaseStart drops the claim of a start that failed to launch.
func (u *EnsureRunning) releaseStart(port string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	delete(u.startedAt, port)
}

// Execute checks the server and starts it only when it is off and the address allows it. It never starts a second
// server: an answering one is left alone. After starting, it waits a bounded time for the server to answer.
func (u *EnsureRunning) Execute(ctx context.Context, settings models.ServerSettings) (models.StartOutcome, error) {
	if u.process.Healthy(ctx, settings.URL) {
		return models.OutcomeRunning, nil
	}
	if !settings.AutoStart() {
		return models.OutcomeSkipped, nil
	}
	if u.claimStart(settings.Port()) {
		// A start is still in progress: wait for it instead of launching a second Engram.
		return u.waitUntilHealthy(ctx, settings)
	}
	if err := u.process.Start(ctx, settings.Port()); err != nil {
		// Nothing was launched, so there is nothing to wait for: the next attempt may launch again.
		u.releaseStart(settings.Port())
		return "", fmt.Errorf("start engram on port %s: %w", settings.Port(), err)
	}
	return u.waitUntilHealthy(ctx, settings)
}

// waitUntilHealthy polls the server for a bounded time after a start.
func (u *EnsureRunning) waitUntilHealthy(ctx context.Context, settings models.ServerSettings) (models.StartOutcome, error) {
	for range startPollAttempts {
		if err := u.sleep(ctx, startPollInterval); err != nil {
			return "", fmt.Errorf("wait for engram: %w", err)
		}
		if u.process.Healthy(ctx, settings.URL) {
			return models.OutcomeStarted, nil
		}
	}
	return "", fmt.Errorf("engram on port %s: %w", settings.Port(), engramerrors.ErrStartFailed)
}
