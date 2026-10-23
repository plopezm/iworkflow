//go:build unix

package process

import (
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKillGroupTerminatesTheGroupLeaderAndItsMembers(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	SetProcessGroup(cmd)
	require.NoError(t, cmd.Start())
	pgid := cmd.Process.Pid
	require.True(t, GroupAlive(pgid))

	require.NoError(t, KillGroup(pgid))

	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case err := <-waited:
		assert.Error(t, err, "a killed process should not exit cleanly")
	case <-time.After(10 * time.Second):
		require.FailNow(t, "killed group leader was not reaped")
	}
	assert.False(t, GroupAlive(pgid))
}

func TestKillGroupIgnoresAGoneGroup(t *testing.T) {
	cmd := exec.Command("true")
	SetProcessGroup(cmd)
	require.NoError(t, cmd.Run())

	assert.NoError(t, KillGroup(cmd.Process.Pid))
}

func TestKillGroupRefusesTheCallersOwnGroup(t *testing.T) {
	assert.Error(t, KillGroup(0))
	assert.Error(t, KillGroup(-1))
	assert.False(t, GroupAlive(0))
}
