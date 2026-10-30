package pty

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"

	creackpty "github.com/creack/pty"

	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
)

var (
	// errSessionEnded is the message of writes and resizes sent to a shell that already exited.
	errSessionEnded error = errors.New("la sesión de terminal ya terminó")
	// errStopTimeout reports a session whose shell or reader did not finish before the caller gave up.
	errStopTimeout error = errors.New("la sesión no terminó a tiempo")
)

// session is one live shell with its PTY. The registry owns it and stops it exactly once.
type session struct {
	meta     models.TerminalSession
	cmd      *exec.Cmd
	master   *os.File
	output   *sharedOutput
	writeMu  sync.Mutex
	stopMu   sync.Once
	lastSize models.TerminalSize
	// reaped is closed when the shell has been waited for; pumped when the reader has finished.
	reaped chan struct{}
	pumped chan struct{}
}

func newSession(meta models.TerminalSession, cmd *exec.Cmd, master *os.File, output *sharedOutput) *session {
	return &session{
		meta:   meta,
		cmd:    cmd,
		master: master,
		output: output,
		reaped: make(chan struct{}),
		pumped: make(chan struct{}),
	}
}

// snapshot returns the metadata with the status that the shell has now.
func (s *session) snapshot() models.TerminalSession {
	snapshot := s.meta
	if s.output.isRunning() {
		snapshot.Status = models.TerminalStatusRunning
	} else {
		snapshot.Status = models.TerminalStatusExited
	}
	return snapshot
}

// ensureRunning fails with a clear error once the shell has ended; the session stays listed until it is closed.
func (s *session) ensureRunning() error {
	if !s.output.isRunning() {
		return terminalerrors.Pty(errSessionEnded) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	return nil
}

// write sends bytes to the shell and flushes them. The PTY is unbuffered, so the write is the flush.
func (s *session) write(data []byte) error {
	if err := s.ensureRunning(); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.master.Write(data); err != nil {
		return terminalerrors.Pty(err) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	return nil
}

// resize updates the window size of the PTY, which delivers SIGWINCH to the shell.
func (s *session) resize(size models.TerminalSize) error {
	if err := s.ensureRunning(); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	// If size is unchanged, nudge the dimensions by 1 row to force the kernel and TUI
	// applications (like pi) to detect a window size change and repaint the full screen.
	if s.lastSize == size && size.Rows > 1 {
		nudged := size
		nudged.Rows--
		_ = creackpty.Setsize(s.master, toWinsize(nudged))
	}

	if err := creackpty.Setsize(s.master, toWinsize(size)); err != nil {
		return terminalerrors.Pty(err) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
	s.lastSize = size
	return nil
}

// terminate kills the shell, its jobs and every process that left the session, and releases the
// PTY. It does not wait: call wait to know when the shell has been reaped and the reader has ended.
// The processes are killed before the shell, so they cannot be re-parented to init and escape.
func (s *session) terminate() {
	s.stopMu.Do(func() {
		killProcessTree(s.cmd.Process.Pid)
		// Process.Kill fails only when the shell is already gone; that is the state we want.
		_ = s.cmd.Process.Kill()
		go func() {
			// The exit status is irrelevant: the session is being discarded. Waiting reaps the
			// shell so it does not stay as a zombie until the server exits.
			_ = s.cmd.Wait()
			close(s.reaped)
		}()
		// Closing the master ends the reader with EIO and releases the terminal.
		_ = s.master.Close()
	})
}

// waitReaped blocks until the shell has been reaped, or until ctx is done. This is the guarantee that
// no child is left behind. It does not wait for the reader: the reader ends when the last process that
// holds the terminal exits, which may be after a process that escaped the kill.
func (s *session) waitReaped(ctx context.Context) error {
	return waitFor(ctx, s.reaped)
}

// waitStopped blocks until the shell is reaped and the reader has finished, or until ctx is done.
func (s *session) waitStopped(ctx context.Context) error {
	if err := waitFor(ctx, s.reaped); err != nil {
		return err
	}
	return waitFor(ctx, s.pumped)
}

// waitFor blocks until done is closed or ctx is done, whichever comes first.
func waitFor(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return terminalerrors.Pty(errStopTimeout) //nolint:wrapcheck // domain error constructor; the text is the user-facing message
	}
}
