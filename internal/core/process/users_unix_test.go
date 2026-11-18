//go:build unix

package process

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func startSleeper(t *testing.T, dir string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	cmd.Dir = dir
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return cmd
}

func TestKillUsersKillsTheProcessesWorkingInsideTheRootOnly(t *testing.T) {
	root := t.TempDir()
	inside := startSleeper(t, root)
	outside := startSleeper(t, t.TempDir())
	done := make(chan struct{})
	go func() { _ = inside.Wait(); close(done) }()

	killed, err := KillUsers(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, 1, killed)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the process working inside the root is still alive")
	}
	assert.NoError(t, outside.Process.Signal(syscallZero()), "the process outside the root is untouched")
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
