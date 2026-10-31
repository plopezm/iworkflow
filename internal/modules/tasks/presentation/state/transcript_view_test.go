package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Rule: the history is folded in order; an event that is not persisted (a text delta) still shows as the text being
// streamed.
func TestFoldTranscriptFoldsTheHistoryInOrder(t *testing.T) {
	t.Parallel()
	first, second := int64(4), int64(9)
	history := []dto.TranscriptEventDTO{
		{Seq: &first, Agent: dto.TranscriptUserMessage{Text: "hola"}},
		{Seq: &second, Agent: dto.TranscriptAssistantMessage{Text: "eco"}},
		{Agent: dto.TranscriptTextDelta{Delta: "no persistido"}},
	}

	transcript := state.FoldTranscript(history)

	assert.Len(t, transcript.Items, 2)
	assert.Equal(t, "no persistido", transcript.StreamingText)
}

func TestFoldTranscriptOfAnEmptyHistoryIsEmpty(t *testing.T) {
	t.Parallel()

	assert.Empty(t, state.FoldTranscript(nil).Items)
}

// Rule: a task view shares the page stream: it refreshes on the activity of its own task only, and when the stream
// reconnects.
func TestActivityTriggerListensToItsOwnTaskAndToReconnections(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "sse:activity[event.detail.data=='t-1'] delay:300ms, sse:reset delay:300ms", state.ActivityTrigger("t-1"))
	assert.Equal(t, "sse:activity[event.detail.data=='t-1'] from:body delay:300ms, sse:reset from:body delay:300ms", state.BodyActivityTrigger("t-1"))
}

func TestTranscriptPathAnswersTheBodyOfTheTask(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/tasks/t-1/transcript", state.TranscriptPath("t-1"))
}
