package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestParseTaskTabAcceptsTheThreeTabsOnly(t *testing.T) {
	t.Parallel()
	for _, tab := range []string{state.TaskTabActions, state.TaskTabFiles, state.TaskTabChanges} {
		got, ok := state.ParseTaskTab(tab)
		assert.True(t, ok, tab)
		assert.Equal(t, tab, got)
	}
	_, ok := state.ParseTaskTab("terminal")
	assert.False(t, ok, "a tab that the strip does not show is refused")
	_, ok = state.ParseTaskTab("")
	assert.False(t, ok)
}

func TestTaskTabsPathCarriesTheTabAndTheScope(t *testing.T) {
	t.Parallel()
	tabs := state.TaskTabs{Scope: "abc", TaskID: "t1", Active: state.TaskTabActions}

	assert.Equal(t, "/tasks/t1/tabs/changes?scope=abc", tabs.Path(state.TaskTabChanges))
	assert.Equal(t, "/tasks/t1/tabs/files", state.TaskTabs{TaskID: "t1"}.Path(state.TaskTabFiles))
	assert.Equal(t, "task-tabs-abc", tabs.RegionID())
}

func TestTaskPanelScopeIsTheWireFormOfTheTaskWorktree(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "task:7d3c2b1a-0000-4000-8000-000000000001", state.TaskPanelScope("7d3c2b1a-0000-4000-8000-000000000001"))
}
