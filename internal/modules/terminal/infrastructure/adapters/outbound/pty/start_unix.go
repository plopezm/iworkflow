//go:build !windows

package pty

import (
	"fmt"
	"os"
	"os/exec"

	creackpty "github.com/creack/pty"

	"iworkflow/internal/modules/terminal/domain/models"
)

// unixBackend is a shell attached to a creack/pty master.
type unixBackend struct {
	cmd    *exec.Cmd
	master *os.File
}

// startBackend starts cmd attached to a new PTY of the given size.
func startBackend(cmd *exec.Cmd, size models.TerminalSize) (backend, error) {
	master, err := creackpty.StartWithSize(cmd, toWinsize(size))
	if err != nil {
		return nil, fmt.Errorf("start the pty: %w", err)
	}
	return &unixBackend{cmd: cmd, master: master}, nil
}

// toWinsize converts the domain size into the PTY window size. Pixel dimensions stay zero, as in the Rust version.
func toWinsize(size models.TerminalSize) *creackpty.Winsize {
	return &creackpty.Winsize{Rows: size.Rows, Cols: size.Cols}
}

func (b *unixBackend) Read(p []byte) (int, error)  { return b.master.Read(p) }  //nolint:wrapcheck // plain io passthrough
func (b *unixBackend) Write(p []byte) (int, error) { return b.master.Write(p) } //nolint:wrapcheck // plain io passthrough

// Close closes the master, which ends the reader with EIO and releases the terminal.
func (b *unixBackend) Close() error { return b.master.Close() } //nolint:wrapcheck // plain io passthrough

func (b *unixBackend) setSize(size models.TerminalSize) error {
	return creackpty.Setsize(b.master, toWinsize(size)) //nolint:wrapcheck // the caller wraps it in a domain error
}

func (b *unixBackend) pid() int { return b.cmd.Process.Pid }

func (b *unixBackend) kill() error { return b.cmd.Process.Kill() } //nolint:wrapcheck // fails only when the shell is gone

func (b *unixBackend) wait() error { return b.cmd.Wait() } //nolint:wrapcheck // the status is irrelevant to callers
