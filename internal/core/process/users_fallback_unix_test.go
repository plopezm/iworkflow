//go:build unix

package process

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// installScanner replaces scanUsers for one test. Scans whose 1-based number fail returns an injected error; the others
// run the real scan, and post (when set) rewrites what the real scan returned. The returned counter is the number of
// scans that ran. The seam is restored in cleanup.
func installScanner(t *testing.T, fail func(call int) bool, post func([]User) []User) *int {
	t.Helper()
	previous := scanUsers
	calls := new(int)
	scanUsers = func(ctx context.Context, roots []string) ([]User, error) {
		*calls++
		if fail(*calls) {
			return nil, errors.New("injected scan failure")
		}
		users, err := previous(ctx, roots)
		if post != nil {
			users = post(users)
		}
		return users, err
	}
	t.Cleanup(func() { scanUsers = previous })
	return calls
}

// installIdentity replaces identityFor for one test and restores it in cleanup.
func installIdentity(t *testing.T, fn func(context.Context, int) (string, bool)) {
	t.Helper()
	previous := identityFor
	identityFor = fn
	t.Cleanup(func() { identityFor = previous })
}

// listedPID lists the users of root and keeps only the one of pid, so the shell's own short-lived children, which
// also work inside the root, are not part of the test.
func listedPID(t *testing.T, root string, pid int) []User {
	t.Helper()
	users, err := ListUsers(context.Background(), root)
	require.NoError(t, err)
	var kept []User
	for _, user := range users {
		if user.PID == pid {
			kept = append(kept, user)
		}
	}
	require.Len(t, kept, 1)
	return kept
}

// endedBy waits for c and returns the signal that ended it, or 0 when it exited on its own.
func endedBy(t *testing.T, c *child) syscall.Signal {
	t.Helper()
	require.True(t, c.exited(5*time.Second), "the process is still alive")
	status, ok := c.cmd.ProcessState.Sys().(syscall.WaitStatus)
	require.True(t, ok)
	if status.Signaled() {
		return status.Signal()
	}
	return 0
}

// stubbornShell starts a shell that ignores SIGTERM and gives it time to install the trap.
func stubbornShell(t *testing.T, root string) *child {
	t.Helper()
	stubborn := start(t, root, "sh", "-c", `trap "" TERM; while :; do sleep 0.1; done`)
	time.Sleep(200 * time.Millisecond)
	return stubborn
}

// A rescan that fails once must not end the grace: the process that exits on SIGTERM is never force-killed.
func TestCloseDoesNotForceKillAProcessWhenOneRescanFails(t *testing.T) {
	previous := termGrace
	termGrace = 3 * time.Second
	t.Cleanup(func() { termGrace = previous })
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "graceful")
	polite := start(t, root, "sh", "-c", `trap 'touch "$0"; sleep 0.3; exit 0' TERM; while :; do sleep 0.1; done`, marker)
	time.Sleep(200 * time.Millisecond)
	users := listedPID(t, root, polite.cmd.Process.Pid)
	installScanner(t, func(call int) bool { return call == 2 }, nil)

	closed, err := Close(context.Background(), root, users)

	require.NoError(t, err, "one failed rescan must not end the grace")
	assert.Equal(t, 1, closed)
	assert.True(t, polite.exited(5*time.Second))
	assert.FileExists(t, marker, "the process got to run its own shutdown")
	assert.Equal(t, 0, polite.cmd.ProcessState.ExitCode(), "the process must exit on its own, not be killed")
}

// Three consecutive failed rescans end the wait with an error; the fallback then force-kills the survivor, which
// is confirmed by the fresh scan that comes after them.
func TestCloseForceKillsAfterThreeConsecutiveRescanFailures(t *testing.T) {
	previous := termGrace
	termGrace = 10 * time.Second
	t.Cleanup(func() { termGrace = previous })
	root := t.TempDir()
	stubborn := stubbornShell(t, root)
	users := listedPID(t, root, stubborn.cmd.Process.Pid)
	calls := installScanner(t, func(call int) bool { return call >= 2 && call <= 4 }, nil)

	closed, err := Close(context.Background(), root, users)

	require.Error(t, err)
	assert.ErrorContains(t, err, "recheck the processes using the roots")
	assert.Equal(t, 1, closed)
	assert.Equal(t, 5, *calls, "the initial scan, three rescans, then the fresh check")
	assert.Equal(t, syscall.SIGKILL, endedBy(t, stubborn))
}

// Double failure with a matching identity: the survivor is verified by its real start token and force-killed.
func TestCloseForceKillsASurvivorWhoseIdentityMatchesWhenTheScansFail(t *testing.T) {
	previous := termGrace
	termGrace = 10 * time.Second
	t.Cleanup(func() { termGrace = previous })
	root := t.TempDir()
	stubborn := stubbornShell(t, root)
	users := listedPID(t, root, stubborn.cmd.Process.Pid)
	installScanner(t, func(call int) bool { return call >= 2 }, nil)

	closed, err := Close(context.Background(), root, users)

	require.Error(t, err, "the scan failures are still reported")
	assert.Equal(t, 1, closed)
	assert.Equal(t, syscall.SIGKILL, endedBy(t, stubborn), "the identity matched, so the survivor is force-killed")
}

// Double failure with a different identity: the pid now belongs to another process, so it is not killed and the
// error names it.
func TestCloseLeavesAProcessRunningWhoseIdentityDiffersWhenTheScansFail(t *testing.T) {
	previous := termGrace
	termGrace = 10 * time.Second
	t.Cleanup(func() { termGrace = previous })
	root := t.TempDir()
	stubborn := stubbornShell(t, root)
	pid := stubborn.cmd.Process.Pid
	users := listedPID(t, root, pid)
	installScanner(t, func(call int) bool { return call >= 2 }, nil)
	installIdentity(t, func(context.Context, int) (string, bool) { return "another-start", true })

	_, err := Close(context.Background(), root, users)

	require.Error(t, err)
	assert.ErrorContains(t, err, fmt.Sprintf("could not verify the identity of process %d; left running", pid))
	assert.True(t, stubborn.alive(), "a pid whose start token differs is another process and must not be killed")
}

// Double failure with an empty start token: the identity can never be verified, so the process is not killed.
func TestCloseNeverForceKillsAProcessWithoutAStartToken(t *testing.T) {
	previous := termGrace
	termGrace = 10 * time.Second
	t.Cleanup(func() { termGrace = previous })
	root := t.TempDir()
	stubborn := stubbornShell(t, root)
	pid := stubborn.cmd.Process.Pid
	users := listedPID(t, root, pid)
	users[0].Started = ""
	clearStart := func(found []User) []User {
		for i := range found {
			found[i].Started = ""
		}
		return found
	}
	installScanner(t, func(call int) bool { return call >= 2 }, clearStart)

	_, err := Close(context.Background(), root, users)

	require.Error(t, err)
	assert.ErrorContains(t, err, fmt.Sprintf("could not verify the identity of process %d; left running", pid))
	assert.True(t, stubborn.alive(), "a process without a start token cannot be verified and must not be killed")
}

// installExpiringFreshScan replaces scanUsers for one test: the first scan (which lists the users) runs for real; a
// scan with a deadline (the fresh check of Close) waits for that deadline and fails; any other scan fails at once, so
// every rescan of the grace fails. The seam is restored in cleanup.
func installExpiringFreshScan(t *testing.T) {
	t.Helper()
	previous := scanUsers
	calls := 0
	scanUsers = func(ctx context.Context, roots []string) ([]User, error) {
		calls++
		if calls == 1 {
			return previous(ctx, roots)
		}
		if _, bounded := ctx.Deadline(); bounded {
			<-ctx.Done()
			return nil, fmt.Errorf("injected fresh scan: %w", ctx.Err())
		}
		return nil, errors.New("injected rescan failure")
	}
	t.Cleanup(func() { scanUsers = previous })
}

// The fresh scan consumes its whole deadline and fails. The identity check must still get a live context of its own:
// a TERM-ignoring survivor whose start token still matches is force-killed, not left running.
func TestCloseForceKillsASurvivorWhenTheFreshScanExpiresBeforeTheIdentityCheck(t *testing.T) {
	previous := forceTimeout
	forceTimeout = 300 * time.Millisecond
	t.Cleanup(func() { forceTimeout = previous })
	root := t.TempDir()
	stubborn := stubbornShell(t, root)
	users := listedPID(t, root, stubborn.cmd.Process.Pid)
	installExpiringFreshScan(t)
	installIdentity(t, func(ctx context.Context, pid int) (string, bool) {
		if ctx.Err() != nil || pid != stubborn.cmd.Process.Pid {
			return "", false
		}
		return users[0].Started, true
	})

	closed, err := Close(context.Background(), root, users)

	require.Error(t, err, "the failed fresh scan is still reported")
	assert.ErrorContains(t, err, "check the processes before force-killing them")
	assert.Equal(t, 1, closed)
	assert.Equal(t, syscall.SIGKILL, endedBy(t, stubborn), "the identity matched, so the survivor is force-killed")
}
