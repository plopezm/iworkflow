//go:build unix

package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"iworkflow/internal/core/process"
)

// setProcessGroup makes the agent lead its own process group, so the group can be killed as a whole and
// the agent's children stop receiving the terminal's Ctrl+C.
func setProcessGroup(cmd *exec.Cmd) {
	process.SetProcessGroup(cmd)
}

// killAgent kills every member of the agent's process group, then the leader itself. Both are sent while
// the leader is unreaped (see supervise), so the pgid cannot yet name another group. Errors of the group
// kill and of the leader kill are both reported; a leader that already exited is not an error.
func killAgent(_ context.Context, proc *agentProcess) error {
	groupErr := process.KillGroup(proc.pgid)
	leaderErr := proc.cmd.Process.Kill()
	if errors.Is(leaderErr, os.ErrProcessDone) {
		leaderErr = nil
	}
	if err := errors.Join(groupErr, leaderErr); err != nil {
		return fmt.Errorf("kill agent group %d: %w", proc.pgid, err)
	}
	return nil
}

// sweepGroup kills what the agent left in its process group (the MCP servers it started) once the leader
// has been reaped. It signals only a group that still has members, checked with signal 0, so an empty group
// whose id may now be reused is never signalled. The check and the kill are not atomic: a short window
// remains, as in the Rust implementation.
func sweepGroup(pgid int) {
	if !process.GroupAlive(pgid) {
		return
	}
	if err := process.KillGroup(pgid); err != nil {
		slog.Warn("no se pudo barrer el grupo del agente", "pgid", pgid, "error", err)
	}
}
