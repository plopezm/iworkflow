package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// Port of the Rust tests of src/modules/tasks/presentation/state/diff_report_state.rs. The Go screen state has
// no signals or timers: the phase is derived from the request, the answer of the transcript and the error of the
// send, and the render gate decides whether the content of a complete report is ready to be rendered.

func TestIdleWhenNothingWasRequested(t *testing.T) {
	t.Parallel()

	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseIdle}, state.PhaseFor(state.ReportRequestNone, nil, nil))
}

func TestPlainReportIgnoresTheExplanationAndTheError(t *testing.T) {
	t.Parallel()

	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhasePlain}, state.PhaseFor(state.ReportRequestPlain, strPtr("x"), strPtr("e")))
}

func TestExplainedWaitsUntilTheAgentAnswers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseWaiting}, state.PhaseFor(state.ReportRequestExplained, nil, nil))
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseReady, Explanation: "texto"}, state.PhaseFor(state.ReportRequestExplained, strPtr("texto"), nil))
}

func TestExplainedFailsOnSendErrorButAnAnswerWins(t *testing.T) {
	t.Parallel()

	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseFailed, Error: "boom"}, state.PhaseFor(state.ReportRequestExplained, nil, strPtr("boom")))
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseReady, Explanation: "texto"}, state.PhaseFor(state.ReportRequestExplained, strPtr("texto"), strPtr("boom")))
}

func TestOnlyTheWaitingPhaseIsWaiting(t *testing.T) {
	t.Parallel()

	assert.True(t, state.ReportPhase{Kind: state.ReportPhaseWaiting}.IsWaiting())
	assert.False(t, state.ReportPhase{Kind: state.ReportPhaseIdle}.IsWaiting())
	assert.False(t, state.ReportPhase{Kind: state.ReportPhasePlain}.IsWaiting())
	assert.False(t, state.ReportPhase{Kind: state.ReportPhaseReady, Explanation: "x"}.IsWaiting())
	assert.False(t, state.ReportPhase{Kind: state.ReportPhaseFailed, Error: "e"}.IsWaiting())
}

func TestCompleteReportsWaitForTheRenderGate(t *testing.T) {
	t.Parallel()

	ready := state.ReportPhase{Kind: state.ReportPhaseReady, Explanation: "x"}
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhasePreparing}, state.GateRender(state.ReportPhase{Kind: state.ReportPhasePlain}, false))
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhasePreparing}, state.GateRender(ready, false))
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhasePlain}, state.GateRender(state.ReportPhase{Kind: state.ReportPhasePlain}, true))
	assert.Equal(t, ready, state.GateRender(ready, true))
}

func TestTheRenderGateLeavesTheIncompletePhasesAlone(t *testing.T) {
	t.Parallel()

	for _, phase := range []state.ReportPhase{
		state.ReportPhase{Kind: state.ReportPhaseIdle},
		state.ReportPhase{Kind: state.ReportPhaseWaiting},
		state.ReportPhase{Kind: state.ReportPhaseFailed, Error: "e"},
	} {
		assert.Equal(t, phase, state.GateRender(phase, false))
	}
}

func TestBusyCoversWaitingAndPreparingOnly(t *testing.T) {
	t.Parallel()

	assert.True(t, state.ReportPhase{Kind: state.ReportPhasePreparing}.IsBusy())
	assert.True(t, state.ReportPhase{Kind: state.ReportPhaseWaiting}.IsBusy())
	assert.False(t, state.ReportPhase{Kind: state.ReportPhasePlain}.IsBusy())
	assert.False(t, state.ReportPhase{Kind: state.ReportPhaseReady, Explanation: "x"}.IsBusy())
	assert.False(t, state.ReportPhase{Kind: state.ReportPhaseIdle}.IsBusy())
}

func TestAnOpenPlainReportIsRenderedOnceTheGateOpens(t *testing.T) {
	t.Parallel()

	report := state.DiffReport{TaskID: "task-1", Request: state.ReportRequestPlain}
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhasePlain}, report.Phase())
}

func TestClosingAnOpenReportReturnsToIdle(t *testing.T) {
	t.Parallel()

	report := state.DiffReport{TaskID: "task-1", Request: state.ReportRequestPlain}.Close()
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseIdle}, report.Phase())
}

func TestAskingForAnExplanationWaitsForTheAgentAnswer(t *testing.T) {
	t.Parallel()

	report := state.DiffReport{TaskID: "task-1", Transcript: state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: state.ExplanationPrompt},
	}}}.RequestExplanation(state.Transcript{Items: []state.TranscriptItem{
		state.UserItem{Text: state.ExplanationPrompt},
	}})

	assert.Equal(t, state.ReportRequestExplained, report.Request)
	assert.Equal(t, 1, report.Since)
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseWaiting}, report.Phase())
}

func TestTheSendErrorOfTheLastDispatchIsRead(t *testing.T) {
	t.Parallel()

	assert.Nil(t, state.SendErrorSince(3, 3, strPtr("failed")))
	assert.Nil(t, state.SendErrorSince(2, 3, strPtr("failed")))
	assert.Nil(t, state.SendErrorSince(4, 3, nil))
	failed := strPtr("failed")
	assert.NotNil(t, state.SendErrorSince(4, 3, failed))
	assert.Contains(t, *state.SendErrorSince(4, 3, failed), "failed")
}

func TestAnAnswerMovesTheReportToReady(t *testing.T) {
	t.Parallel()

	report := state.DiffReport{
		TaskID:  "task-1",
		Request: state.ReportRequestExplained,
		Since:   0,
		Transcript: state.Transcript{Items: []state.TranscriptItem{
			state.UserItem{Text: state.ExplanationPrompt},
			state.AssistantItem{Text: "Explicación"},
		}},
	}
	assert.Equal(t, state.ReportPhase{Kind: state.ReportPhaseReady, Explanation: "Explicación"}, report.Phase())
}
