package state_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/task_hooks/presentation/state"
)

func TestPathsKeepTheWorkspaceAndTheHookInTheAddress(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/workspaces/w1/hooks", state.Path("w1", ""))
	assert.Equal(t, "/workspaces/w1/hooks/panel", state.PanelPath("w1"))
	assert.Equal(t, "/workspaces/w1/hooks", state.SavePath("w1"))
	assert.Equal(t, "/workspaces/w1/hooks/reorder", state.ReorderPath("w1"))
	assert.Equal(t, "/workspaces/w1/hooks/h1/enabled", state.EnabledPath("w1", "h1"))
	assert.Equal(t, "/workspaces/w1/hooks/h1/delete", state.DeletePath("w1", "h1"))
	assert.Equal(t, "/workspaces/w1/hooks/h1/test", state.TestPath("w1", "h1"))
	assert.Equal(t, "workspace-hooks-w1", state.HostID("w1"))
}

func TestPathsEscapeTheIdentifiers(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/workspaces/a%2Fb/hooks/c%20d/test", state.TestPath("a/b", "c d"))
}

func TestEditorPathCarriesTheQueryOnlyWhenThereIsOne(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "/workspaces/w1/hooks/editor", state.EditorPath("w1", nil))
	assert.Equal(t, "/workspaces/w1/hooks/editor?event=task_start&phase=pre",
		state.EditorPath("w1", url.Values{"phase": {"pre"}, "event": {"task_start"}}))
}
