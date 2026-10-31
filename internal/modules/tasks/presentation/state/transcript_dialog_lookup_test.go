package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Dialog finds a dialog by its id whether it is answered or not; the latest one wins when an id repeats.
func TestTranscriptDialogFindsTheLatestDialogWithTheID(t *testing.T) {
	t.Parallel()
	request := func(id, title string) dto.TranscriptEventDTO {
		return dto.TranscriptEventDTO{Agent: dto.TranscriptDialogRequested{Request: dto.TranscriptDialog{ID: id, Title: title}}}
	}
	transcript := state.FoldTranscript([]dto.TranscriptEventDTO{
		request("d1", "primero"),
		{Agent: dto.TranscriptDialogResolved{DialogID: "d1"}},
		request("d2", "otro"),
		request("d1", "segundo"),
	})

	dialog, found := transcript.Dialog("d1")
	assert.True(t, found)
	assert.Equal(t, "segundo", dialog.Request.Title)
	assert.False(t, dialog.Resolved)

	answered, found := transcript.Dialog("d2")
	assert.True(t, found)
	assert.False(t, answered.Resolved)

	_, found = transcript.Dialog("gone")
	assert.False(t, found)
}
