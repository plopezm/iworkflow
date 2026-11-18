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
		// Exit 1 only says that some file could not be read or nothing matched. Any other failure, a timeout
		// that killed lsof half way included, means the list is incomplete, so it is reported with what was found.
		var exit *exec.ExitError
		benign := errors.As(err, &exit) && exit.ExitCode() == 1 && ctx.Err() == nil
		failed := err != nil && !benign
		for _, pid := range parseLsof(string(out)) {
			if _, dup := seen[pid]; !dup {
				seen[pid] = struct{}{}
				pids = append(pids, pid)
			}
		}
		if failed {
			return pids, fmt.Errorf("run lsof: %w", err)
		}
	}
	return pids, nil
}

// signalPID sends SIGKILL to pid when force is set, SIGTERM otherwise.
func signalPID(pid int, force bool) error {
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	err := syscall.Kill(pid, signal)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, syscall.ESRCH):
		return errGone
	default:
		return fmt.Errorf("kill process %d: %w", pid, err)
	}
}
