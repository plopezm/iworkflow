package dto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// Each kind of dialog keeps what the form needs to draw it: the options, the message, the placeholder or the prefill.
func TestTranscriptEventsKeepTheKindOfADialogRequest(t *testing.T) {
	t.Parallel()
	taskID, err := models.ParseTaskID("0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b")
	require.NoError(t, err)
	message := "¿Permitir ls?"
	placeholder := "Escribe un nombre"
	prefill := "borrador"
	cases := map[string]struct {
		kind models.DialogKind
		want TranscriptDialogKind
	}{
		"select":  {models.SelectDialog{Options: []string{"Sí", "No"}}, TranscriptSelectDialog{Options: []string{"Sí", "No"}}},
		"confirm": {models.ConfirmDialog{Message: &message}, TranscriptConfirmDialog{Message: &message}},
		"input":   {models.InputDialog{Placeholder: &placeholder}, TranscriptInputDialog{Placeholder: &placeholder}},
		"editor":  {models.EditorDialog{Prefill: &prefill}, TranscriptEditorDialog{Prefill: &prefill}},
	}
	for name, tc := range cases {
		events := []models.TaskEvent{{
			TaskID: taskID,
			Kind: models.AgentEventKind{Event: models.DialogRequested{Request: models.DialogRequest{
				ID:    "d1",
				Title: "¿Seguir?",
				Kind:  tc.kind,
			}}},
		}}
		mapped, err := TranscriptEventsFromDomain(events)
		require.NoError(t, err, name)
		require.Len(t, mapped, 1, name)
		want := TranscriptDialogRequested{Request: TranscriptDialog{ID: "d1", Title: "¿Seguir?", Kind: tc.want}}
		assert.Equal(t, want, mapped[0].Agent, name)
	}
}

// unknownDialogKind is a kind the transcript has no form for. Embedding a known kind gives it the unexported method of
// models.DialogKind, so it can be built outside the models package.
type unknownDialogKind struct{ models.SelectDialog }

// A dialog of a kind without a form is an error of the history, not a dialog the transcript silently drops.
func TestTranscriptEventsRefuseADialogOfAnUnknownKind(t *testing.T) {
	t.Parallel()
	taskID, err := models.ParseTaskID("0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b")
	require.NoError(t, err)
	event := models.TaskEvent{
		TaskID: taskID,
		Kind: models.AgentEventKind{Event: models.DialogRequested{Request: models.DialogRequest{
			ID:    "d1",
			Title: "¿Seguir?",
			Kind:  unknownDialogKind{},
		}}},
	}

	mapped, err := TranscriptEventsFromDomain([]models.TaskEvent{event})

	require.Error(t, err)
	assert.Nil(t, mapped)
}
