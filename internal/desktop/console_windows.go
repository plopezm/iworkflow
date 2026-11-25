//go:build windows

package desktop

import (
	"os/exec"
	"syscall"
)

// createNoWindow stops Windows from opening a console window for a console-subsystem child process.
const createNoWindow = 0x08000000

// hideConsole makes the server child run without a visible console. Its logs are read through pipes, and the
// shells it starts inherit the hidden console instead of opening a terminal of their own.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
