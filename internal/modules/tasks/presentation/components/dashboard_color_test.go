package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestDashboardCardOfAColoredWorkspaceGetsARailAndADotNotAChip(t *testing.T) {
	t.Parallel()

	item := state.DashboardTaskItem{Task: dto.TaskDTO{ID: "t1"}, WorkspaceName: "iWorkflow", WorkspaceColor: "blue"}
	html := renderComponent(t, i18n.Es, components.DashboardTaskCard(item, "s"))

	assert.Contains(t, html, "ws-rail ws-blue")
	assert.Contains(t, html, `<span class="ws-dot"`)
	assert.NotContains(t, html, "ws-badge")
}

func TestDashboardCardOfAnUncoloredWorkspaceHasNoRail(t *testing.T) {
	t.Parallel()

	item := state.DashboardTaskItem{Task: dto.TaskDTO{ID: "t1"}, WorkspaceName: "iWorkflow"}
	html := renderComponent(t, i18n.Es, components.DashboardTaskCard(item, "s"))

	assert.NotContains(t, html, "ws-rail")
	assert.NotContains(t, html, "ws-dot")
}
