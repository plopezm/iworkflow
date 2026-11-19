//go:build windows

package process

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A process that detaches from its parent stays in the job, so closing the job kills it.
func TestSurvivorsKillsTheProcessTrackedInTheJob(t *testing.T) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", "Start-Sleep -Seconds 60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })

	survivors := NewSurvivors()
	survivors.Track(context.Background(), cmd.Process.Pid)

	assert.Equal(t, 1, survivors.Kill())
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the job did not kill the tracked process")
	}
	assert.Equal(t, 0, survivors.Kill(), "a second kill has nothing left")
}
