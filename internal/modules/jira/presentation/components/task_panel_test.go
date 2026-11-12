package components_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/jira/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/jira/presentation/components"
	"iworkflow/internal/modules/jira/presentation/state"
)

func renderPanel(t *testing.T, locale i18n.Locale, view state.TaskPanelView) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var out strings.Builder
	require.NoError(t, components.JiraTaskPanel(view).Render(ctx, &out))
	return out.String()
}

func linkedIssue() *dto.IssueDTO {
	priority := "High"
	assignee := "Ana"
	return &dto.IssueDTO{
		Key:         "B-7",
		Summary:     "Login roto",
		Description: "No entra nadie",
		Status:      "En curso",
		Priority:    &priority,
		Assignee:    &assignee,
		URL:         "https://jira.example/browse/B-7",
	}
}

func TestJiraTaskPanelWithoutAnIssueOnlyOffersTheLink(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{TaskID: "t1"}

	html := renderPanel(t, i18n.Es, view)

	assert.Contains(t, html, `id="jira-task-panel-t1"`)
	assert.Contains(t, html, `hx-post="/jira/tasks/t1/link"`)
	assert.Contains(t, html, `name="key"`)
	assert.Contains(t, html, ">Vincular<")
	assert.Contains(t, html, `hx-target="#jira-task-panel-t1"`)
	assert.Contains(t, html, `hx-swap="outerHTML"`)
	assert.NotContains(t, html, `name="time"`)
	assert.NotContains(t, html, "Registrar en Jira")
}

func TestJiraTaskPanelLinkedShowsTheIssueAndItsActions(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{
		TaskID: "t1",
		Issue:  linkedIssue(),
		Comments: []dto.CommentDTO{
			{Author: "Luis", Body: "primero", CreatedAt: "2026-11-10T09:00:00.000+0000"},
		},
		Transitions: []dto.TransitionDTO{{ID: "31", Name: "Cerrar", ToStatus: "Done"}},
	}

	html := renderPanel(t, i18n.Es, view)

	assert.Contains(t, html, ">B-7<")
	assert.Contains(t, html, "Login roto")
	assert.Contains(t, html, "En curso")
	assert.Contains(t, html, "High")
	assert.Contains(t, html, "Ana")
	assert.Contains(t, html, `href="https://jira.example/browse/B-7"`)
	assert.Contains(t, html, `hx-post="/jira/tasks/t1/refresh"`)
	assert.Contains(t, html, ">Refrescar<")
	assert.Contains(t, html, "<details>")
	assert.Contains(t, html, "No entra nadie")
	assert.Contains(t, html, "Luis")
	assert.Contains(t, html, `hx-post="/jira/tasks/t1/transition"`)
	assert.Contains(t, html, "Cerrar → Done")
	assert.NotContains(t, html, `hx-post="/jira/tasks/t1/link"`)
}

func TestJiraTaskPanelShowsTheLatestThreeComments(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{TaskID: "t1", Issue: linkedIssue()}
	for _, author := range []string{"uno", "dos", "tres", "cuatro"} {
		view.Comments = append(view.Comments, dto.CommentDTO{Author: author, Body: "x", CreatedAt: "2026-11-10T09:00:00.000+0000"})
	}

	html := renderPanel(t, i18n.Es, view)

	assert.NotContains(t, html, "<strong>uno</strong>")
	assert.Contains(t, html, "<strong>dos</strong>")
	assert.Contains(t, html, "<strong>cuatro</strong>")
}

func TestJiraTaskPanelWorklogIsPrefilledAndUncheckedByDefault(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{
		TaskID:       "t1",
		Issue:        linkedIssue(),
		Pending:      true,
		ProposedTime: "1h 20m",
		TimeText:     "1h 20m",
		Description:  "Migrar login\n\n- add login",
	}

	html := renderPanel(t, i18n.Es, view)

	assert.Contains(t, html, `hx-post="/jira/tasks/t1/worklog"`)
	assert.Contains(t, html, `name="time"`)
	assert.Contains(t, html, `value="1h 20m"`)
	assert.Contains(t, html, "Migrar login")
	assert.Contains(t, html, `name="also_comment"`)
	assert.NotContains(t, html, `name="also_comment" value="on" checked`)
	assert.Contains(t, html, "Comentar también en el issue")
	assert.Contains(t, html, `class="btn-primary"`)
	assert.Contains(t, html, "Registrar en Jira")
	assert.NotContains(t, html, `class="btn-primary" disabled`)
}

func TestJiraTaskPanelFinishedTaskShowsABannerNotAModal(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{
		TaskID:       "t1",
		Issue:        linkedIssue(),
		Finished:     true,
		Pending:      true,
		ProposedTime: "1h 30m",
	}

	html := renderPanel(t, i18n.Es, view)

	assert.Contains(t, html, `role="status"`)
	assert.Contains(t, html, "Task terminada, ¿registrar 1h 30m?")
	assert.NotContains(t, html, "dialog")
	assert.NotContains(t, html, "modal")
}

func TestJiraTaskPanelRunningTaskShowsNoBanner(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{TaskID: "t1", Issue: linkedIssue(), Finished: false, Pending: true, ProposedTime: "1h 30m"}

	html := renderPanel(t, i18n.Es, view)

	assert.NotContains(t, html, `role="status"`)
	assert.NotContains(t, html, "Task terminada")
}

func TestJiraTaskPanelShowsWhatWasLoggedAndDisablesTheButton(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{
		TaskID:  "t1",
		Issue:   linkedIssue(),
		Logged:  "1h 30m",
		Pending: false,
		Worklogs: []dto.WorklogDTO{
			{JiraID: "777", Time: "1h 30m", Comment: "x", LoggedAt: "2026-11-11 12:00"},
		},
	}

	html := renderPanel(t, i18n.Es, view)

	assert.Contains(t, html, "Registrado: 1h 30m")
	assert.Contains(t, html, "No queda tiempo por registrar.")
	assert.Contains(t, html, `class="btn-primary" disabled>`)
	assert.Contains(t, html, "Registros anteriores")
	assert.Contains(t, html, "2026-11-11 12:00 · 1h 30m")
}

func TestJiraTaskPanelShowsActionErrorsInline(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{
		TaskID:    "t1",
		Issue:     linkedIssue(),
		ActionErr: "tiempo no válido",
		TimeText:  "soon",
		Pending:   true,
	}

	html := renderPanel(t, i18n.Es, view)

	assert.Contains(t, html, "tiempo no válido")
	assert.Contains(t, html, `value="soon"`)
}

func TestJiraTaskPanelShowsReadFailuresInTheirSections(t *testing.T) {
	t.Parallel()
	view := state.TaskPanelView{
		TaskID:         "t1",
		Issue:          linkedIssue(),
		CommentsErr:    "jira no responde",
		TransitionsErr: "sin transiciones",
		WorklogErr:     "no se pudo leer el tiempo",
	}

	html := renderPanel(t, i18n.En, view)

	assert.Contains(t, html, "jira no responde")
	assert.Contains(t, html, "sin transiciones")
	assert.Contains(t, html, "no se pudo leer el tiempo")
	assert.Contains(t, html, "Log time", "the worklog form stays usable when its data fails")
}
