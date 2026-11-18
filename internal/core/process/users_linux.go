//go:build linux

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

const procRoot string = "/proc"

// usersOf lists the processes whose working directory, executable, root or open files are inside one of the
// roots, reading /proc without cgo. Entries that vanish or belong to another user are skipped.
func usersOf(ctx context.Context, roots []string) ([]int, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", procRoot, err)
	}
	var pids []int
	for _, entry := range entries {
		if ctx.Err() != nil {
			return pids, fmt.Errorf("scan the processes: %w", ctx.Err())
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if usesRoots(pid, roots) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

// usesRoots reports whether the process has a path inside the roots.
func usesRoots(pid int, roots []string) bool {
	base := procRoot + "/" + strconv.Itoa(pid)
	for _, link := range []string{"/cwd", "/exe", "/root"} {
		if target, err := os.Readlink(base + link); err == nil && withinAny(roots, trimDeleted(target)) {
			return true
		}
	}
	fds, err := os.ReadDir(base + "/fd")
	if err != nil {
		return false
	}
	for _, fd := range fds {
		if target, err := os.Readlink(base + "/fd/" + fd.Name()); err == nil && withinAny(roots, trimDeleted(target)) {
			return true
		}
	}
	return false
}

// trimDeleted drops the " (deleted)" marker the kernel appends to the target of an unlinked file.
func trimDeleted(target string) string {
	return strings.TrimSuffix(target, " (deleted)")
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
