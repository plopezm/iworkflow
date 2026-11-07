//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

// ownProcessGroup starts the command in its own process group and makes a cancellation kill the whole group,
// so a script that spawned children does not leave them running after a timeout.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
