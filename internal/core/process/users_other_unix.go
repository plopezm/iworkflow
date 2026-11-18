//go:build unix && !linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
)

// usersOf lists the processes with a file inside one of the roots (working directory, executable and open
// files) through lsof, which every macOS and BSD ships. The scan is bounded by ctx. lsof exits 1 when nothing
// matches, which is not an error.
func usersOf(ctx context.Context, roots []string) ([]int, error) {
	seen := map[int]struct{}{}
	var pids []int
	for _, root := range roots {
		out, err := exec.CommandContext(ctx, "lsof", "-nP", "-F", "pn", "+D", root).Output()
		var exit *exec.ExitError
		if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) && len(out) == 0 {
			return pids, fmt.Errorf("run lsof: %w", err)
		}
		for _, pid := range parseLsof(string(out)) {
			if _, dup := seen[pid]; !dup {
				seen[pid] = struct{}{}
				pids = append(pids, pid)
			}
		}
	}
	return pids, nil
}

// killPID sends SIGKILL to pid.
func killPID(pid int) error {
	err := syscall.Kill(pid, syscall.SIGKILL)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.ESRCH):
		return errGone
	default:
		return fmt.Errorf("kill process %d: %w", pid, err)
	}
}
