package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
)

func TestWorkspaceSwitcherRendersSelectPillWithInputAndChevron(t *testing.T) {
	t.Parallel()

	workspaces := []dto.WorkspaceOptionDTO{
		{ID: "ws-1", Name: "iworkflow", BaseBranch: "master"},
		{ID: "ws-2", Name: "ifinances", BaseBranch: "main"},
	}

	html := renderComponent(t, i18n.Es, components.WorkspaceSwitcher(workspaces, "ws-1", "board-ws-test"))

	assert.Contains(t, html, `class="select-pill max-w-xs"`)
	assert.Contains(t, html, `<select id="board-ws-test" name="workspace" class="select-pill-input">`)
	assert.Contains(t, html, `class="select-pill-chevron"`)
	assert.Contains(t, html, `<option value="ws-1" selected>iworkflow (master)</option>`)
	assert.Contains(t, html, `<option value="ws-2">ifinances (main)</option>`)
}
