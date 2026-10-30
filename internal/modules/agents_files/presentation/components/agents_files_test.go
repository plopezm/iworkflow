package components_test

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/agents_files/presentation/components"
	"iworkflow/internal/modules/agents_files/presentation/state"
)

// disabledAttr matches the boolean disabled attribute, not hx-disabled-elt.
var disabledAttr = regexp.MustCompile(`\sdisabled[\s>]`)

func spanish() context.Context {
	return i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
}

func render(t *testing.T, ctx context.Context, component templ.Component) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

func sampleItem() state.FileItem {
	return state.FileItem{
		ID:          "6f1c2a3e-0000-4000-8000-000000000001",
		Description: "Reglas",
		Path:        "/data/a.md",
		CreatedAt:   "2026-01-02T03:04:05+00:00",
		Projects:    []state.ProjectOption{{ID: "p1", Name: "demo"}},
	}
}

func TestEmptyListShowsThePlaceholder(t *testing.T) {
	out := render(t, spanish(), components.FileList(state.FileList{}))

	require.Contains(t, out, `<p class="muted">Aún no hay ficheros AGENTS.md.</p>`)
	require.Contains(t, out, `id="agents-files-list"`)
}

func TestListErrorReplacesThePlaceholder(t *testing.T) {
	out := render(t, spanish(), components.FileList(state.FileList{Error: "error running server function: boom"}))

	require.Contains(t, out, `<p class="text-sm text-error">error running server function: boom</p>`)
	require.NotContains(t, out, "Aún no hay ficheros AGENTS.md.")
}

func TestFileItemShowsTheDataAndTheProjectSelector(t *testing.T) {
	out := render(t, spanish(), components.FileItem(sampleItem()))

	require.Contains(t, out, `<article id="agents-file-6f1c2a3e-0000-4000-8000-000000000001"`)
	require.Contains(t, out, "02/01/2026 03:04 UTC")
	require.Contains(t, out, "/data/a.md")
	require.Contains(t, out, `hx-delete="/agents-files/6f1c2a3e-0000-4000-8000-000000000001"`)
	require.Contains(t, out, `hx-confirm="¿Eliminar &#34;Reglas&#34;?"`)
	require.Contains(t, out, `<option value="p1"`)
	require.Contains(t, out, "Elige un proyecto")
}

func TestFileItemWithoutWorkspacesShowsTheHint(t *testing.T) {
	item := sampleItem()
	item.Projects = nil

	out := render(t, spanish(), components.FileItem(item))

	require.Contains(t, out, "No hay workspaces registrados.")
	require.NotContains(t, out, "Añadir a proyecto")
}

func TestApplyButtonIsDisabledUntilAProjectIsChosen(t *testing.T) {
	disabled := render(t, spanish(), components.ApplyButton("f1", ""))
	enabled := render(t, spanish(), components.ApplyButton("f1", "p1"))

	require.Regexp(t, disabledAttr, disabled)
	require.NotRegexp(t, disabledAttr, enabled)
	require.Contains(t, enabled, `hx-post="/agents-files/f1/apply"`)
	require.Contains(t, enabled, `hx-include="#project-f1"`)
}

func TestTargetExistsShowsTheOverwritePrompt(t *testing.T) {
	item := sampleItem()
	item.Feedback = state.FeedbackTargetExists
	item.SelectedProjectID = "p1"

	out := render(t, spanish(), components.FileItem(item))

	require.Contains(t, out, "Ya existe AGENTS.md en ese proyecto. ¿Sobrescribir?")
	require.Contains(t, out, "Sobrescribir")
	require.Contains(t, out, `hx-vals="{&#34;overwrite&#34;:&#34;true&#34;}"`)
}

func TestCommittedFeedbackShowsTheResultAndClose(t *testing.T) {
	item := sampleItem()
	item.Feedback = state.FeedbackCommitted

	out := render(t, spanish(), components.FileItem(item))

	require.Contains(t, out, `<p class="text-sm text-ok">Añadido y commiteado</p>`)
	require.Contains(t, out, ">Cerrar</button>")
}

func TestNotCommittedFeedbackUsesTheWarningStyle(t *testing.T) {
	item := sampleItem()
	item.Feedback = state.FeedbackNotCommitted

	out := render(t, context.Background(), components.FileItem(item))

	require.Contains(t, out, `<p class="text-sm text-warn">Added (the commit could not be created; left uncommitted)</p>`)
}

func TestUploadSectionKeepsTheTypedDescriptionAndShowsTheError(t *testing.T) {
	out := render(t, spanish(), components.UploadSection(state.UploadForm{
		Description: "reglas",
		Error:       "error running server function: la descripción es obligatoria",
	}))

	require.Contains(t, out, `value="reglas"`)
	require.Contains(t, out, `<p class="text-sm text-error">error running server function: la descripción es obligatoria</p>`)
	require.Contains(t, out, `hx-post="/agents-files/upload"`)
}

func TestUploadDoneSwapsTheListOutOfBand(t *testing.T) {
	out := render(t, spanish(), components.UploadDone(state.FileList{}))

	require.Contains(t, out, `id="agents-files-upload"`)
	require.Contains(t, out, `<div id="agents-files-list" class="stack" hx-swap-oob="true">`)
	require.NotContains(t, out, `value="reglas"`)
}
