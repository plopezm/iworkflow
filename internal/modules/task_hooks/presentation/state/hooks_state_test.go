package state

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The paths of the screen address the hook and the assignment they name, and escape the ids, so a value never breaks
// the address it is built into.
func TestPathsAddressTheHookAndTheAssignment(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/hooks/h-1/delete", DeletePath("h-1"))
	assert.Equal(t, "/hooks/h-1/test", TestPath("h-1"))
	assert.Equal(t, "/hooks/h-1/assignments", AssignmentsPath("h-1"))
	assert.Equal(t, "/hooks/h-1/assignments/a-1", AssignmentPath("h-1", "a-1"))
	assert.Equal(t, "/hooks/h-1/assignments/a-1/delete", AssignmentDeletePath("h-1", "a-1"))
	assert.Equal(t, "/hooks/order", OrderPath())
	assert.Equal(t, "/hooks/library", LibraryPath())
	assert.Equal(t, "/hooks/group", GroupPath())
	assert.Equal(t, "/hooks/editor", EditorPath())
	assert.Equal(t, "/hooks", SavePath())
	assert.Equal(t, "hooks-screen", HostID)
}

func TestPathsEscapeTheIdentifiers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/hooks/a%2Fb/delete", DeletePath("a/b"))
	assert.Equal(t, "/hooks/a%20b/assignments/x%3Fy", AssignmentPath("a b", "x?y"))
}

// The history address names its filters, and the applied marker is always there so the failed box can be off.
func TestHistoryPathCarriesItsFilters(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/hooks/history?applied=1&failed=1&hook=h-1&project=ws-1", HistoryPath("h-1", "ws-1", true))
	assert.Equal(t, "/hooks/history?applied=1&hook=&project=", HistoryPath("", "", false))
}
