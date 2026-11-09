//go:build windows

package pty

import (
	"fmt"
	"os/exec"

	gopty "github.com/aymanbagabas/go-pty"

	"iworkflow/internal/modules/terminal/domain/models"
)

// conBackend is a shell attached to a Windows pseudo console (ConPTY).
type conBackend struct {
	console gopty.Pty
	cmd     *gopty.Cmd
}

// startBackend starts cmd attached to a new pseudo console of the given size. creack/pty has no
// Windows support, so ConPTY is driven through go-pty; only the resolved path, arguments,
// environment and directory of cmd are used.
func startBackend(cmd *exec.Cmd, size models.TerminalSize) (backend, error) {
	if cmd.Err != nil {
		return nil, fmt.Errorf("find the shell: %w", cmd.Err)
	}
	console, err := gopty.New()
	if err != nil {
		return nil, fmt.Errorf("create the pseudo console: %w", err)
	}
	if err := console.Resize(int(size.Cols), int(size.Rows)); err != nil {
		_ = console.Close() // the resize failure is the error to report
		return nil, fmt.Errorf("size the pseudo console: %w", err)
	}
	started := console.Command(cmd.Path, cmd.Args[1:]...)
	started.Env = cmd.Env
	started.Dir = cmd.Dir
	if err := started.Start(); err != nil {
		_ = console.Close() // the start failure is the error to report
		return nil, fmt.Errorf("start the shell: %w", err)
	}
	return &conBackend{console: console, cmd: started}, nil
}

func (b *conBackend) Read(p []byte) (int, error)  { return b.console.Read(p) }  //nolint:wrapcheck // plain io passthrough
func (b *conBackend) Write(p []byte) (int, error) { return b.console.Write(p) } //nolint:wrapcheck // plain io passthrough

func (b *conBackend) Close() error { return b.console.Close() } //nolint:wrapcheck // plain io passthrough

func (b *conBackend) setSize(size models.TerminalSize) error {
	return b.console.Resize(int(size.Cols), int(size.Rows)) //nolint:wrapcheck // the caller wraps it in a domain error
}

func (b *conBackend) pid() int { return b.cmd.Process.Pid }

func (b *conBackend) kill() error { return b.cmd.Process.Kill() } //nolint:wrapcheck // fails only when the shell is gone

func (b *conBackend) wait() error { return b.cmd.Wait() } //nolint:wrapcheck // the status is irrelevant to callers
