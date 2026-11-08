//go:build !windows

package process

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// ownProcessGroup starts the command in its own process group and makes a cancellation kill the whole group,
// so a script that spawned children does not leave them running after a timeout. A group that already exited
// answers ESRCH: there is nothing left to kill, so that is not an error.
func ownProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err != nil && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("kill the process group: %w", err)
		}
		return nil
	}
}
