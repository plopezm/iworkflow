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
func usersOf(ctx context.Context, roots []string) ([]User, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", procRoot, err)
	}
	var users []User
	for _, entry := range entries {
		if ctx.Err() != nil {
			return users, fmt.Errorf("scan the processes: %w", ctx.Err())
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if user, ok := userOf(pid, roots); ok {
			users = append(users, user)
		}
	}
	return users, nil
}

// userOf describes the process when it has a path inside the roots.
func userOf(pid int, roots []string) (User, bool) {
	base := procRoot + "/" + strconv.Itoa(pid)
	links := []struct {
		name   string
		reason Reason
	}{{"/cwd", ReasonWorkingDirectory}, {"/exe", ReasonExecutable}, {"/root", ReasonWorkingDirectory}}
	for _, link := range links {
		if target, err := os.Readlink(base + link.name); err == nil && withinAny(roots, trimDeleted(target)) {
			return describe(pid, trimDeleted(target), link.reason), true
		}
	}
	fds, err := os.ReadDir(base + "/fd")
	if err != nil {
		return User{}, false
	}
	for _, fd := range fds {
		if target, err := os.Readlink(base + "/fd/" + fd.Name()); err == nil && withinAny(roots, trimDeleted(target)) {
			return describe(pid, trimDeleted(target), ReasonOpenFile), true
		}
	}
	return User{}, false
}

// describe fills the name and the start time of the process from /proc.
func describe(pid int, path string, reason Reason) User {
	base := procRoot + "/" + strconv.Itoa(pid)
	user := User{PID: pid, Path: path, Reason: reason}
	if comm, err := os.ReadFile(base + "/comm"); err == nil {
		user.Name = strings.TrimSpace(string(comm))
	}
	if stat, err := os.ReadFile(base + "/stat"); err == nil {
		user.Started = startTime(string(stat))
	}
	return user
}

// startTime extracts field 22 (starttime, in clock ticks since boot) of a /proc/<pid>/stat line. comm may hold
// spaces and parentheses, so the line is split after the last ')'.
func startTime(stat string) string {
	closing := strings.LastIndexByte(stat, ')')
	if closing < 0 {
		return ""
	}
	fields := strings.Fields(stat[closing+1:])
	// fields[0] is the state, field 3 of the line; starttime is field 22, so index 19.
	if len(fields) < 20 {
		return ""
	}
	return fields[19]
}

// identityOf returns the start token of pid as usersOf reports it, and false when the process is gone or unreadable.
func identityOf(_ context.Context, pid int) (string, bool) {
	stat, err := os.ReadFile(procRoot + "/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return "", false
	}
	started := startTime(string(stat))
	return started, started != ""
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
