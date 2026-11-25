package desktop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCloseGuardAllowsAndRemembersAUserCloseThatCancelledTheUpdate(t *testing.T) {
	guard := &CloseGuard{Cancel: func() bool { return true }}

	assert.True(t, guard.Allow())
	assert.True(t, guard.UserClosed())

	guard.Reset()
	assert.False(t, guard.UserClosed())
}

func TestCloseGuardRefusesAUserCloseOnceTheUpdateCannotBeCancelled(t *testing.T) {
	guard := &CloseGuard{Cancel: func() bool { return false }}

	assert.False(t, guard.Allow())
	assert.False(t, guard.UserClosed(), "a refused close is not a cancel")
}

func TestCloseGuardLetsTheAppCloseItsOwnWindowWithoutCancelling(t *testing.T) {
	cancelled := 0
	guard := &CloseGuard{Cancel: func() bool { cancelled++; return true }}

	var allowed bool
	guard.Programmatic(func() { allowed = guard.Allow() })

	assert.True(t, allowed)
	assert.Zero(t, cancelled)
	assert.False(t, guard.UserClosed())
	assert.True(t, guard.Cancel != nil && !guardProgrammatic(guard), "the mark is cleared afterwards")
}

func guardProgrammatic(g *CloseGuard) bool { return g.programmed.Load() }
