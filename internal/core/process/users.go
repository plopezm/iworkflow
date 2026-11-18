package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// errInvalidRoot rejects an empty root: matching everything would kill the whole machine.
var errInvalidRoot = errors.New("invalid root: it is empty")

// termGrace is how long a process gets to exit on its own after it was asked to; a variable so tests can shorten it.
var termGrace = 3 * time.Second

// pollEvery is how often the survivors are looked for during the grace.
const pollEvery time.Duration = 250 * time.Millisecond

// KillUsers terminates every process that uses files under root: its working directory, its executable or
// (where the platform reports them) its open files are inside root. Each one is first asked to exit (SIGTERM on
// Unix) and gets termGrace to flush and close; the ones still using the root after that are scanned again, so a
// reused pid is never hit, and force-killed. Windows has no polite signal: processes are terminated at once.
// The caller and its parent are never touched. It returns how many distinct processes it signalled. A process
// that is already gone is skipped; every other failure, including an incomplete scan, is joined in the error
// next to the count, so the caller decides whether it matters.
func KillUsers(ctx context.Context, root string) (int, error) {
	if strings.TrimSpace(root) == "" {
		return 0, errInvalidRoot
	}
	roots := rootsOf(root)
	protected := protectedPIDs()
	pids, scanErr := usersOf(ctx, roots)
	touched := map[int]struct{}{}
	var errs []error
	signalAll(without(pids, protected), false, touched, &errs)
	if len(touched) > 0 {
		signalAll(waitForExit(ctx, roots, protected), true, touched, &errs)
	}
	if scanErr != nil {
		errs = append(errs, fmt.Errorf("find the processes using %s: %w", root, scanErr))
	}
	return len(touched), errors.Join(errs...)
}

// signalAll signals each pid (force or polite), records it in touched and collects the failures other than "gone".
func signalAll(pids []int, force bool, touched map[int]struct{}, errs *[]error) {
	for _, pid := range pids {
		switch err := signalPID(pid, force); {
		case err == nil:
			touched[pid] = struct{}{}
		case !errors.Is(err, errGone):
			*errs = append(*errs, err)
		}
	}
}

// waitForExit rescans until nothing uses the roots or the grace ends, and returns the processes still using them.
func waitForExit(ctx context.Context, roots []string, protected map[int]struct{}) []int {
	deadline := time.Now().Add(termGrace)
	for {
		remaining, _ := usersOf(ctx, roots) // a failed rescan only means fewer survivors are seen; KillUsers reports the first scan
		remaining = without(remaining, protected)
		if len(remaining) == 0 || ctx.Err() != nil || !time.Now().Before(deadline) {
			return remaining
		}
		select {
		case <-ctx.Done():
		case <-time.After(pollEvery):
		}
	}
}

// without returns pids minus the protected ones.
func without(pids []int, protected map[int]struct{}) []int {
	kept := make([]int, 0, len(pids))
	for _, pid := range pids {
		if _, skip := protected[pid]; !skip {
			kept = append(kept, pid)
		}
	}
	return kept
}

// rootsOf returns the spellings of root a process may report: as given and with its symbolic links resolved.
func rootsOf(root string) []string {
	cleaned := filepath.Clean(root)
	roots := append([]string{cleaned}, extraSpellings(cleaned)...)
	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil && resolved != cleaned {
		roots = append(roots, resolved)
	}
	return roots
}

// within reports whether path is root or lies below it. A sibling that merely shares the prefix does not match.
func within(root string, path string) bool { return withinFold(root, path, foldsCase) }

// foldsCase says the default file systems of the platform ignore the case of names.
var foldsCase = runtime.GOOS == "windows" || runtime.GOOS == "darwin"

// withinFold is within with an explicit case rule.
func withinFold(root string, path string, fold bool) bool {
	if path == "" {
		return false
	}
	path = filepath.Clean(path)
	if fold {
		root, path = strings.ToLower(root), strings.ToLower(path)
	}
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
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
