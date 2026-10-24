package pty

import (
	"context"
	"errors"
	"maps"
	"os"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"

	creackpty "github.com/creack/pty"

	"iworkflow/internal/core/env"
	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
)

// errShutdown reports a spawn attempted after Shutdown.
var errShutdown error = errors.New("el registro de terminales está cerrado")

// Adapter implements outbound.PtyPort. The registry is guarded by one mutex; each session has its
// own write lock, so a slow write never blocks the other sessions.
type Adapter struct {
	// shell is the fixed shell. Empty means $SHELL, with the platform fallback.
	shell    string
	mu       sync.Mutex
	sessions map[models.TerminalSessionID]*session
	closed   bool
}

// NewAdapter builds the adapter. An empty shell means $SHELL (sh when unset; PowerShell on Windows).
func NewAdapter(shell string) *Adapter {
	return &Adapter{shell: shell, sessions: make(map[models.TerminalSessionID]*session)}
}

// Spawn starts a login shell in a new PTY and registers the session.
func (a *Adapter) Spawn(_ context.Context, workspaceID uuid.UUID, workspaceName string, cwd string, size models.TerminalSize) (models.TerminalSession, error) {
	a.mu.Lock()
	closed := a.closed
	a.mu.Unlock()
	if closed {
		return models.TerminalSession{}, terminalerrors.Pty(errShutdown) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	flavor := env.NativeFlavor()
	shell := a.shell
	if shell == "" {
		shell = flavor.SelectShell(os.Getenv("SHELL"))
	}
	cmd := shellCommand(flavor, shell, cwd, parentVars())
	master, err := creackpty.StartWithSize(cmd, toWinsize(size))
	if err != nil {
		return models.TerminalSession{}, terminalerrors.Pty(err) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	output := newSharedOutput()
	meta := models.TerminalSession{
		ID:            models.NewTerminalSessionID(),
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
		CreatedAt:     time.Now().UTC(),
		Status:        models.TerminalStatusRunning,
	}
	current := newSession(meta, cmd, master, output)
	go func() {
		defer close(current.pumped)
		pump(master, output)
	}()

	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		current.terminate()
		return models.TerminalSession{}, terminalerrors.Pty(errShutdown) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	a.sessions[meta.ID] = current
	a.mu.Unlock()
	return current.snapshot(), nil
}

// List returns every registered session, in no particular order (the use case sorts them).
func (a *Adapter) List(_ context.Context) ([]models.TerminalSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	sessions := make([]models.TerminalSession, 0, len(a.sessions))
	for _, current := range a.sessions {
		sessions = append(sessions, current.snapshot())
	}
	return sessions, nil
}

// Attach returns the scrollback and a live channel for the session. An exited session yields its
// scrollback and a channel that is already closed.
func (a *Adapter) Attach(_ context.Context, id models.TerminalSessionID) (models.TerminalAttachment, error) {
	current, err := a.lookup(id)
	if err != nil {
		return models.TerminalAttachment{}, err
	}
	scrollback, output, detach := current.output.subscribe()
	return models.NewTerminalAttachment(scrollback, output, detach), nil
}

// Write sends input to a running session.
func (a *Adapter) Write(_ context.Context, id models.TerminalSessionID, data []byte) error {
	current, err := a.lookup(id)
	if err != nil {
		return err
	}
	return current.write(data)
}

// Resize changes the window size of a running session.
func (a *Adapter) Resize(_ context.Context, id models.TerminalSessionID, size models.TerminalSize) error {
	current, err := a.lookup(id)
	if err != nil {
		return err
	}
	return current.resize(size)
}

// Close removes the session, kills its processes and waits (until ctx is done) for the shell to be
// reaped. The session is removed even when the wait fails, so it cannot be used again.
func (a *Adapter) Close(ctx context.Context, id models.TerminalSessionID) error {
	a.mu.Lock()
	current, ok := a.sessions[id]
	if ok {
		delete(a.sessions, id)
	}
	a.mu.Unlock()
	if !ok {
		return terminalerrors.ErrSessionNotFound
	}
	current.terminate()
	return current.waitReaped(ctx)
}

// Shutdown kills every session and waits for all of them to stop. Later spawns fail. The composition
// root calls it when the server stops, so no shell outlives the process.
func (a *Adapter) Shutdown(ctx context.Context) error {
	a.mu.Lock()
	a.closed = true
	sessions := slices.Collect(maps.Values(a.sessions))
	clear(a.sessions)
	a.mu.Unlock()
	for _, current := range sessions {
		current.terminate()
	}
	var errs []error
	for _, current := range sessions {
		if err := current.waitStopped(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// lookup returns the registered session or ErrSessionNotFound.
func (a *Adapter) lookup(id models.TerminalSessionID) (*session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	current, ok := a.sessions[id]
	if !ok {
		return nil, terminalerrors.ErrSessionNotFound
	}
	return current, nil
}
