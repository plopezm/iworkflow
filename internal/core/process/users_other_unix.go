//go:build unix && !linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// usersOf lists the processes with a file inside one of the roots (working directory, executable and open
// files) through lsof, which every macOS and BSD ships. The scan is bounded by ctx. lsof exits 1 when nothing
// matches, which is not an error. Each process is reported once, by its strongest tie to the root (see parseLsof).
// Started is the start time ps reports for the pid, so a reused pid is told apart from the process it replaced.
func usersOf(ctx context.Context, roots []string) ([]User, error) {
	index := map[int]int{}
	var users []User
	for _, root := range roots {
		out, err := exec.CommandContext(ctx, "lsof", "-nP", "-F", "pcfn", "+D", root).Output()
		// Exit 1 only says that some file could not be read or nothing matched. Any other failure, a timeout
		// that killed lsof half way included, means the list is incomplete, so it is reported with what was found.
		var exit *exec.ExitError
		benign := errors.As(err, &exit) && exit.ExitCode() == 1 && ctx.Err() == nil
		failed := err != nil && !benign
		for _, found := range parseLsof(string(out)) {
			i, known := index[found.PID]
			if !known {
				index[found.PID] = len(users)
				users = append(users, found)
				continue
			}
			if reasonRank(found.Reason) < reasonRank(users[i].Reason) {
				users[i].Path, users[i].Reason = found.Path, found.Reason
			}
		}
		if failed {
			return withStarted(ctx, users), fmt.Errorf("run lsof: %w", err)
		}
	}
	return withStarted(ctx, users), nil
}

// withStarted fills in the start token of every user.
func withStarted(ctx context.Context, users []User) []User {
	for i := range users {
		users[i].Started = startedOf(ctx, users[i].PID)
	}
	return users
}

// startedOf returns the start time ps reports for pid, trimmed. It is empty when ps fails (for instance because the
// process exited meanwhile); identity then rests on the pid alone for that process.
func startedOf(ctx context.Context, pid int) string {
	out, err := exec.CommandContext(ctx, "ps", "-o", "lstart=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
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
