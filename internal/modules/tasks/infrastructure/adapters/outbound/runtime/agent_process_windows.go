//go:build windows

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"iworkflow/internal/core/process"
)

// setProcessGroup is a no-op on Windows: there is no exec there, and the tree is killed with taskkill.
func setProcessGroup(*exec.Cmd) {}

// killAgent kills the agent's process tree while it is not reaped, so the pid cannot name a recycled
// process. Like the Rust version, a failing taskkill is ignored: the tree may have exited already.
func killAgent(ctx context.Context, proc *agentProcess) error {
	if !proc.reaped.Load() {

		_ = process.KillTreeContext(ctx, uint32(proc.cmd.Process.Pid))
	}
	if err := proc.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("kill agent %d: %w", proc.cmd.Process.Pid, err)
	}
	return nil
}

// sweepGroup is a no-op on Windows, where killAgent already killed the whole tree.
func sweepGroup(int) {}
