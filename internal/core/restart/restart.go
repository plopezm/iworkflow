// Package restart implements restarting the server process, requested from the UI.
//
// The server shuts down gracefully and then either hands over to its supervisor (the desktop
// shell, which respawns it) or starts a fresh copy of itself.
package restart

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"sync"
	"sync/atomic"
)

// ExitCode tells a supervisor to restart the server (EX_TEMPFAIL).
const ExitCode int = 75

// SupervisedVar is set by a supervisor that respawns the server when it exits with ExitCode.
const SupervisedVar string = "IWF_SUPERVISED"

const (
	supervisedValue string = "1"
	noOpenFlag      string = "--no-open"
)

// Handle is the shared signal between the handler that asks for a restart and main, which owns
// the shutdown. Create it with NewHandle; the zero value is not usable.
type Handle struct {
	once      sync.Once
	requested atomic.Bool
	done      chan struct{}
}

// NewHandle returns a Handle with no restart requested.
func NewHandle() *Handle {
	return &Handle{done: make(chan struct{})}
}

// Request asks the server to shut down and start again. Safe to call more than once.
func (h *Handle) Request() {
	h.once.Do(func() {
		h.requested.Store(true)
		close(h.done)
	})
}

// IsRequested reports whether a restart was requested.
func (h *Handle) IsRequested() bool {
	return h.requested.Load()
}

// Requested returns a channel that is closed once a restart was requested, even if that happened
// before the call.
func (h *Handle) Requested() <-chan struct{} {
	return h.done
}

// IsSupervised reports whether a supervisor respawns the server after it exits with ExitCode.
func IsSupervised() bool {
	return supervisedFrom(os.Getenv(SupervisedVar))
}

// supervisedFrom accepts only the value the desktop shell sets, so "0" or an empty value does not count.
func supervisedFrom(value string) bool {
	return value == supervisedValue
}

// Action is what main does once the server has shut down.
type Action uint8

const (
	// ActionStop is a plain shutdown.
	ActionStop Action = iota
	// ActionHandOver exits with ExitCode so the supervisor starts the server again.
	ActionHandOver
	// ActionRespawn starts a fresh copy of this executable.
	ActionRespawn
)

// Decide returns the action to take after shutdown, given whether a restart was requested and
// whether a supervisor is present.
func Decide(restartRequested bool, supervised bool) Action {
	switch {
	case !restartRequested:
		return ActionStop
	case supervised:
		return ActionHandOver
	default:
		return ActionRespawn
	}
}

// RespawnArgs returns the arguments for the new process: the current ones, never reopening the
// browser tab. The caller's slice is not modified.
func RespawnArgs(args []string) []string {
	next := slices.Clone(args)
	if !slices.Contains(next, noOpenFlag) {
		next = append(next, noOpenFlag)
	}
	return next
}

// Respawn starts a detached copy of this executable. Call it only after the listener is closed.
func Respawn(ctx context.Context, args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	// The child must outlive a cancelled request context, so only its values are kept.
	cmd := exec.CommandContext(context.WithoutCancel(ctx), exe, RespawnArgs(args)...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start respawned server: %w", err)
	}
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release respawned server: %w", err)
	}
	return nil
}
