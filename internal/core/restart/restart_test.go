package restart

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnlyTheShellValueMarksTheServerAsSupervised(t *testing.T) {
	assert.True(t, supervisedFrom("1"))
	assert.False(t, supervisedFrom("0"))
	assert.False(t, supervisedFrom(""))
}

func TestDecideDependsOnTheRequestAndTheSupervisor(t *testing.T) {
	tests := []struct {
		name      string
		requested bool
		supervise bool
		want      Action
	}{
		{"plain shutdown when supervised", false, true, ActionStop},
		{"plain shutdown when not supervised", false, false, ActionStop},
		{"restart under a supervisor hands over", true, true, ActionHandOver},
		{"restart without a supervisor respawns", true, false, ActionRespawn},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Decide(tt.requested, tt.supervise))
		})
	}
}

func TestRespawnNeverReopensTheBrowser(t *testing.T) {
	assert.Equal(t, []string{"--no-open"}, RespawnArgs(nil))
}

func TestRespawnKeepsAnExistingNoOpenFlagOnce(t *testing.T) {
	args := []string{"--no-open"}

	assert.Equal(t, []string{"--no-open"}, RespawnArgs(args))
}

func TestRespawnDoesNotMutateTheCallersArguments(t *testing.T) {
	args := []string{"--verbose"}

	got := RespawnArgs(args)

	assert.Equal(t, []string{"--verbose", "--no-open"}, got)
	assert.Equal(t, []string{"--verbose"}, args)
}

func TestRequestMadeBeforeWaitingIsNotLost(t *testing.T) {
	handle := NewHandle()
	assert.False(t, handle.IsRequested())

	handle.Request()
	handle.Request()

	assert.True(t, handle.IsRequested())
	select {
	case <-handle.Requested():
	case <-time.After(time.Second):
		require.FailNow(t, "pending request should wake the server")
	}
}
