package dto_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

var transcriptNow = time.Date(2026, 10, 25, 9, 0, 0, 0, time.UTC)

func persistedAgent(seq int64, event models.AgentEvent) models.TaskEvent {
	id := seq
	return models.TaskEvent{TaskID: models.TaskID{}, Seq: &id, Kind: models.AgentEventKind{Event: event}, CreatedAt: transcriptNow}
}

// Rule: board notices ("changed" and "removed") are not part of a transcript, so they are left out; the sequence
// numbers of the rest are kept in order.
func TestTranscriptEventsLeaveOutBoardNoticesAndKeepSequence(t *testing.T) {
	t.Parallel()
	first := int64(1)
	events := []models.TaskEvent{
		{Seq: &first, Kind: models.ChangedEventKind{}, CreatedAt: transcriptNow},
		persistedAgent(2, models.RunStarted{}),
		{Kind: models.RemovedEventKind{}, CreatedAt: transcriptNow},
		persistedAgent(3, models.RunSettled{}),
	}

	mapped, err := dto.TranscriptEventsFromDomain(events)

	require.NoError(t, err)
	require.Len(t, mapped, 2)
	assert.Equal(t, int64(2), *mapped[0].Seq)
	assert.IsType(t, dto.TranscriptRunStarted{}, mapped[0].Agent)
	assert.Equal(t, int64(3), *mapped[1].Seq)
	assert.IsType(t, dto.TranscriptRunSettled{}, mapped[1].Agent)
}

// Rule: the live-agent flag is the domain's decision, carried with the status, so the transcript closes orphan
// dialogs the same way the Rust version does.
func TestTranscriptStatusCarriesTheDomainLiveAgentFlag(t *testing.T) {
	t.Parallel()
	seq := int64(7)
	events := []models.TaskEvent{
		{Seq: &seq, Kind: models.StatusEventKind{Status: models.StatusRunning}, CreatedAt: transcriptNow},
		{Seq: &seq, Kind: models.StatusEventKind{Status: models.StatusAwaitingReview}, CreatedAt: transcriptNow},
	}

	mapped, err := dto.TranscriptEventsFromDomain(events)

	require.NoError(t, err)
	require.Len(t, mapped, 2)
	assert.Equal(t, &dto.TranscriptStatusDTO{Status: "running", HasLiveAgent: models.StatusRunning.HasLiveAgent()}, mapped[0].Status)
	assert.True(t, mapped[0].Status.HasLiveAgent)
	assert.False(t, mapped[1].Status.HasLiveAgent)
}

// Rule: a widget update without lines is a clear, so the mapping keeps the nil list (an empty list would not).
func TestTranscriptWidgetWithoutLinesIsAClear(t *testing.T) {
	t.Parallel()
	events := []models.TaskEvent{
		persistedAgent(1, models.WidgetChanged{Key: "todo", Lines: nil}),
		persistedAgent(2, models.WidgetChanged{Key: "todo", Lines: &[]string{"- a"}}),
	}

	mapped, err := dto.TranscriptEventsFromDomain(events)

	require.NoError(t, err)
	require.Len(t, mapped, 2)
	assert.Equal(t, dto.TranscriptWidgetChanged{Key: "todo", Lines: nil}, mapped[0].Agent)
	assert.Equal(t, dto.TranscriptWidgetChanged{Key: "todo", Lines: []string{"- a"}}, mapped[1].Agent)
}
