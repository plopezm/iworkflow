package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// errInvalidRoot rejects an empty root: matching everything would kill the whole machine.
var errInvalidRoot = errors.New("invalid root: it is empty")

// KillUsers force-terminates every process that uses files under root: its working directory, its executable
// or (where the platform reports them) its open files are inside root. The caller and its parent are never
// touched. It returns how many processes it signalled. A process that is already gone, or that cannot be
// inspected or signalled (another user's), is skipped; the errors that remain are joined in the result next to
// the count, so the caller decides whether they matter.
func KillUsers(ctx context.Context, root string) (int, error) {
	if strings.TrimSpace(root) == "" {
		return 0, errInvalidRoot
	}
	roots := rootsOf(root)
	pids, err := usersOf(ctx, roots)
	protected := protectedPIDs()
	killed := 0
	var killErrs []error
	for _, pid := range pids {
		if _, skip := protected[pid]; skip {
			continue
		}
		switch killErr := killPID(pid); {
		case killErr == nil:
			killed++
		case !errors.Is(killErr, errGone):
			killErrs = append(killErrs, killErr)
		}
	}
	if err != nil {
		err = fmt.Errorf("find the processes using %s: %w", root, err)
	}
	return killed, errors.Join(append(killErrs, err)...)
}

// rootsOf returns the spellings of root a process may report: as given and with its symbolic links resolved.
func rootsOf(root string) []string {
	cleaned := filepath.Clean(root)
	roots := []string{cleaned}
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil && resolved != cleaned {
		roots = append(roots, resolved)
	}
	return roots
}

// within reports whether path is root or lies below it. A sibling that merely shares the prefix does not match.
func within(root string, path string) bool {
	if path == "" {
		return false
	}
	path = filepath.Clean(path)
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// withinAny reports whether path is inside any of the roots.
func withinAny(roots []string, path string) bool {
	for _, root := range roots {
		if within(root, path) {
			return true
		}
	}
	return false
}

// protectedPIDs are the processes that must survive: the caller and its parent.
func protectedPIDs() map[int]struct{} {
	return map[int]struct{}{os.Getpid(): {}, os.Getppid(): {}}
}

// errGone says the process ended before it could be signalled, which is the state we want.
var errGone = errors.New("process is gone")

// parseLsof reads the output of `lsof -F pn` and returns each pid that has at least one reported file, once and
// in order of appearance.
func parseLsof(out string) []int {
	var pids []int
	seen := map[int]struct{}{}
	current := 0
	for line := range strings.SplitSeq(out, "\n") {
		switch {
		case strings.HasPrefix(line, "p"):
			pid, err := strconv.Atoi(line[1:])
			if err != nil {
				current = 0
				continue
			}
			current = pid
		case strings.HasPrefix(line, "n") && current != 0:
			if _, dup := seen[current]; !dup {
				seen[current] = struct{}{}
				pids = append(pids, current)
			}
		}
	}
	return pids
}
