package components

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/workspace_branches/presentation/state"
)

// render renders a component in Spanish, the language the workspaces screens are checked in.
func render(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

func listWith(rows ...state.BranchRow) state.ListView {
	return state.ListView{DeleteAction: "/workspaces/w1/branches/delete", Rows: rows}
}

func TestScreenSearchFiltersOnTheServerWithoutCustomScript(t *testing.T) {
	body := render(t, BranchesScreen(state.ScreenView{
		WorkspaceName: "alpha",
		SearchHref:    "/workspaces/w1/branches",
		List:          listWith(),
	}))

	assert.Contains(t, body, `hx-get="/workspaces/w1/branches"`)
	assert.Contains(t, body, `hx-trigger="input changed delay:250ms, search"`)
	assert.Contains(t, body, `hx-target="#branch-list"`)
	assert.Contains(t, body, `name="q"`)
	assert.NotContains(t, body, "<script", "the screen adds no javascript of its own")
}

func TestRowsCarryTheReferenceAndAPerRowDeleteThatIgnoresTheCheckboxes(t *testing.T) {
	body := render(t, BranchListBody(listWith(state.BranchRow{
		Value: "remote:origin/gone", Name: "origin/gone", Remote: true,
		DeleteHref: "/workspaces/w1/branches/delete?branch=remote%3Aorigin%2Fgone&q=",
	})))

	assert.Contains(t, body, `name="branch" value="remote:origin/gone"`)
	assert.Contains(t, body, `hx-post="/workspaces/w1/branches/delete?branch=remote%3Aorigin%2Fgone&amp;q="`)
	assert.Contains(t, body, `hx-params="none"`)
}

func TestRemoteDeleteSaysItRemovesTheBranchFromTheRemote(t *testing.T) {
	remote := render(t, BranchListBody(listWith(state.BranchRow{Value: "remote:origin/gone", Name: "origin/gone", Remote: true, DeleteHref: "/d"})))
	local := render(t, BranchListBody(listWith(state.BranchRow{Value: "local:feat", Name: "feat", DeleteHref: "/d"})))

	assert.Contains(t, remote, "Se borra también del remoto", "the confirmation names the remote")
	assert.NotContains(t, local, "Se borra también del remoto")
	assert.Contains(t, local, "Se borra de este equipo")
	assert.Contains(t, remote, `hx-confirm=`)
	assert.Contains(t, local, `hx-confirm=`)
}

func TestBulkDeleteAsksForConfirmationAndSubmitsTheCheckedBoxes(t *testing.T) {
	body := render(t, BranchListBody(listWith(state.BranchRow{Value: "local:feat", Name: "feat", DeleteHref: "/d"})))

	assert.Contains(t, body, `id="branch-delete-form"`)
	assert.Contains(t, body, `hx-post="/workspaces/w1/branches/delete"`)
	assert.Contains(t, body, `type="submit"`)
	assert.Contains(t, body, "Eliminar seleccionadas")
	assert.Contains(t, body, "¿Eliminar las ramas seleccionadas?")
}

func TestProtectedRowIsDisabledAndSaysWhy(t *testing.T) {
	body := render(t, BranchListBody(listWith(state.BranchRow{
		Value: "local:main", Name: "main", Protected: true, Reason: "checked_out", DeleteHref: "/d",
	})))

	assert.Contains(t, body, `value="local:main"`)
	assert.Regexp(t, `type="checkbox"[^>]*disabled`, body)
	assert.Regexp(t, `disabled[^>]*hx-post="/d"|hx-post="/d"[^>]*disabled`, body)
	assert.Contains(t, body, "Rama actual")
}

func TestUnprotectedRowHasNoReasonAndIsEnabled(t *testing.T) {
	body := render(t, BranchListBody(listWith(state.BranchRow{Value: "local:feat", Name: "feat", DeleteHref: "/d"})))

	assert.NotRegexp(t, `type="checkbox"[^>]*disabled`, body)
	assert.NotContains(t, body, "Rama actual")
}

func TestResultsNameEveryOutcome(t *testing.T) {
	body := render(t, BranchListBody(state.ListView{
		Results: []state.ResultLine{
			{Name: "a", Outcome: "deleted"},
			{Name: "b", Outcome: "refused", Reason: "base_branch"},
			{Name: "c", Outcome: "failed", Failure: "not_found"},
			{Name: "d", Outcome: "failed", Failure: "invalid"},
			{Name: "e", Outcome: "failed", Failure: "git", Error: "boom"},
		},
	}))

	assert.Contains(t, body, "Eliminada: a")
	assert.Contains(t, body, "No se eliminó b: Rama base del workspace")
	assert.Contains(t, body, "c ya no existe")
	assert.Contains(t, body, "Nombre de rama no válido: d")
	assert.Contains(t, body, "No se pudo eliminar e: boom")
}

func TestNoneSelectedAndEmptyListSayWhatHappened(t *testing.T) {
	none := render(t, BranchListBody(state.ListView{NoneSelected: true, DeleteAction: "/d"}))
	empty := render(t, BranchListBody(state.ListView{DeleteAction: "/d"}))

	assert.Contains(t, none, "Selecciona al menos una rama.")
	assert.Contains(t, empty, "No hay ramas que coincidan.")
	assert.NotContains(t, empty, `type="submit"`, "nothing to delete, so no bulk button")
}

func TestListErrorIsShownInsteadOfTheRows(t *testing.T) {
	body := render(t, BranchListBody(state.ListView{Error: "workspace not found"}))

	assert.Contains(t, body, "workspace not found")
}

func TestRowsEscapeTheirNames(t *testing.T) {
	body := render(t, BranchListBody(listWith(state.BranchRow{Value: "local:<b>", Name: "<b>x</b>", DeleteHref: "/d"})))

	assert.NotContains(t, body, "<b>x</b>")
	assert.True(t, strings.Contains(body, "&lt;b&gt;x&lt;/b&gt;"))
}

// htmx reads hx-confirm from the element that issues the request: for the bulk delete that is the form, so the
// confirmation must sit on the form and not on its submit button.
func TestBulkDeleteAsksForConfirmationOnTheFormItself(t *testing.T) {
	body := render(t, BranchListBody(listWith(state.BranchRow{Value: "local:feat", Name: "feat", DeleteHref: "/d"})))

	start := strings.Index(body, `<form id="branch-delete-form"`)
	require.GreaterOrEqual(t, start, 0)
	formTag := body[start : start+strings.Index(body[start:], ">")]
	assert.Contains(t, formTag, "hx-confirm=", "the form carries the confirmation of the bulk delete")
	assert.Contains(t, formTag, "Las remotas se borran también del remoto")
	submit := body[strings.Index(body, `type="submit"`):]
	submit = submit[:strings.Index(submit, ">")]
	assert.NotContains(t, submit, "hx-confirm", "the submit button is not where htmx looks")
}

func TestListOffersSelectAllAndBreathingRoom(t *testing.T) {
	list := render(t, BranchListBody(listWith(state.BranchRow{Value: "local:feat", Name: "feat", DeleteHref: "/d"})))
	assert.Contains(t, list, "data-branch-select-all")
	assert.Contains(t, list, "Seleccionar todas")

	screen := render(t, BranchesScreen(state.ScreenView{WorkspaceName: "alpha", SearchHref: "/s", List: listWith()}))
	assert.Contains(t, screen, `id="workspace-branches" class="stack h-full min-h-0 overflow-y-auto px-6 py-4"`)
}
