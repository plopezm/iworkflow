//go:build unix

package process

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// child is a started process with a single owner of Wait, so a test and its cleanup never race on it.
type child struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func start(t *testing.T, dir string, name string, args ...string) *child {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	require.NoError(t, cmd.Start())
	c := &child{cmd: cmd, done: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(c.done) }()
	t.Cleanup(func() { _ = cmd.Process.Kill(); <-c.done })
	return c
}

func (c *child) exited(within time.Duration) bool {
	select {
	case <-c.done:
		return true
	case <-time.After(within):
		return false
	}
}

func (c *child) alive() bool {
	select {
	case <-c.done:
		return false
	default:
		return true
	}
}

func TestKillUsersKillsTheProcessesWorkingInsideTheRootOnly(t *testing.T) {
	root := t.TempDir()
	inside := start(t, root, "sleep", "60")
	outside := start(t, t.TempDir(), "sleep", "60")

	killed, err := KillUsers(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, 1, killed)
	assert.True(t, inside.exited(5*time.Second), "the process working inside the root is still alive")
	assert.True(t, outside.alive(), "the process outside the root is untouched")
}

func TestKillUsersFindsAProcessHoldingAnOpenFileOfTheRoot(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "open.log")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	holder := start(t, t.TempDir(), "sh", "-c", `exec sleep 60 < "$0"`, file)

	killed, err := KillUsers(context.Background(), root)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, killed, 1)
	assert.True(t, holder.exited(5*time.Second), "the process with an open file of the root is still alive")
}

func TestKillUsersFindsAProcessRunningAnExecutableFromTheRoot(t *testing.T) {
	root := t.TempDir()
	sleepPath, err := exec.LookPath("sleep")
	require.NoError(t, err)
	copied := filepath.Join(root, "tool")
	source, err := os.Open(sleepPath)
	require.NoError(t, err)
	defer func() { _ = source.Close() }()
	target, err := os.OpenFile(copied, os.O_CREATE|os.O_WRONLY, 0o700)
	require.NoError(t, err)
	_, err = io.Copy(target, source)
	require.NoError(t, err)
	require.NoError(t, target.Close())
	runner := start(t, t.TempDir(), copied, "60")

	killed, err := KillUsers(context.Background(), root)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, killed, 1)
	assert.True(t, runner.exited(5*time.Second), "the process running an executable of the root is still alive")
}

func TestKillUsersAsksPoliteFirstThenForcesAProcessThatIgnoresIt(t *testing.T) {
	previous := termGrace
	termGrace = 400 * time.Millisecond
	t.Cleanup(func() { termGrace = previous })
	root := t.TempDir()
	stubborn := start(t, root, "sh", "-c", `trap "" TERM; while :; do sleep 0.1; done`)
	time.Sleep(200 * time.Millisecond) // let the trap be installed

	killed, err := KillUsers(context.Background(), root)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, killed, 1)
	assert.True(t, stubborn.exited(5*time.Second), "the process that ignores SIGTERM must be killed after the grace")
}

func TestKillUsersLetsAProcessExitOnItsOwnAfterTheTermSignal(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(t.TempDir(), "graceful")
	polite := start(t, root, "sh", "-c", `trap 'touch "$0"; exit 0' TERM; while :; do sleep 0.1; done`, marker)
	time.Sleep(200 * time.Millisecond)

	_, err := KillUsers(context.Background(), root)

	require.NoError(t, err)
	assert.True(t, polite.exited(5*time.Second))
	assert.FileExists(t, marker, "the process got the chance to run its own shutdown")
}

func TestKillUsersNeverKillsTheCaller(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(root))
	t.Cleanup(func() { _ = os.Chdir(previous) })

	_, err = KillUsers(context.Background(), root)

	require.NoError(t, err, "the caller, whose working directory is the root, is still running")
}

func TestKillUsersReportsACancelledScan(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := KillUsers(ctx, t.TempDir())

	assert.Error(t, err)
}

func TestKillUsersWithoutUsersKillsNothing(t *testing.T) {
	killed, err := KillUsers(context.Background(), t.TempDir())

	require.NoError(t, err)
	assert.Zero(t, killed)
}

func TestKillUsersRefusesAnEmptyRoot(t *testing.T) {
	_, err := KillUsers(context.Background(), "")

	assert.ErrorIs(t, err, errInvalidRoot)
}
