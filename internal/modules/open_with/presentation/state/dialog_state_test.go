package state_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/open_with/presentation/state"
)

func TestIDsAndPathsCarryTheScope(t *testing.T) {
	require.Equal(t, "task-open-with", state.HostID(""))
	require.Equal(t, "task-open-with-s1", state.HostID("s1"))
	require.Equal(t, "task-open-with-s1-form", state.FormID("s1"))
	require.Equal(t, "task-open-with-s1-entries", state.PickerBodyID("s1"))
	require.Equal(t, "/open-with/tasks/t1", state.Path("t1", "", ""))
	require.Equal(t, "/open-with/tasks/t1/browse?scope=s1", state.BrowsePath("t1", "s1"))
	require.Equal(t, "/open-with/tasks/t1/choose", state.ChoosePath("t1", ""))
}

func TestPathEscapesTheTaskID(t *testing.T) {
	require.Equal(t, "/open-with/tasks/a%2Fb", state.Path("a/b", "", ""))
}
