package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestBoardToolbarRendersHiddenWorkspaceInputWithoutSelect(t *testing.T) {
	t.Parallel()

	view := state.BoardView{
		Scope: "scope-123",
		Choice: state.WorkspaceChoice{
			Kind: state.ChoiceSelected,
			ID:   "ws-1",
			Name: "iworkflow",
		},
	}

	html := renderComponent(t, i18n.Es, components.BoardToolbar(view))

	assert.Contains(t, html, `<input type="hidden" id="board-workspace-scope-123" name="workspace" value="ws-1">`)
	assert.NotContains(t, html, `<select`)
}

func TestBoardToolbarPutsTheMemoriesButtonBeforeNewTaskOnlyForAWorkspace(t *testing.T) {
	t.Parallel()

	chosen := state.BoardView{Scope: "s", Choice: state.WorkspaceChoice{Kind: state.ChoiceSelected, ID: "ws-1", Name: "iworkflow"}}
	html := renderComponent(t, i18n.Es, components.BoardToolbar(chosen))

	memories := strings.Index(html, `href="/workspaces/ws-1/memories"`)
	newTask := strings.Index(html, `href="`+state.NewTaskPathFrom("ws-1", state.BoardPath("ws-1"))+`"`)
	assert.Positive(t, memories)
	assert.Less(t, memories, newTask, "the memories button is left of the new task button")

	none := renderComponent(t, i18n.Es, components.BoardToolbar(state.BoardView{Scope: "s"}))
	assert.NotContains(t, none, "/memories")
}
