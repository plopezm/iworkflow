package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// Port of the Rust tests of src/modules/tasks/presentation/state/transcript.rs.

func transcriptAgent(event dto.TranscriptAgentEvent) dto.TranscriptEventDTO {
	return dto.TranscriptEventDTO{Agent: event}
}

func transcriptStatus(status string, live bool) dto.TranscriptEventDTO {
	return dto.TranscriptEventDTO{Status: &dto.TranscriptStatusDTO{Status: status, HasLiveAgent: live}}
}

func transcriptDialog(id string) dto.TranscriptAgentEvent {
	return dto.TranscriptDialogRequested{Request: dto.TranscriptDialog{ID: id, Title: "¿Seguir?"}}
}

func strPtr(value string) *string { return &value }

func TestTranscriptUserMessageKeepsTheImageCount(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptAgent(dto.TranscriptUserMessage{Text: "", Images: 2}))

	assert.Equal(t, []state.TranscriptItem{state.UserItem{Text: "", Images: 2}}, transcript.Items)
}

func TestTranscriptSubagentsChangedBumpsTheVersionAndCountsActiveAndWaiting(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptAgent(dto.TranscriptSubagentsChanged{Active: 2, Waiting: 1}))
	assert.Equal(t, uint32(1), transcript.SubagentsVersion)
	assert.Equal(t, uint32(3), transcript.SubagentsActive)

	transcript.Apply(transcriptAgent(dto.TranscriptSubagentsChanged{Active: 0, Waiting: 0}))
	assert.Equal(t, uint32(2), transcript.SubagentsVersion)
	assert.Equal(t, uint32(0), transcript.SubagentsActive)
}

func TestTranscriptOtherAgentEventsLeaveSubagentsUntouched(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptAgent(dto.TranscriptRunStarted{}))

	assert.Equal(t, uint32(0), transcript.SubagentsVersion)
}

func TestTranscriptStreamingTextIsReplacedByTheFinalMessage(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptAgent(dto.TranscriptTextDelta{Delta: "Ho"}))
	transcript.Apply(transcriptAgent(dto.TranscriptTextDelta{Delta: "la"}))
	assert.Equal(t, "Hola", transcript.StreamingText)

	transcript.Apply(transcriptAgent(dto.TranscriptAssistantMessage{Text: "Hola"}))

	assert.False(t, transcript.IsStreaming())
	assert.Len(t, transcript.Items, 1)
}

func TestTranscriptToolLifecycleUpdatesASingleItem(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptAgent(dto.TranscriptToolStarted{CallID: "c1", ToolName: "bash", Args: "{}"}))
	transcript.Apply(transcriptAgent(dto.TranscriptToolUpdated{CallID: "c1", Output: "parcial"}))
	transcript.Apply(transcriptAgent(dto.TranscriptToolFinished{CallID: "c1", ToolName: "bash", Output: "final", IsError: false}))

	isError := false
	assert.Equal(t, []state.TranscriptItem{state.ToolItem{CallID: "c1", Name: "bash", Args: "{}", Output: "final", IsError: &isError}}, transcript.Items)
}

func TestTranscriptDialogsArePendingUntilResolvedOrProcessExit(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript
	transcript.Apply(transcriptAgent(transcriptDialog("d1")))
	transcript.Apply(transcriptAgent(transcriptDialog("d2")))
	pending, ok := transcript.PendingDialog()
	require.True(t, ok)
	assert.Equal(t, "d2", pending.ID)

	transcript.Apply(transcriptAgent(dto.TranscriptDialogResolved{DialogID: "d2"}))
	pending, ok = transcript.PendingDialog()
	require.True(t, ok)
	assert.Equal(t, "d1", pending.ID)

	transcript.Apply(transcriptAgent(dto.TranscriptProcessExited{Code: nil}))
	_, ok = transcript.PendingDialog()
	assert.False(t, ok)
}

func TestTranscriptStatusWithoutLiveAgentClosesOrphanDialogs(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript
	transcript.Apply(transcriptAgent(transcriptDialog("d1")))

	transcript.Apply(transcriptStatus("running", true))
	_, open := transcript.PendingDialog()
	assert.True(t, open)

	transcript.Apply(transcriptStatus("awaiting_review", false))
	_, open = transcript.PendingDialog()
	assert.False(t, open)
}

// A dialog the turn leaves open is closed by the turn, whichever fact ends the turn: the transcript derives it.
func TestTranscriptDialogLeftOpenWhenTheTurnEndsIsClosedByTheTurn(t *testing.T) {
	t.Parallel()
	cases := map[string]dto.TranscriptEventDTO{
		"process exit":              transcriptAgent(dto.TranscriptProcessExited{Code: nil}),
		"status without live agent": transcriptStatus("interrupted", false),
	}
	for name, endOfTurn := range cases {
		transcript, _ := state.FoldTranscript([]dto.TranscriptEventDTO{
			transcriptAgent(transcriptDialog("d1")),
			endOfTurn,
		})
		dialog, found := transcript.Dialog("d1")
		require.True(t, found, name)
		assert.True(t, dialog.Resolved, name)
		assert.True(t, dialog.ClosedByTurn, name)
	}
}

// A dialog answered before the turn ends keeps its answer: the turn does not close it.
func TestTranscriptDialogAnsweredBeforeTheTurnEndsIsNotClosedByTheTurn(t *testing.T) {
	t.Parallel()
	transcript, _ := state.FoldTranscript([]dto.TranscriptEventDTO{
		transcriptAgent(transcriptDialog("d1")),
		transcriptAgent(dto.TranscriptDialogResolved{DialogID: "d1"}),
		transcriptAgent(dto.TranscriptProcessExited{Code: nil}),
	})
	dialog, found := transcript.Dialog("d1")
	require.True(t, found)
	assert.True(t, dialog.Resolved)
	assert.False(t, dialog.ClosedByTurn)
}

func TestTranscriptLiveStatusesKeepDialogsOpenAndOthersCloseThem(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"starting", "running", "idle"} {
		var transcript state.Transcript
		transcript.Apply(transcriptAgent(transcriptDialog("d1")))
		transcript.Apply(transcriptStatus(status, true))
		_, open := transcript.PendingDialog()
		assert.True(t, open, status)
	}

	for _, status := range []string{"interrupted", "failed", "finishing"} {
		var transcript state.Transcript
		transcript.Apply(transcriptAgent(transcriptDialog("d1")))
		transcript.Apply(transcriptStatus(status, false))
		_, open := transcript.PendingDialog()
		assert.False(t, open, status)
		// The dialog stays in the transcript, marked as resolved, and the status comes after it.
		require.Equal(t, state.DialogItem{Request: dto.TranscriptDialog{ID: "d1", Title: "¿Seguir?"}, Resolved: true, ClosedByTurn: true}, transcript.Items[0], status)
		assert.IsType(t, state.StatusItem{}, transcript.Items[len(transcript.Items)-1], status)
	}
}

func TestTranscriptNonLiveStatusWithoutDialogOnlyRecordsTheStatus(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptStatus("awaiting_review", false))

	_, open := transcript.PendingDialog()
	assert.False(t, open)
	assert.Len(t, transcript.Items, 1)
}

func TestTranscriptExtensionStatusAndWidgetsCanBeCleared(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript
	transcript.Apply(transcriptAgent(dto.TranscriptStatusChanged{Key: "mcp", Text: strPtr("ok")}))
	transcript.Apply(transcriptAgent(dto.TranscriptWidgetChanged{Key: "todo", Lines: []string{"- a"}}))
	assert.Equal(t, "ok", transcript.Statuses["mcp"])

	transcript.Apply(transcriptAgent(dto.TranscriptStatusChanged{Key: "mcp", Text: nil}))
	transcript.Apply(transcriptAgent(dto.TranscriptWidgetChanged{Key: "todo", Lines: nil}))

	assert.Empty(t, transcript.Statuses)
	assert.Empty(t, transcript.Widgets)
}

func TestTranscriptStatusEventsAreListedInOrder(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptStatus("running", true))

	assert.Equal(t, []state.TranscriptItem{state.StatusItem{Status: "running"}}, transcript.Items)
}

// Rule: a process exit ends the streaming text, closes the dialogs, empties the queues and the extension lines, and
// records the exit code in the transcript.
func TestTranscriptProcessExitClosesWhatTheAgentLeftOpen(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript
	code := int32(3)
	transcript.Apply(transcriptAgent(dto.TranscriptTextDelta{Delta: "a medias"}))
	transcript.Apply(transcriptAgent(transcriptDialog("d1")))
	transcript.Apply(transcriptAgent(dto.TranscriptQueueChanged{Steering: []string{"x"}, FollowUp: []string{"y"}}))
	transcript.Apply(transcriptAgent(dto.TranscriptStatusChanged{Key: "mcp", Text: strPtr("ok")}))

	transcript.Apply(transcriptAgent(dto.TranscriptProcessExited{Code: &code}))

	assert.False(t, transcript.IsStreaming())
	_, open := transcript.PendingDialog()
	assert.False(t, open)
	assert.Empty(t, transcript.QueuedSteering)
	assert.Empty(t, transcript.QueuedFollowUp)
	assert.Empty(t, transcript.Statuses)
	assert.Equal(t, state.ExitItem{Code: &code}, transcript.Items[len(transcript.Items)-1])
}

// Rule: a usage limit is shown as a warning notice, and the subagents total never overflows.
func TestTranscriptUsageLimitIsAWarningNoticeAndSubagentsSaturate(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptAgent(dto.TranscriptUsageLimitReached{Message: "límite"}))
	transcript.Apply(transcriptAgent(dto.TranscriptSubagentsChanged{Active: ^uint32(0), Waiting: 5}))

	assert.Equal(t, []state.TranscriptItem{state.NoticeItem{Level: dto.NoticeLevelWarning, Message: "límite"}}, transcript.Items)
	assert.Equal(t, ^uint32(0), transcript.SubagentsActive)
}

// Triangulation: a tool that finishes without a start (its start was not in the history) still shows up, once.
func TestTranscriptToolFinishedWithoutStartAddsTheTool(t *testing.T) {
	t.Parallel()
	var transcript state.Transcript

	transcript.Apply(transcriptAgent(dto.TranscriptToolFinished{CallID: "c9", ToolName: "read", Output: "ok", IsError: true}))

	isError := true
	assert.Equal(t, []state.TranscriptItem{state.ToolItem{CallID: "c9", Name: "read", Output: "ok", IsError: &isError}}, transcript.Items)
}
