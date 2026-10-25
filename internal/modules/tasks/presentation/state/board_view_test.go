package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestListPathRepeatsTheWorkspaceAndTheCompletedToggle(t *testing.T) {
	assert.Equal(t, "/board/list?workspace=w1", state.ListPath("w1", false, 3))
	assert.Equal(t, "/board/list?completed=1&pages=3&workspace=w1", state.ListPath("w1", true, 3))
}

func TestListPathOmitsThePagesWhileCompletedIsOff(t *testing.T) {
	assert.NotContains(t, state.ListPath("w1", false, 5), "pages=")
}

func TestListPathClampsThePagesItRepeats(t *testing.T) {
	assert.Contains(t, state.ListPath("w1", true, 0), "pages=1")
	assert.Contains(t, state.ListPath("w1", true, 99), "pages=20")
}

func TestListPathEscapesTheWorkspaceIdentifier(t *testing.T) {
	assert.Equal(t, "/board/list?workspace=a+b%26c", state.ListPath("a b&c", false, 1))
}
