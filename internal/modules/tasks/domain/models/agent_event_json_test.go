package models_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

func TestDeltasAreEphemeralAndMessagesPersistent(t *testing.T) {
	t.Parallel()
	assert.False(t, models.TextDelta{Delta: "x"}.IsPersistent())
	assert.True(t, models.AssistantMessage{Text: "x"}.IsPersistent())
	assert.True(t, models.RunSettled{}.IsPersistent())
}

func TestAgentEventSerializesWithTypeTag(t *testing.T) {
	t.Parallel()
	data, err := models.EncodeAgentEvent(models.RunStarted{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"run_started"}`, string(data))
}

func TestAgentEventNullsAndEmptyListsMatchSerde(t *testing.T) {
	t.Parallel()
	data, err := models.EncodeAgentEvent(models.QueueChanged{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"queue_changed","steering":[],"follow_up":[]}`, string(data))

	data, err = models.EncodeAgentEvent(models.ProcessExited{})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"process_exited","code":null}`, string(data))
}

func TestDialogRequestedUsesFlatSerdeShape(t *testing.T) {
	t.Parallel()
	message := "¿seguro?"
	timeout := uint64(1500)
	data, err := models.EncodeAgentEvent(models.DialogRequested{Request: models.DialogRequest{
		ID:        "d1",
		Title:     "t",
		Kind:      models.ConfirmDialog{Message: &message},
		TimeoutMS: &timeout,
	}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"dialog_requested","id":"d1","title":"t","kind":{"kind":"confirm","message":"¿seguro?"},"timeout_ms":1500}`, string(data))
}

func TestAgentEventRoundTripsEveryVariant(t *testing.T) {
	t.Parallel()
	code := int32(3)
	text := "x"
	resets := int64(1784283600)
	lines := []string{"a"}
	events := []models.AgentEvent{
		models.RunStarted{},
		models.RunSettled{},
		models.UserMessage{Text: "hola", Images: 2},
		models.AssistantMessage{Text: "ok", Thinking: &text},
		models.TextDelta{Delta: "d"},
		models.ThinkingDelta{Delta: "t"},
		models.ToolStarted{CallID: "c", ToolName: "bash", Args: "{}"},
		models.ToolUpdated{CallID: "c", Output: "o"},
		models.ToolFinished{CallID: "c", ToolName: "bash", Output: "o", IsError: true},
		models.QueueChanged{Steering: []string{"s"}, FollowUp: []string{}},
		models.DialogRequested{Request: models.DialogRequest{ID: "d", Title: "t", Kind: models.SelectDialog{Options: []string{"a", "b"}}}},
		models.DialogRequested{Request: models.DialogRequest{ID: "e", Title: "t", Kind: models.InputDialog{}}},
		models.DialogResolved{DialogID: "d"},
		models.Notice{Level: models.NoticeWarning, Message: "m"},
		models.StatusChanged{Key: "k", Text: &text},
		models.WidgetChanged{Key: "k", Lines: &lines},
		models.SubagentsChanged{Active: 2, Waiting: 1},
		models.UsageLimitReached{ResetsAt: &resets, Message: "límite"},
		models.ProcessExited{Code: &code},
	}
	for _, event := range events {
		data, err := models.EncodeAgentEvent(event)
		require.NoError(t, err)
		decoded, err := models.DecodeAgentEvent(data)
		require.NoError(t, err)
		assert.Equal(t, event, decoded)
	}
}

func TestDecodeAgentEventDefaultsMissingImageCountToZero(t *testing.T) {
	t.Parallel()
	event, err := models.DecodeAgentEvent([]byte(`{"type":"user_message","text":"hola"}`))
	require.NoError(t, err)
	assert.Equal(t, models.UserMessage{Text: "hola"}, event)
}

func TestDecodeAgentEventRejectsUnknownType(t *testing.T) {
	t.Parallel()
	_, err := models.DecodeAgentEvent([]byte(`{"type":"nope"}`))
	assert.Error(t, err)
}

func TestTaskEventKindRoundTripsAndKeepsSourceTag(t *testing.T) {
	t.Parallel()
	message := "boom"
	kinds := []models.TaskEventKind{
		models.AgentEventKind{Event: models.RunStarted{}},
		models.StatusEventKind{Status: models.StatusFailed, Error: &message},
		models.StatusEventKind{Status: models.StatusRunning},
		models.ChangedEventKind{},
		models.RemovedEventKind{},
	}
	for _, kind := range kinds {
		data, err := models.EncodeTaskEventKind(kind)
		require.NoError(t, err)
		decoded, err := models.DecodeTaskEventKind(data)
		require.NoError(t, err)
		assert.Equal(t, kind, decoded)
	}

	data, err := models.EncodeTaskEventKind(models.StatusEventKind{Status: models.StatusRunning})
	require.NoError(t, err)
	assert.JSONEq(t, `{"source":"status","status":"running","error":null}`, string(data))

	data, err = models.EncodeTaskEventKind(models.AgentEventKind{Event: models.RunStarted{}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"source":"agent","event":{"type":"run_started"}}`, string(data))
}

func TestOnlyChangedAndRemovedAreEphemeralTaskEventKinds(t *testing.T) {
	t.Parallel()
	assert.False(t, models.ChangedEventKind{}.IsPersistent())
	assert.False(t, models.RemovedEventKind{}.IsPersistent())
	assert.True(t, models.StatusKind(models.StatusIdle, nil).IsPersistent())
	assert.False(t, models.AgentEventKind{Event: models.TextDelta{}}.IsPersistent())
}
