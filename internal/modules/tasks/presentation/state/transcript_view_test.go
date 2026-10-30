package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Rule: the history is folded in order, and the cursor of the live stream is the last sequence number that was
// persisted; events without a sequence number never move it.
func TestFoldTranscriptReturnsTheLastPersistedSequenceAsCursor(t *testing.T) {
	t.Parallel()
	first, second := int64(4), int64(9)
	history := []dto.TranscriptEventDTO{
		{Seq: &first, Agent: dto.TranscriptUserMessage{Text: "hola"}},
		{Seq: &second, Agent: dto.TranscriptAssistantMessage{Text: "eco"}},
		{Agent: dto.TranscriptTextDelta{Delta: "no persistido"}},
	}

	transcript, cursor := state.FoldTranscript(history)

	require.NotNil(t, cursor)
	assert.Equal(t, int64(9), *cursor)
	assert.Len(t, transcript.Items, 2)
	assert.Equal(t, "no persistido", transcript.StreamingText)
}

func TestFoldTranscriptOfAnEmptyHistoryHasNoCursor(t *testing.T) {
	t.Parallel()

	transcript, cursor := state.FoldTranscript(nil)

	assert.Nil(t, cursor)
	assert.Empty(t, transcript.Items)
}

// Rule: the live stream starts after the cursor, so the history the page shows is not sent twice; without a cursor it
// starts from the beginning.
func TestTaskEventsPathStartsAfterTheCursor(t *testing.T) {
	t.Parallel()
	seq := int64(12)

	assert.Equal(t, "/api/tasks/t-1/events", state.TaskEventsPath("t-1", nil))
	assert.Equal(t, "/api/tasks/t-1/events?after=12", state.TaskEventsPath("t-1", &seq))
}

func TestTranscriptPathAnswersTheBodyOfTheTask(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/tasks/t-1/transcript", state.TranscriptPath("t-1"))
}
