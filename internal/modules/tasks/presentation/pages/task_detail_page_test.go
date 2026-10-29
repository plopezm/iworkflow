package pages_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/pages"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func renderIn(t *testing.T, loc i18n.Locale, cmp templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: loc, Chosen: true})
	var buf bytes.Buffer
	err := cmp.Render(ctx, &buf)
	require.NoError(t, err)
	return buf.String()
}

func testTaskView(stage string) state.TaskDetailView {
	return state.TaskDetailView{
		Scope: "sc1",
		Task: dto.TaskDTO{
			ID:         "11111111-2222-3333-4444-555555555555",
			Title:      "Mi Tarea",
			Stage:      stage,
			Status:     "running",
			Branch:     "pi/task-11111111",
			BaseBranch: "main",
		},
		CanMessage: true,
	}
}

func TestTaskDetailPageRendersSideBySideWithCollapsiblePanelWhenRunning(t *testing.T) {
	t.Parallel()
	view := testTaskView(state.StageWorking)
	tabs := state.TaskTabs{Scope: "sc1", TaskID: view.Task.ID, Active: state.TaskTabActions}

	html := renderIn(t, i18n.Es, pages.TaskDetailPage(
		view,
		tabs,
		templ.Raw("<p>acciones-body</p>"),
		nil,
		nil,
		nil,
	))

	// Left column has header and transcript scroller
	assert.Contains(t, html, "Mi Tarea")
	assert.Contains(t, html, `id="task-transcript-scroll-sc1"`)
	// Header has mobile button to open the side panel overlay
	assert.Contains(t, html, `data-mobile-open`)
	assert.Contains(t, html, `data-target="task-panel-sc1"`)
	assert.Contains(t, html, ">Panel<")

	// Right column has CollapsePanel
	assert.Contains(t, html, `data-collapse-root`)
	assert.Contains(t, html, `data-persist-key="iwf-task-panel-`+view.Task.ID+`"`)
	assert.Contains(t, html, `data-collapse-toggle`)
	// Mobile close overlay button
	assert.Contains(t, html, `data-overlay-close`)
	assert.Contains(t, html, `data-overlay-close`)
	// Tab region is inside the panel
	assert.Contains(t, html, `id="task-tabs-sc1"`)
	assert.Contains(t, html, "<p>acciones-body</p>")
}

func TestTaskDetailPageInBacklogDoesNotRenderRightPanel(t *testing.T) {
	t.Parallel()
	view := testTaskView(state.StageBacklog)
	tabs := state.TaskTabs{Scope: "sc1", TaskID: view.Task.ID, Active: state.TaskTabActions}

	html := renderIn(t, i18n.Es, pages.TaskDetailPage(
		view,
		tabs,
		templ.Raw("<p>acciones-body</p>"),
		nil,
		nil,
		nil,
	))

	assert.Contains(t, html, "Mi Tarea")
	assert.NotContains(t, html, `data-collapse-root`)
	assert.NotContains(t, html, `data-mobile-open`)
	assert.NotContains(t, html, `id="task-transcript-scroll-sc1"`)
}
