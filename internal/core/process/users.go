package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// errInvalidRoot rejects an empty root: matching everything would kill the whole machine.
var errInvalidRoot = errors.New("invalid root: it is empty")

// Reason says how a process uses the files of the root.
type Reason string

// The ways a process can use the files of a root.
const (
	// ReasonWorkingDirectory: its current directory is inside the root.
	ReasonWorkingDirectory Reason = "cwd"
	// ReasonExecutable: it runs a program that lives inside the root.
	ReasonExecutable Reason = "exe"
	// ReasonOpenFile: it holds a file of the root open.
	ReasonOpenFile Reason = "file"
)

// User is a process that uses files under a root. PID and Started together identify it: a pid that was reused by
// another process has a different Started, so a selection made earlier never reaches a stranger.
type User struct {
	PID  int
	Name string
	// Path is the file that ties the process to the root.
	Path   string
	Reason Reason
	// Started is an opaque token of the moment the process started; equal tokens mean the same process.
	Started string
}

// key identifies the process across scans.
func (u User) key() string { return strconv.Itoa(u.PID) + "|" + u.Started }

// ListUsers lists the processes that use files under root: its working directory, its executable or (where the
// platform reports them) its open files are inside root. The caller and its parent are never listed. It kills
// nothing. An incomplete scan returns what it found together with the error.
func ListUsers(ctx context.Context, root string) ([]User, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errInvalidRoot
	}
	users, err := usersOf(ctx, rootsOf(root))
	users = withoutProtected(users, protectedPIDs())
	slices.SortFunc(users, func(a User, b User) int { return a.PID - b.PID })
	if err != nil {
		return users, fmt.Errorf("find the processes using %s: %w", root, err)
	}
	return users, nil
}

// KillUsers closes every process that ListUsers finds, with the same rules as Close.
func KillUsers(ctx context.Context, root string) (int, error) {
	users, listErr := ListUsers(ctx, root)
	if errors.Is(listErr, errInvalidRoot) {
		return 0, listErr
	}
	closed, closeErr := Close(ctx, root, users)
	return closed, errors.Join(listErr, closeErr)
}

// scanUsers is the scan Close relies on; a variable so tests can make it fail.
var scanUsers = usersOf

// identityFor returns the start token of a pid as usersOf reports it; a variable so tests can contradict it.
var identityFor = identityOf

// forceTimeout bounds the fresh scan that checks the polite-signalled processes before they are force-killed.
const forceTimeout = 5 * time.Second

// termGrace is how long a process gets to exit on its own after it was asked to; a variable so tests can shorten it.
var termGrace = 3 * time.Second

// pollEvery is how often the survivors are looked for during the grace.
const pollEvery time.Duration = 250 * time.Millisecond

// Close terminates the selected processes of root. A selected process is acted on only when a fresh scan still
// finds it using the root with the same identity (pid and start time): one that already exited, or whose pid was
// reused, is skipped. Each one is first asked to exit (SIGTERM on Unix) and gets termGrace to flush and close; the
// ones still there are force-killed. Windows has no polite signal: processes are terminated at once. The caller
// and its parent are never touched. It returns how many processes it signalled; failures other than "already gone"
// are joined in the error.
func Close(ctx context.Context, root string, selected []User) (int, error) {
	if strings.TrimSpace(root) == "" {
		return 0, errInvalidRoot
	}
	roots := rootsOf(root)
	protected := protectedPIDs()
	wanted := map[string]struct{}{}
	for _, user := range withoutProtected(selected, protected) {
		wanted[user.key()] = struct{}{}
	}
	current, scanErr := scanUsers(ctx, roots)
	touched := map[int]struct{}{}
	var errs []error
	polite := signalAll(onlyWanted(current, wanted), false, touched, &errs)
	if len(polite) > 0 {
		survivors, waitErr := waitForExit(ctx, roots, wanted)
		if waitErr != nil {
			errs = append(errs, waitErr)
			survivors, waitErr = unconfirmed(ctx, roots, polite, wanted)
			if waitErr != nil {
				errs = append(errs, waitErr)
			}
		}
		signalAll(survivors, true, touched, &errs)
	}
	if scanErr != nil {
		errs = append(errs, fmt.Errorf("find the processes using %s: %w", root, scanErr))
	}
	return len(touched), errors.Join(errs...)
}

// signalAll signals each process (force or polite), records it in touched and collects the failures other than "gone".
// It returns the processes it found to signal; a gone one is not among them.
func signalAll(users []User, force bool, touched map[int]struct{}, errs *[]error) []User {
	var signalled []User
	for _, user := range users {
		switch err := signalPID(user.PID, force); {
		case err == nil:
			touched[user.PID] = struct{}{}
			signalled = append(signalled, user)
		case !errors.Is(err, errGone):
			*errs = append(*errs, err)
		}
	}
	return signalled
}

// maxRescanFailures is how many rescans in a row may fail before the wait gives up; one transient failure must not
// cut the grace short and turn a clean exit into a force-kill.
const maxRescanFailures = 3

// waitForExit rescans until none of the wanted processes uses the roots or the grace ends, and returns the ones
// still using them. A failed rescan is retried until the grace ends; only maxRescanFailures failures in a row, or a
// context that ended, return an error (the last one). Then the returned list is not a complete answer and the caller
// must not trust it.
func waitForExit(ctx context.Context, roots []string, wanted map[string]struct{}) ([]User, error) {
	deadline := time.Now().Add(termGrace)
	failures := 0
	for {
		scanned, err := scanUsers(ctx, roots)
		var remaining []User
		if err != nil {
			failures++
			if failures >= maxRescanFailures {
				return nil, fmt.Errorf("recheck the processes using the roots: %w", err)
			}
		} else {
			failures = 0
			remaining = onlyWanted(scanned, wanted)
			if len(remaining) == 0 {
				return nil, nil
			}
		}
		if !time.Now().Before(deadline) {
			if err != nil {
				// The grace is over but the last rescan failed: nothing is confirmed, so the caller must check again.
				return nil, fmt.Errorf("recheck the processes using the roots: %w", err)
			}
			return remaining, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return remaining, fmt.Errorf("recheck the processes using the roots: %w", ctxErr)
		}
		select {
		case <-ctx.Done():
		case <-time.After(pollEvery):
		}
	}
}

// unconfirmed returns the polite-signalled processes that must be force-killed because their exit was not confirmed. A
// fresh scan, with its own short context, checks that each one still uses the roots with its identity, so a process
// that already exited is skipped. When that scan fails too, nothing can be confirmed by scanning, so each process is
// force-killed only if identityFor still reports its start token, equal to the one recorded at selection (see
// verifiedOf); the others are left running and reported.
func unconfirmed(ctx context.Context, roots []string, polite []User, wanted map[string]struct{}) ([]User, error) {
	fresh, cancel := context.WithTimeout(context.WithoutCancel(ctx), forceTimeout)
	defer cancel()
	scanned, err := scanUsers(fresh, roots)
	if err != nil {
		kept, identityErrs := verifiedOf(fresh, polite)
		return kept, errors.Join(append([]error{fmt.Errorf("check the processes before force-killing them: %w", err)}, identityErrs...)...)
	}
	return onlyWanted(scanned, wanted), nil
}

// verifiedOf keeps the polite-signalled processes whose identity is still confirmed by identityFor: a non-empty token
// equal to the Started recorded at selection. A pid that is gone, unreadable, or now belongs to another process is not
// verified; a process recorded without a start token can never be verified. Each unverified one is reported and left
// running.
func verifiedOf(ctx context.Context, polite []User) ([]User, []error) {
	var kept []User
	var errs []error
	for _, user := range polite {
		if user.Started != "" {
			if started, ok := identityFor(ctx, user.PID); ok && started != "" && started == user.Started {
				kept = append(kept, user)
				continue
			}
		}
		errs = append(errs, fmt.Errorf("could not verify the identity of process %d; left running", user.PID))
	}
	return kept, errs
}

// onlyWanted keeps the users whose identity is in wanted.
func onlyWanted(users []User, wanted map[string]struct{}) []User {
	kept := make([]User, 0, len(users))
	for _, user := range users {
		if _, ok := wanted[user.key()]; ok {
			kept = append(kept, user)
		}
	}
	return kept
}

// withoutProtected returns users minus the protected pids.
func withoutProtected(users []User, protected map[int]struct{}) []User {
	kept := make([]User, 0, len(users))
	for _, user := range users {
		if _, skip := protected[user.PID]; !skip {
			kept = append(kept, user)
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

// parseLsof reads the output of `lsof -nP -F pcfn` and returns one User per process that has at least one reported
// file, in order of appearance. Each process is described by the file that ties it best to the root: its working
// directory first, then its executable, then any other open file (see reasonRank). Name comes from the c field,
// Path and Reason from that file. Started is left empty: the caller fills it in.
func parseLsof(out string) []User {
	var users []User
	index := map[int]int{} // pid -> position in users
	pid := 0               // 0 while the current record is malformed or before any p field
	name := ""
	fd := ""
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			continue
		}
		value := line[1:]
		switch line[0] {
		case 'p':
			pid, name, fd = 0, "", ""
			if parsed, err := strconv.Atoi(value); err == nil {
				pid = parsed
			}
		case 'c':
			name = value
			if i, known := index[pid]; known {
				users[i].Name = name
			}
		case 'f':
			fd = value
		case 'n':
			if pid == 0 {
				continue
			}
			reason := reasonOf(fd)
			i, known := index[pid]
			if !known {
				index[pid] = len(users)
				users = append(users, User{PID: pid, Name: name, Path: value, Reason: reason})
				continue
			}
			if reasonRank(reason) < reasonRank(users[i].Reason) {
				users[i].Path, users[i].Reason = value, reason
			}
		}
	}
	return users
}

// reasonOf maps an lsof file descriptor name to the reason the process uses the file: the working directory and
// the executable text are named by lsof ("cwd", "txt"); every other descriptor is an open file.
func reasonOf(fd string) Reason {
	switch fd {
	case "cwd":
		return ReasonWorkingDirectory
	case "txt":
		return ReasonExecutable
	default:
		return ReasonOpenFile
	}
}

// reasonRank orders the reasons from the strongest tie to the root (lowest) to the weakest.
func reasonRank(reason Reason) int {
	switch reason {
	case ReasonWorkingDirectory:
		return 0
	case ReasonExecutable:
		return 1
	default:
		return 2
	}
}
