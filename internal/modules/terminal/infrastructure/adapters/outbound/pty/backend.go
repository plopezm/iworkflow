package pty

import (
	"io"

	"iworkflow/internal/modules/terminal/domain/models"
)

// backend is the platform side of one live shell: the PTY (creack/pty on POSIX, ConPTY on Windows)
// together with the process attached to it. Reading yields the shell output; writing sends input.
type backend interface {
	io.ReadWriteCloser
	// setSize changes the window size of the terminal.
	setSize(size models.TerminalSize) error
	// pid returns the process id of the shell.
	pid() int
	// kill terminates the shell process. It fails only when the shell is already gone.
	kill() error
	// wait blocks until the shell has exited and releases its process resources.
	wait() error
}
