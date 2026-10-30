package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestListPathCarriesTheWorkspaceAndTheCompletedToggle(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/board/list?workspace=w1", state.ListPath("", "w1", false, 3))
	assert.Equal(t, "/board/list?completed=1&pages=3&workspace=w1", state.ListPath("", "w1", true, 3))
}

func TestListPathCarriesTheInstanceScope(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/board/list?completed=1&pages=3&scope=k3&workspace=w1", state.ListPath("k3", "w1", true, 3))
}

func TestListPathDropsPagesWhenCompletedIsOff(t *testing.T) {
	t.Parallel()
	assert.NotContains(t, state.ListPath("", "w1", false, 5), "pages=")
}

func TestListPathClampsThePages(t *testing.T) {
	t.Parallel()
	assert.Contains(t, state.ListPath("", "w1", true, 0), "pages=1")
	assert.Contains(t, state.ListPath("", "w1", true, 99), "pages=20")
}

func TestListPathEscapesTheWorkspace(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/board/list?workspace=a+b%26c", state.ListPath("", "a b&c", false, 1))
}

func TestFramePathAnswersTheWholeListElementForTheSameAddress(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/board/frame?workspace=w1", state.FramePath("", "w1", false, 3))
	assert.Equal(t, "/board/frame?completed=1&pages=3&scope=k3&workspace=w1", state.FramePath("k3", "w1", true, 3))
}
