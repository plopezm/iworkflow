package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
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

func TestTaskCardHasAStableIDSoAMorphKeepsItsButtons(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, i18n.En, components.TaskCard(dto.TaskDTO{ID: "t-1", Stage: "backlog"}, "scope-1"))

	assert.Contains(t, html, `id="task-card-t-1-scope-1"`)
}
