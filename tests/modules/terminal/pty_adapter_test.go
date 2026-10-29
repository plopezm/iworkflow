//go:build unix

// Integration tests of the PTY adapter against real shells (sh). Each test owns its adapter and
// shuts it down, so a failing test cannot leave a process behind.
package terminal_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	terminalerrors "iworkflow/internal/modules/terminal/domain/errors"
	"iworkflow/internal/modules/terminal/domain/models"
	"iworkflow/internal/modules/terminal/infrastructure/adapters/outbound/pty"
)

const (
	waitTimeout  time.Duration = 10 * time.Second
	pollInterval time.Duration = 20 * time.Millisecond
	// termValue is what the shell must advertise.
	termValue string = "xterm-256color"
)

var testSize = models.TerminalSize{Cols: 80, Rows: 24}

// newAdapter builds a PTY adapter that runs sh and shuts it down when the test ends.
func newAdapter(t *testing.T) *pty.Adapter {
	t.Helper()
	adapter := pty.NewAdapter("sh")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
		defer cancel()
		require.NoError(t, adapter.Shutdown(ctx))
	})
	return adapter
}

// spawnSh starts a session in dir and returns its id.
func spawnSh(t *testing.T, adapter *pty.Adapter, dir string, name string) models.TerminalSessionID {
	t.Helper()
	session, err := adapter.Spawn(context.Background(), uuid.New(), name, dir, testSize)
	require.NoError(t, err)
	return session.ID
}

// attach returns the live output channel of a session.
func attach(t *testing.T, adapter *pty.Adapter, id models.TerminalSessionID) (models.TerminalAttachment, <-chan []byte) {
	t.Helper()
	attachment, err := adapter.Attach(context.Background(), id)
	require.NoError(t, err)
	return attachment, attachment.Output
}

// readUntil collects output until needle appears and fails the test on timeout.
func readUntil(t *testing.T, output <-chan []byte, needle string) string {
	t.Helper()
	var collected strings.Builder
	timeout := time.After(waitTimeout)
	for {
		select {
		case chunk, open := <-output:
			require.True(t, open, "the output closed before %q appeared; got %q", needle, collected.String())
			collected.Write(chunk)
			if strings.Contains(collected.String(), needle) {
				return collected.String()
			}
		case <-timeout:
			require.FailNowf(t, "timeout", "%q did not appear; got %q", needle, collected.String())
		}
	}
}

// readPID waits for tag followed by digits and a newline, and returns the number. The echo of the
// typed command has "$" after the tag, so it never matches; the newline stops partial reads.
func readPID(t *testing.T, output <-chan []byte, tag string) int {
	t.Helper()
	pattern := regexp.MustCompile(regexp.QuoteMeta(tag) + `(\d+)\r?\n`)
	var collected strings.Builder
	timeout := time.After(waitTimeout)
	for {
		select {
		case chunk, open := <-output:
			require.True(t, open, "no pid for %q; got %q", tag, collected.String())
			collected.Write(chunk)
			if match := pattern.FindStringSubmatch(collected.String()); match != nil {
				pid, err := strconv.Atoi(match[1])
				require.NoError(t, err)
				return pid
			}
		case <-timeout:
			require.FailNowf(t, "timeout", "no pid for %q; got %q", tag, collected.String())
		}
	}
}

// isAlive reports whether a process exists and is not a zombie.
func isAlive(pid int) bool {
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	closing := strings.LastIndexByte(string(stat), ')')
	return closing >= 0 && !strings.HasPrefix(strings.TrimSpace(string(stat[closing+1:])), "Z")
}

// assertDies waits until the process is gone or a zombie.
func assertDies(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for isAlive(pid) {
		if time.Now().After(deadline) {
			require.FailNowf(t, "leaked process", "process %d is still alive", pid)
		}
		time.Sleep(pollInterval)
	}
}

// waitUntilExited waits until the session reports the Exited status.
func waitUntilExited(t *testing.T, adapter *pty.Adapter, id models.TerminalSessionID) {
	t.Helper()
	deadline := time.Now().Add(waitTimeout)
	for {
		sessions, err := adapter.List(context.Background())
		require.NoError(t, err)
		for _, session := range sessions {
			if session.ID == id && session.Status == models.TerminalStatusExited {
				return
			}
		}
		if time.Now().After(deadline) {
			require.FailNow(t, "the session did not report Exited")
		}
		time.Sleep(pollInterval)
	}
}

func TestSpawnWriteReadAndCloseAShell(t *testing.T) {
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, output := attach(t, adapter, id)

	require.NoError(t, adapter.Write(context.Background(), id, []byte("echo hello_pty\n")))
	readUntil(t, output, "hello_pty")
	require.NoError(t, adapter.Resize(context.Background(), id, models.TerminalSize{Cols: 100, Rows: 30}))

	require.NoError(t, adapter.Close(context.Background(), id))
	require.ErrorIs(t, adapter.Write(context.Background(), id, []byte("x")), terminalerrors.ErrSessionNotFound)
}

func TestSpawnsInTheGivenCwd(t *testing.T) {
	dir := t.TempDir()
	expected, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, dir, "demo")
	_, output := attach(t, adapter, id)

	require.NoError(t, adapter.Write(context.Background(), id, []byte("pwd\n")))
	readUntil(t, output, expected)
	require.NoError(t, adapter.Close(context.Background(), id))
}

func TestShellKeepsSystemVariablesButNotThoseSetByTheParent(t *testing.T) {
	t.Setenv("IWF_TERMINAL_PARENT_PROBE", "set")
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, output := attach(t, adapter, id)

	require.NoError(t, adapter.Write(context.Background(), id,
		[]byte("echo parent=${IWF_TERMINAL_PARENT_PROBE-unset} home=${HOME:+set} term=$TERM\n")))
	seen := readUntil(t, output, "term="+termValue)
	require.Contains(t, seen, "parent=unset")
	require.Contains(t, seen, "home=set")

	require.NoError(t, adapter.Write(context.Background(), id, []byte("echo path=$PATH\n")))
	pathSeen := readUntil(t, output, "path=/")
	require.NotContains(t, pathSeen, "path="+os.Getenv("PATH")+"\n", "the shell must not inherit the parent PATH")
	require.NoError(t, adapter.Close(context.Background(), id))
}

func TestShellStartsAsALoginShell(t *testing.T) {
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, output := attach(t, adapter, id)

	require.NoError(t, adapter.Write(context.Background(), id, []byte("tr '\\0' ' ' < /proc/$$/cmdline | sed 's/^/args=/'\n")))
	seen := readUntil(t, output, " -l")
	require.Contains(t, seen, "args=")
	require.NoError(t, adapter.Close(context.Background(), id))
}

func TestUnknownSessionIsNotFound(t *testing.T) {
	adapter := pty.NewAdapter("sh")
	id := models.NewTerminalSessionID()

	require.ErrorIs(t, adapter.Close(context.Background(), id), terminalerrors.ErrSessionNotFound)
	_, err := adapter.Attach(context.Background(), id)
	require.ErrorIs(t, err, terminalerrors.ErrSessionNotFound)
}

func TestReattachReplaysEarlierOutputThenReceivesLiveOutput(t *testing.T) {
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, first := attach(t, adapter, id)
	require.NoError(t, adapter.Write(context.Background(), id, []byte("echo before_marker\n")))
	readUntil(t, first, "before_marker")

	reattached, live := attach(t, adapter, id)
	require.Contains(t, string(reattached.Scrollback), "before_marker")
	require.NoError(t, adapter.Write(context.Background(), id, []byte("echo after_marker\n")))
	readUntil(t, live, "after_marker")
	require.NoError(t, adapter.Close(context.Background(), id))
}

func TestTwoSubscribersReceiveTheSameLiveOutput(t *testing.T) {
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, first := attach(t, adapter, id)
	_, second := attach(t, adapter, id)

	require.NoError(t, adapter.Write(context.Background(), id, []byte("echo shared_marker\n")))
	readUntil(t, first, "shared_marker")
	readUntil(t, second, "shared_marker")
	require.NoError(t, adapter.Close(context.Background(), id))
}

func TestListReturnsSessionsWithMetadata(t *testing.T) {
	adapter := newAdapter(t)
	dir := t.TempDir()
	workspaceID := uuid.New()
	alpha, err := adapter.Spawn(context.Background(), workspaceID, "alpha", dir, testSize)
	require.NoError(t, err)
	betaID := spawnSh(t, adapter, dir, "beta")

	sessions, err := adapter.List(context.Background())
	require.NoError(t, err)
	require.Len(t, sessions, 2)
	var listedAlpha models.TerminalSession
	for _, session := range sessions {
		if session.ID == alpha.ID {
			listedAlpha = session
		}
	}
	require.Equal(t, workspaceID, listedAlpha.WorkspaceID)
	require.Equal(t, "alpha", listedAlpha.WorkspaceName)
	require.True(t, alpha.CreatedAt.Equal(listedAlpha.CreatedAt))
	require.Equal(t, models.TerminalStatusRunning, listedAlpha.Status)
	require.Contains(t, sessions, sessionNamed(sessions, betaID, "beta"))

	require.NoError(t, adapter.Close(context.Background(), alpha.ID))
	require.NoError(t, adapter.Close(context.Background(), betaID))
	remaining, err := adapter.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, remaining)
}

// sessionNamed returns the listed session with id and name, for assertions.
func sessionNamed(sessions []models.TerminalSession, id models.TerminalSessionID, name string) models.TerminalSession {
	for _, session := range sessions {
		if session.ID == id && session.WorkspaceName == name {
			return session
		}
	}
	return models.TerminalSession{}
}

func TestExitedSessionStaysListedUntilClose(t *testing.T) {
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, output := attach(t, adapter, id)
	require.NoError(t, adapter.Write(context.Background(), id, []byte("echo bye_marker\nexit\n")))
	waitUntilExited(t, adapter, id)

	sessions, err := adapter.List(context.Background())
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	attachment, exited := attach(t, adapter, id)
	require.Contains(t, string(attachment.Scrollback), "bye_marker")
	drainUntilClosed(t, exited)
	drainUntilClosed(t, output)

	require.Error(t, adapter.Write(context.Background(), id, []byte("x")))
	require.Error(t, adapter.Resize(context.Background(), id, testSize))

	require.NoError(t, adapter.Close(context.Background(), id))
	remaining, err := adapter.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, remaining)
	_, err = adapter.Attach(context.Background(), id)
	require.ErrorIs(t, err, terminalerrors.ErrSessionNotFound)
}

// drainUntilClosed reads until the channel closes; a timeout fails the test.
func drainUntilClosed(t *testing.T, output <-chan []byte) {
	t.Helper()
	timeout := time.After(waitTimeout)
	for {
		select {
		case _, open := <-output:
			if !open {
				return
			}
		case <-timeout:
			require.FailNow(t, "the live channel should close after the shell exits")
		}
	}
}

func TestScrollbackIsBounded(t *testing.T) {
	const limit int = 1024 * 1024
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	// This subscriber never reads: it falls behind and must not stall the reader.
	_, _ = attach(t, adapter, id)

	// About 1.2 MiB of output followed by a marker.
	command := "i=0; while [ $i -lt 16000 ]; do echo xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx; i=$((i+1)); done; echo done_$((1+1))_marker\n"
	require.NoError(t, adapter.Write(context.Background(), id, []byte(command)))

	deadline := time.Now().Add(waitTimeout)
	for {
		attachment, err := adapter.Attach(context.Background(), id)
		require.NoError(t, err)
		if strings.Contains(string(attachment.Scrollback), "done_2_marker") {
			require.LessOrEqual(t, len(attachment.Scrollback), limit)
			attachment.Detach()
			break
		}
		attachment.Detach()
		if time.Now().After(deadline) {
			require.FailNow(t, "the final marker did not reach the scrollback")
		}
		time.Sleep(pollInterval)
	}
	require.NoError(t, adapter.Close(context.Background(), id))
}

func TestCloseKillsForegroundBackgroundAndSetsidProcesses(t *testing.T) {
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, output := attach(t, adapter, id)

	// The escaped process gets its own session but keeps a live parent in the shell's tree, which is
	// how kill_descendants finds it. The foreground job goes last: it blocks the shell until it dies.
	require.NoError(t, adapter.Write(context.Background(), id, []byte("nohup sleep 300 >/dev/null 2>&1 & echo bgpid=$!\n")))
	background := readPID(t, output, "bgpid=")
	require.NoError(t, adapter.Write(context.Background(), id, []byte("sh -c 'setsid sleep 300 & echo sidpid=$!; exec sleep 300' &\n")))
	escaped := readPID(t, output, "sidpid=")
	require.NoError(t, adapter.Write(context.Background(), id, []byte("sh -c 'echo fgpid=$$; exec sleep 300'\n")))
	foreground := readPID(t, output, "fgpid=")
	require.True(t, isAlive(background) && isAlive(foreground) && isAlive(escaped))

	require.NoError(t, adapter.Close(context.Background(), id))

	assertDies(t, background)
	assertDies(t, foreground)
	assertDies(t, escaped)
}

func TestCloseReapsTheShell(t *testing.T) {
	adapter := newAdapter(t)
	id := spawnSh(t, adapter, t.TempDir(), "demo")
	_, output := attach(t, adapter, id)
	require.NoError(t, adapter.Write(context.Background(), id, []byte("echo shellpid=$$\n")))
	shell := readPID(t, output, "shellpid=")

	require.NoError(t, adapter.Close(context.Background(), id))

	// Close waits for the reap, so the shell must not be found at all, not even as a zombie.
	_, err := os.Stat("/proc/" + strconv.Itoa(shell))
	require.True(t, os.IsNotExist(err), "the shell %d was not reaped: %v", shell, err)
}

func TestShutdownKillsEverySessionAndRefusesNewOnes(t *testing.T) {
	adapter := pty.NewAdapter("sh")
	dir := t.TempDir()
	first := spawnSh(t, adapter, dir, "one")
	second := spawnSh(t, adapter, dir, "two")
	_, output := attach(t, adapter, first)
	require.NoError(t, adapter.Write(context.Background(), first, []byte("nohup sleep 300 >/dev/null 2>&1 & echo bgpid=$!\n")))
	background := readPID(t, output, "bgpid=")

	ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
	defer cancel()
	require.NoError(t, adapter.Shutdown(ctx))

	assertDies(t, background)
	sessions, err := adapter.List(context.Background())
	require.NoError(t, err)
	require.Empty(t, sessions)
	_, err = adapter.Attach(context.Background(), second)
	require.ErrorIs(t, err, terminalerrors.ErrSessionNotFound)
	_, err = adapter.Spawn(context.Background(), uuid.New(), "late", dir, testSize)
	require.ErrorIs(t, err, terminalerrors.ErrPty)
}
