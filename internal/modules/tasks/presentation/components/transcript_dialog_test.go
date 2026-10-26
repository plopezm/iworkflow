package components_test

import (
	"bytes"
	"context"
	"html"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

const dialogTestTask = "t1"

// dialogRequest is a dialog the agent opened, as the transcript receives it.
func dialogRequest(id string, kind dto.TranscriptDialogKind) dto.TranscriptEventDTO {
	return agentEvent(dto.TranscriptDialogRequested{Request: dto.TranscriptDialog{ID: id, Title: "¿Seguir?", Kind: kind}})
}

// dialogBody renders the transcript of the task with the events, in Spanish, and unescapes the attributes so the
// JSON values can be compared as written.
func dialogBody(t *testing.T, locale i18n.Locale, events ...dto.TranscriptEventDTO) string {
	t.Helper()
	transcript, _ := state.FoldTranscript(events)
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, components.TranscriptBody(dialogTestTask, transcript).Render(ctx, &out))
	return html.UnescapeString(out.String())
}

func TestPendingSelectDialogOffersEachOptionAsAnAnswer(t *testing.T) {
	t.Parallel()
	body := dialogBody(t, i18n.Es, dialogRequest("d1", dto.TranscriptSelectDialog{Options: []string{"Sí", "No"}}))
	assert.Contains(t, body, `hx-post="/tasks/t1/dialogs/d1/answer"`)
	assert.Contains(t, body, `hx-vals="{"kind":"value","value":"Sí"}"`)
	assert.Contains(t, body, `hx-vals="{"kind":"value","value":"No"}"`)
	assert.Contains(t, body, `hx-target="closest [data-dialog]"`)
	assert.Contains(t, body, `hx-swap="outerHTML"`)
}

func TestPendingConfirmDialogAsksYesOrNoWithItsMessage(t *testing.T) {
	t.Parallel()
	message := "Ejecutar ls"
	body := dialogBody(t, i18n.Es, dialogRequest("d1", dto.TranscriptConfirmDialog{Message: &message}))
	assert.Contains(t, body, "Ejecutar ls")
	assert.Contains(t, body, `hx-vals="{"kind":"confirmed","confirmed":true}"`)
	assert.Contains(t, body, `hx-vals="{"kind":"confirmed","confirmed":false}"`)
}

func TestPendingInputDialogPostsTheTextTypedInIt(t *testing.T) {
	t.Parallel()
	placeholder := "Nombre"
	body := dialogBody(t, i18n.Es, dialogRequest("d1", dto.TranscriptInputDialog{Placeholder: &placeholder}))
	assert.Contains(t, body, `<form`)
	assert.Contains(t, body, `hx-post="/tasks/t1/dialogs/d1/answer"`)
	assert.Contains(t, body, `name="value"`)
	assert.Contains(t, body, `placeholder="Nombre"`)
	assert.Contains(t, body, `name="kind" value="value"`)
}

func TestPendingEditorDialogPostsTheTextAndStartsWithThePrefill(t *testing.T) {
	t.Parallel()
	prefill := "borrador"
	body := dialogBody(t, i18n.Es, dialogRequest("d1", dto.TranscriptEditorDialog{Prefill: &prefill}))
	assert.Contains(t, body, `<textarea`)
	assert.Contains(t, body, `name="value"`)
	assert.Contains(t, body, ">borrador</textarea>")
	assert.Contains(t, body, `hx-post="/tasks/t1/dialogs/d1/answer"`)
}

func TestPendingDialogOffersCancelWhateverItsKind(t *testing.T) {
	t.Parallel()
	body := dialogBody(t, i18n.Es, dialogRequest("d1", dto.TranscriptInputDialog{}))
	assert.Contains(t, body, `hx-vals="{"kind":"cancelled"}"`)
	assert.Contains(t, body, "Cancelar diálogo")
}

func TestAnsweredDialogShowsNoControls(t *testing.T) {
	t.Parallel()
	body := dialogBody(t, i18n.Es,
		dialogRequest("d1", dto.TranscriptSelectDialog{Options: []string{"Sí"}}),
		agentEvent(dto.TranscriptDialogResolved{DialogID: "d1"}),
	)
	assert.Contains(t, body, "(respondido)")
	assert.NotContains(t, body, "hx-post")
	assert.NotContains(t, body, "Cancelar diálogo")
}

// A dialog its turn closed without an answer reads as closed in every language, not as answered by the user.
func TestDialogClosedByItsTurnIsLabelledClosedInEveryLanguage(t *testing.T) {
	t.Parallel()
	labels := map[i18n.Locale]string{
		i18n.En: "(closed)",
		i18n.Es: "(cerrado)",
		i18n.Zh: "(已关闭)",
		i18n.De: "(geschlossen)",
		i18n.Fr: "(fermé)",
	}
	item := state.DialogItem{Request: dto.TranscriptDialog{ID: "d1", Title: "¿Seguir?"}, Resolved: true, ClosedByTurn: true}
	for locale, label := range labels {
		ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
		var buf bytes.Buffer
		require.NoError(t, components.TranscriptDialog(dialogTestTask, item, "").Render(ctx, &buf))
		body := html.UnescapeString(buf.String())
		assert.Contains(t, body, label, locale)
		assert.NotContains(t, body, "hx-post", locale)
	}
}

func TestDialogAnswerPathEscapesTheDialogID(t *testing.T) {
	t.Parallel()
	body := dialogBody(t, i18n.Es, dialogRequest("perm/a b", dto.TranscriptSelectDialog{Options: []string{"Sí"}}))
	assert.Contains(t, body, `hx-post="/tasks/t1/dialogs/perm%2Fa%20b/answer"`)
}

// The controls of a dialog are in the five languages of the interface.
func TestPendingDialogControlsAreLocalized(t *testing.T) {
	t.Parallel()
	cases := map[i18n.Locale]struct{ yes, no, cancel string }{
		i18n.En: {"Yes", "No", "Cancel dialog"},
		i18n.Es: {"Sí", "No", "Cancelar diálogo"},
		i18n.Zh: {"是", "否", "取消对话框"},
		i18n.De: {"Ja", "Nein", "Dialog abbrechen"},
		i18n.Fr: {"Oui", "Non", "Annuler la boîte de dialogue"},
	}
	message := "¿Seguir?"
	for locale, want := range cases {
		body := dialogBody(t, locale, dialogRequest("d1", dto.TranscriptConfirmDialog{Message: &message}))
		assert.Contains(t, body, ">"+want.yes+"<", locale)
		assert.Contains(t, body, ">"+want.no+"<", locale)
		assert.Contains(t, body, ">"+want.cancel+"<", locale)
	}
}
