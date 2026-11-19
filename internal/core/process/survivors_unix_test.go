//go:build unix

package process

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aliveFor reports whether pid is still alive after waiting up to wait for it to die.
func aliveFor(pid int, wait time.Duration) bool {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}

// A tracked descendant is killed even though its parent already exited and re-parented it.
func TestSurvivorsKillsATrackedDescendantAfterItsParentExited(t *testing.T) {
	parent := exec.Command("sh", "-c", "sleep 60 & echo $!; sleep 30")
	out, err := parent.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, parent.Start())
	t.Cleanup(func() { _ = parent.Process.Kill(); _ = parent.Wait() })
	var child int
	_, err = fmt.Fscan(out, &child)
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Kill(child, syscall.SIGKILL) })

	survivors := NewSurvivors()
	survivors.Track(context.Background(), parent.Process.Pid)
	require.NoError(t, parent.Process.Kill())
	_ = parent.Wait()

	assert.Equal(t, 1, survivors.Kill())
	assert.False(t, aliveFor(child, 3*time.Second), "the tracked descendant outlived its parent")
}

// The guard against pid reuse: a pid whose recorded start stamp no longer matches is never signalled.
func TestSurvivorsDoesNotKillAProcessWhoseStartStampChanged(t *testing.T) {
	other := exec.Command("sleep", "60")
	require.NoError(t, other.Start())
	t.Cleanup(func() { _ = other.Process.Kill(); _ = other.Wait() })

	survivors := NewSurvivors()
	survivors.known[other.Process.Pid] = "not-the-recorded-start"

	assert.Equal(t, 0, survivors.Kill())
	assert.True(t, aliveFor(other.Process.Pid, 300*time.Millisecond), "an unrelated process was killed")
}
