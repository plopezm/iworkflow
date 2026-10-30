//go:build unix

package git

import (
	"os/exec"

	"iworkflow/internal/core/process"
)

// killProcessTree makes cancellation kill git together with the children it started: git runs in its
// own process group, and the whole group receives the kill.
func killProcessTree(cmd *exec.Cmd) {
	process.SetProcessGroup(cmd)
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return process.KillGroup(cmd.Process.Pid)
	}
}
