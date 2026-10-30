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
	"iworkflow/internal/modules/workspaces/presentation/state"
)

// render renders a component in Spanish, the language the e2e scripts run in.
func render(t *testing.T, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	var buffer bytes.Buffer
	require.NoError(t, component.Render(ctx, &buffer))
	return buffer.String()
}

func TestEmptyTableSaysThereAreNoWorkspaces(t *testing.T) {
	body := render(t, WorkspaceTable(state.TableView{}))

	assert.Contains(t, body, "Aún no hay workspaces registrados.")
	assert.NotContains(t, body, "<article", "no card is rendered without rows")
}

func TestTableRowEscapesTheNameAndLinksTheTabs(t *testing.T) {
	body := render(t, WorkspaceTable(state.TableView{Rows: []state.WorkspaceRow{{
		ID: "w1", Name: `<script>alert(1)</script>`, Path: "/repos/a", BaseBranch: "main",
		EditHref: "/workspaces/w1/edit", TerminalHref: "/workspaces/w1/terminal",
	}}}))

	assert.NotContains(t, body, "<script>alert(1)</script>")
	assert.Contains(t, body, "&lt;script&gt;")
	assert.Contains(t, body, `href="/workspaces/w1/edit"`)
	assert.Contains(t, body, `hx-post="/workspaces/w1/delete"`)
	assert.Contains(t, body, "¿Quitar el workspace &#34;&lt;script&gt;")
}

func TestTableIsSwappedOutOfBandOnlyWhenAskedTo(t *testing.T) {
	inPlace := render(t, WorkspaceTable(state.TableView{}))
	outOfBand := render(t, WorkspaceTable(state.TableView{OutOfBand: true}))

	assert.NotContains(t, inPlace, "hx-swap-oob")
	assert.Contains(t, outOfBand, `hx-swap-oob="true"`)
}

func TestTableShowsTheErrorOfTheLastAttempt(t *testing.T) {
	body := render(t, WorkspaceTable(state.TableView{Error: "el workspace tiene tareas activas"}))

	assert.Contains(t, body, "el workspace tiene tareas activas")
}

func TestPromptNamesTheReasonAndItsConfirmLabel(t *testing.T) {
	cases := map[string]struct {
		reason  string
		message string
		label   string
	}{
		"missing folder":   {state.ReasonMissingFolder, "La carpeta /repos/x no existe.", "Crear e inicializar"},
		"not a repository": {state.ReasonNotAGitRepo, "/repos/x no es un repositorio git.", "Inicializar"},
		"no commits":       {state.ReasonNoCommits, "El repositorio /repos/x no tiene commits.", "Crear commit"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			prompt := state.InitializationPrompt{Path: "/repos/x", Reason: tc.reason}

			body := render(t, registerPrompt(prompt))

			assert.Contains(t, body, tc.message)
			assert.Contains(t, body, tc.label)
		})
	}
}

func TestClosedPickerIsAnEmptyContainer(t *testing.T) {
	body := render(t, DirectoryPicker(state.DirectoryPicker{}))

	assert.Equal(t, `<div id="workspace-picker"></div>`, strings.TrimSpace(body))
}

func TestPickerWithoutEntriesSaysNoSubfolders(t *testing.T) {
	body := render(t, DirectoryPicker(state.DirectoryPicker{Open: true, Path: "/repos"}))

	assert.Contains(t, body, `id="modal-panel"`)
	assert.Contains(t, body, "Sin subcarpetas")
	assert.Contains(t, body, `disabled`, "the up button is disabled without a parent")
}

func TestPickerEntriesMarkGitRepositories(t *testing.T) {
	body := render(t, PickerEntries(state.DirectoryPicker{Entries: []state.DirectoryEntry{
		{Name: "repo", Path: "/repos/repo", IsGitRepo: true},
		{Name: "plain", Path: "/repos/plain"},
	}}))

	assert.Contains(t, body, ">git</span>")
	assert.Equal(t, 1, strings.Count(body, ">git</span>"), "only the repository carries the badge")
}
