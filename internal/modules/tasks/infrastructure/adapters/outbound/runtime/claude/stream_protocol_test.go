package claude

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// testTaskID returns a fresh task id for a decode call. The protocol ignores it.
func testTaskID(t *testing.T) models.TaskID {
	t.Helper()
	taskID, err := models.NewTaskID()
	require.NoError(t, err)
	return taskID
}

// decodeLines feeds the records to the protocol as stdout lines, in order, and returns every event.
func decodeLines(t *testing.T, p *streamProtocol, records ...map[string]any) []models.AgentEvent {
	t.Helper()
	taskID := testTaskID(t)
	var events []models.AgentEvent
	for _, record := range records {
		raw, err := json.Marshal(record)
		require.NoError(t, err)
		events = append(events, p.Decode(taskID, string(raw))...)
	}
	return events
}

// mustEncode encodes a command that must produce a line and returns it.
func mustEncode(t *testing.T, p *streamProtocol, command models.AgentCommand, requestID string) string {
	t.Helper()
	line, ok, err := p.Encode(command, requestID)
	require.NoError(t, err)
	require.True(t, ok, "el comando debía producir una línea")
	return line
}

// parseLine decodes an encoded line back to a generic JSON object, the way the agent reads it.
func parseLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &record), line)
	return record
}

func streamDelta(delta map[string]any) map[string]any {
	return map[string]any{
		"type": "stream_event", "parent_tool_use_id": nil,
		"event": map[string]any{"type": "content_block_delta", "index": 0, "delta": delta},
	}
}

func thinkingDelta(text string) map[string]any {
	return streamDelta(map[string]any{"type": "thinking_delta", "thinking": text})
}

func resultLine(subtype string, isError bool) map[string]any {
	return map[string]any{"type": "result", "subtype": subtype, "is_error": isError, "result": "boom"}
}

func assistantBlocks(blocks ...map[string]any) map[string]any {
	content := make([]any, 0, len(blocks))
	for _, block := range blocks {
		content = append(content, block)
	}
	return map[string]any{"type": "assistant", "message": map[string]any{"content": content}}
}

func controlRequest(requestID, tool string, input map[string]any) map[string]any {
	return map[string]any{"type": "control_request", "request_id": requestID, "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": tool, "display_name": tool, "input": input}}
}

// requestPermission makes the protocol hold a permission request r1 for a Bash command.
func requestPermission(t *testing.T, p *streamProtocol) {
	t.Helper()
	decodeLines(t, p, map[string]any{"type": "control_request", "request_id": "r1", "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": "Bash", "input": map[string]any{"command": "ls"}}})
}

func twoQuestions() map[string]any {
	return map[string]any{"questions": []any{
		map[string]any{"question": "¿Rojo o azul?", "header": "Color", "multiSelect": false, "options": []any{
			map[string]any{"label": "Rojo", "description": "Elegir rojo"},
			map[string]any{"label": "Azul", "description": ""},
		}},
		map[string]any{"question": "¿Cuántos?", "header": "", "multiSelect": false, "options": []any{
			map[string]any{"label": "Uno", "description": ""},
			map[string]any{"label": "Dos", "description": ""},
		}},
	}}
}

func answerCommand(dialogID string, answer models.DialogAnswer) models.AnswerDialogCommand {
	return models.AnswerDialogCommand{DialogID: dialogID, Answer: answer}
}

func valueAnswer(text string) models.ValueAnswer {
	return models.ValueAnswer{Value: text}
}

func confirmedAnswer(confirmed bool) models.ConfirmedAnswer {
	return models.ConfirmedAnswer{Confirmed: confirmed}
}

func TestStreamProtocolTextAndThinkingDeltasStartTheRunOnce(t *testing.T) {
	// Rust: stream_protocol::text_and_thinking_deltas_start_the_run_once
	p := newStreamProtocol()
	events := decodeLines(t, p,
		streamDelta(map[string]any{"type": "text_delta", "text": "ho"}),
		streamDelta(map[string]any{"type": "thinking_delta", "thinking": "mm"}),
	)
	require.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.TextDelta{Delta: "ho"},
		models.ThinkingDelta{Delta: "mm"},
	}, events)
}

func TestStreamProtocolAssistantTextAndThinkingBecomeOneMessage(t *testing.T) {
	// Rust: stream_protocol::assistant_text_and_thinking_become_one_message
	p := newStreamProtocol()
	events := decodeLines(t, p, assistantBlocks(
		map[string]any{"type": "thinking", "thinking": "pienso"},
		map[string]any{"type": "text", "text": "hola"},
	))
	require.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.AssistantMessage{Text: "hola", Thinking: ptrTo("pienso")},
	}, events)
}

func TestStreamProtocolEmptyThinkingSignatureBlocksProduceNothing(t *testing.T) {
	// Rust: stream_protocol::empty_thinking_signature_blocks_produce_nothing
	p := newStreamProtocol()
	events := decodeLines(t, p, assistantBlocks(
		map[string]any{"type": "thinking", "thinking": "", "signature": "abc"},
	))
	require.Equal(t, []models.AgentEvent{models.RunStarted{}}, events)
}

func TestStreamProtocolStreamedThinkingIsAttachedToTheNextTextMessage(t *testing.T) {
	// Rust: stream_protocol::streamed_thinking_is_attached_to_the_next_text_message
	p := newStreamProtocol()
	events := decodeLines(t, p,
		thinkingDelta("pien"),
		thinkingDelta("so"),
		assistantBlocks(map[string]any{"type": "thinking", "thinking": "", "signature": "abc"}),
		assistantBlocks(map[string]any{"type": "text", "text": "hola"}),
		assistantBlocks(map[string]any{"type": "text", "text": "otra"}),
	)
	var messages []models.AgentEvent
	for _, event := range events {
		if _, isMessage := event.(models.AssistantMessage); isMessage {
			messages = append(messages, event)
		}
	}
	require.Equal(t, []models.AgentEvent{
		models.AssistantMessage{Text: "hola", Thinking: ptrTo("pienso")},
		models.AssistantMessage{Text: "otra"},
	}, messages)
}

func TestStreamProtocolStreamedThinkingBeforeAToolCallIsEmittedBeforeTheTool(t *testing.T) {
	// Rust: stream_protocol::streamed_thinking_before_a_tool_call_is_emitted_before_the_tool
	p := newStreamProtocol()
	events := decodeLines(t, p,
		thinkingDelta("voy a listar"),
		assistantBlocks(map[string]any{"type": "tool_use", "id": "t1", "name": "Bash", "input": map[string]any{}}),
	)
	require.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.ThinkingDelta{Delta: "voy a listar"},
		models.AssistantMessage{Thinking: ptrTo("voy a listar")},
		models.ToolStarted{CallID: "t1", ToolName: "Bash", Args: "{}"},
	}, events)
}

func TestStreamProtocolAFinishedRunForgetsItsPendingPermissionsAndThinking(t *testing.T) {
	// Rust: stream_protocol::a_finished_run_forgets_its_pending_permissions_and_thinking
	p := newStreamProtocol()
	requestPermission(t, p)
	decodeLines(t, p, thinkingDelta("sin usar"), resultLine("success", false))

	_, ok, err := p.Encode(answerCommand("r1", confirmedAnswer(true)), "x")
	require.ErrorIs(t, err, werrors.Runtime("no hay un permiso pendiente con id r1"))
	assert.False(t, ok)

	next := decodeLines(t, p, assistantBlocks(map[string]any{"type": "text", "text": "nuevo"}))
	require.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.AssistantMessage{Text: "nuevo"},
	}, next)
}

func TestStreamProtocolAFinishedRunResolvesTheDialogsTheAgentNoLongerAwaits(t *testing.T) {
	// Rust: stream_protocol::a_finished_run_resolves_the_dialogs_the_agent_no_longer_awaits
	// Divergence from Rust: the run settles first and the dialogs it left open are resolved after it, as the cursor
	// runtime does. The transcript closes the open dialogs when the run settles, so a dialog the agent abandoned is shown
	// as closed by the turn; resolved before the settle it would be shown as answered by the user.
	p := newStreamProtocol()
	requestPermission(t, p)

	events := decodeLines(t, p, resultLine("success", false))
	require.Equal(t, []models.AgentEvent{
		models.RunSettled{},
		models.DialogResolved{DialogID: "r1"},
	}, events)
}

func TestStreamProtocolToolUseAndResultArePairedByID(t *testing.T) {
	// Rust: stream_protocol::tool_use_and_result_are_paired_by_id
	p := newStreamProtocol()
	events := decodeLines(t, p,
		assistantBlocks(map[string]any{"type": "tool_use", "id": "t1", "name": "Bash",
			"input": map[string]any{"command": "ls"}}),
		map[string]any{"type": "user", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "t1", "content": "a.txt", "is_error": false},
		}}},
	)
	require.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.ToolStarted{CallID: "t1", ToolName: "Bash", Args: `{"command":"ls"}`},
		models.ToolFinished{CallID: "t1", ToolName: "Bash", Output: "a.txt", IsError: false},
	}, events)
}

func TestStreamProtocolToolResultBlocksListIsJoinedAndErrorFlagKept(t *testing.T) {
	// Rust: stream_protocol::tool_result_blocks_list_is_joined_and_error_flag_kept
	p := newStreamProtocol()
	events := decodeLines(t, p, map[string]any{"type": "user", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": "x", "is_error": true, "content": []any{
			map[string]any{"type": "text", "text": "uno"},
			map[string]any{"type": "text", "text": "dos"},
		}},
	}}})
	require.Equal(t, []models.AgentEvent{
		models.ToolFinished{CallID: "x", ToolName: "", Output: "uno\ndos", IsError: true},
	}, events)
}

func TestStreamProtocolReplayedUserMessageIsEmittedButPlainStdinEchoIsNot(t *testing.T) {
	// Rust: stream_protocol::replayed_user_message_is_emitted_but_plain_stdin_echo_is_not
	p := newStreamProtocol()
	events := decodeLines(t, p,
		map[string]any{"type": "user", "isReplay": true, "message": map[string]any{"content": "di hola"}},
		map[string]any{"type": "user", "message": map[string]any{"content": "otro"}},
	)
	require.Equal(t, []models.AgentEvent{models.UserMessage{Text: "di hola", Images: 0}}, events)
}

func TestStreamProtocolSubagentLinesAreIgnored(t *testing.T) {
	// Rust: stream_protocol::subagent_lines_are_ignored
	p := newStreamProtocol()
	events := decodeLines(t, p, map[string]any{"type": "assistant", "parent_tool_use_id": "p1",
		"message": map[string]any{"content": []any{map[string]any{"type": "text", "text": "sub"}}}})
	assert.Empty(t, events)
}

func TestStreamProtocolPermissionRequestBecomesAConfirmDialog(t *testing.T) {
	// Rust: stream_protocol::permission_request_becomes_a_confirm_dialog
	p := newStreamProtocol()
	events := decodeLines(t, p, map[string]any{"type": "control_request", "request_id": "r1", "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": "Bash", "display_name": "Bash",
		"description": "Escribe a.txt", "input": map[string]any{"command": "echo hola > a.txt"}}})
	require.Equal(t, []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    "r1",
		Title: "Permitir Bash",
		Kind:  models.ConfirmDialog{Message: ptrTo("echo hola > a.txt\nEscribe a.txt")},
	}}}, events)
}

func TestStreamProtocolPermissionDialogShowsTheFileAToolWillTouch(t *testing.T) {
	// Rust: stream_protocol::permission_dialog_shows_the_file_a_tool_will_touch
	p := newStreamProtocol()
	events := decodeLines(t, p, controlRequest("r1", "Write",
		map[string]any{"file_path": "/w/a.txt", "content": "hola"}))
	require.Equal(t, []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    "r1",
		Title: "Permitir Write",
		Kind:  models.ConfirmDialog{Message: ptrTo("/w/a.txt")},
	}}}, events)
}

func TestStreamProtocolAnInventedQuestionIDDoesNotConsumeAToolPermission(t *testing.T) {
	// Rust: stream_protocol::an_invented_question_id_does_not_consume_a_tool_permission
	p := newStreamProtocol()
	requestPermission(t, p)

	for _, invented := range []string{"r1#0", "r1#5", "r1#x"} {
		_, ok, err := p.Encode(answerCommand(invented, confirmedAnswer(true)), "x")
		require.Error(t, err, invented)
		assert.False(t, ok, invented)
	}
	line := mustEncode(t, p, answerCommand("r1", confirmedAnswer(true)), "x")
	assert.Equal(t, "allow", parseLine(t, line)["response"].(map[string]any)["response"].(map[string]any)["behavior"])
}

func TestStreamProtocolAnEmptyDisplayNameFallsBackToTheToolName(t *testing.T) {
	// Rust: stream_protocol::an_empty_display_name_falls_back_to_the_tool_name
	p := newStreamProtocol()
	events := decodeLines(t, p, map[string]any{"type": "control_request", "request_id": "r1", "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": "Bash", "display_name": "",
		"input": map[string]any{"command": "ls"}}})
	require.Len(t, events, 1)
	request := events[0].(models.DialogRequested).Request
	assert.Equal(t, "Permitir Bash", request.Title)
}

func TestStreamProtocolALongCommandIsShownWholeBeforeTheModelsDescription(t *testing.T) {
	// Rust: stream_protocol::a_long_command_is_shown_whole_before_the_models_description
	p := newStreamProtocol()
	command := "echo " + strings.Repeat("x", maxSummaryChars*2) + "\nrm -rf ./build"
	events := decodeLines(t, p, map[string]any{"type": "control_request", "request_id": "r1", "request": map[string]any{
		"subtype": "can_use_tool", "tool_name": "Bash", "display_name": "Bash",
		"description": "Inofensivo", "input": map[string]any{"command": command}}})
	require.Len(t, events, 1)
	confirm := events[0].(models.DialogRequested).Request.Kind.(models.ConfirmDialog)
	require.NotNil(t, confirm.Message)
	assert.Equal(t, command+"\nInofensivo", *confirm.Message)
}

func TestStreamProtocolAskUserQuestionBecomesOneSelectDialogPerQuestion(t *testing.T) {
	// Rust: stream_protocol::ask_user_question_becomes_one_select_dialog_per_question
	p := newStreamProtocol()
	events := decodeLines(t, p, controlRequest("r1", "AskUserQuestion", twoQuestions()))
	require.Equal(t, []models.AgentEvent{
		models.DialogRequested{Request: models.DialogRequest{
			ID:    "r1#0",
			Title: "Color: ¿Rojo o azul?",
			Kind:  models.SelectDialog{Options: []string{"Rojo — Elegir rojo", "Azul"}},
		}},
		models.DialogRequested{Request: models.DialogRequest{
			ID:    "r1#1",
			Title: "¿Cuántos?",
			Kind:  models.SelectDialog{Options: []string{"Uno", "Dos"}},
		}},
	}, events)
}

func TestStreamProtocolAnswersAreSentOnceEveryQuestionHasOne(t *testing.T) {
	// Rust: stream_protocol::answers_are_sent_once_every_question_has_one
	p := newStreamProtocol()
	decodeLines(t, p, controlRequest("r1", "AskUserQuestion", twoQuestions()))

	_, partialOK, partialErr := p.Encode(answerCommand("r1#1", valueAnswer("Dos")), "x")
	require.NoError(t, partialErr)
	assert.False(t, partialOK, "la respuesta parcial no produce línea")

	line := mustEncode(t, p, answerCommand("r1#0", valueAnswer("Rojo — Elegir rojo")), "x")
	assert.Equal(t, map[string]any{"type": "control_response", "response": map[string]any{
		"subtype": "success", "request_id": "r1",
		"response": map[string]any{"behavior": "allow", "updatedInput": map[string]any{
			"questions": twoQuestions()["questions"],
			"answers":   map[string]any{"¿Rojo o azul?": "Rojo", "¿Cuántos?": "Dos"},
		}},
	}}, parseLine(t, line))
}

func TestStreamProtocolAFreeTextAnswerIsPassedThroughUnchanged(t *testing.T) {
	// Rust: stream_protocol::a_free_text_answer_is_passed_through_unchanged
	p := newStreamProtocol()
	decodeLines(t, p, controlRequest("r1", "AskUserQuestion", map[string]any{"questions": []any{
		map[string]any{"question": "¿Qué?", "options": []any{map[string]any{"label": "A", "description": ""}}},
	}}))
	line := mustEncode(t, p, answerCommand("r1#0", valueAnswer("otra cosa")), "x")
	updated := parseLine(t, line)["response"].(map[string]any)["response"].(map[string]any)["updatedInput"]
	assert.Equal(t, map[string]any{"¿Qué?": "otra cosa"}, updated.(map[string]any)["answers"])
}

func TestStreamProtocolCancellingAQuestionDeniesTheRequestAndIgnoresItsSiblings(t *testing.T) {
	// Rust: stream_protocol::cancelling_a_question_denies_the_request_and_ignores_its_siblings
	p := newStreamProtocol()
	decodeLines(t, p, controlRequest("r1", "AskUserQuestion", twoQuestions()))

	line := mustEncode(t, p, answerCommand("r1#0", models.CancelledAnswer{}), "x")
	assert.Equal(t, "deny", parseLine(t, line)["response"].(map[string]any)["response"].(map[string]any)["behavior"])

	sibling, ok, err := p.Encode(answerCommand("r1#1", valueAnswer("Uno")), "x")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, sibling)
}

func TestStreamProtocolExitPlanModeAsksToApproveThePlan(t *testing.T) {
	// Rust: stream_protocol::exit_plan_mode_asks_to_approve_the_plan
	p := newStreamProtocol()
	events := decodeLines(t, p, controlRequest("r1", "ExitPlanMode", map[string]any{"plan": "# Plan\n1. Crear a.txt"}))
	require.Equal(t, []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    "r1",
		Title: "Aprobar plan",
		Kind:  models.ConfirmDialog{Message: ptrTo("# Plan\n1. Crear a.txt")},
	}}}, events)

	line := mustEncode(t, p, answerCommand("r1", confirmedAnswer(false)), "x")
	assert.Equal(t, map[string]any{"behavior": "deny", "message": denyPlanMessage},
		parseLine(t, line)["response"].(map[string]any)["response"])
}

func TestStreamProtocolAMalformedQuestionRequestFallsBackToAPlainConfirmation(t *testing.T) {
	// Rust: stream_protocol::a_malformed_question_request_falls_back_to_a_plain_confirmation
	p := newStreamProtocol()
	events := decodeLines(t, p, controlRequest("r1", "AskUserQuestion", map[string]any{"questions": []any{}}))
	require.Equal(t, []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    "r1",
		Title: "Permitir AskUserQuestion",
		Kind:  models.ConfirmDialog{Message: ptrTo(`{"questions":[]}`)},
	}}}, events)
}

func TestStreamProtocolOtherControlRequestsAndNoiseAreIgnored(t *testing.T) {
	// Rust: stream_protocol::other_control_requests_and_noise_are_ignored
	p := newStreamProtocol()
	events := decodeLines(t, p,
		map[string]any{"type": "control_request", "request_id": "r", "request": map[string]any{"subtype": "hook_callback"}},
		map[string]any{"type": "control_response", "response": map[string]any{"subtype": "success"}},
		map[string]any{"type": "system", "subtype": "init"},
		map[string]any{"type": "rate_limit_event"},
	)
	assert.Empty(t, events)
	assert.Empty(t, p.Decode(testTaskID(t), "no es json"))
}

func TestStreamProtocolSuccessfulResultSettlesTheRunAndANewRunRestarts(t *testing.T) {
	// Rust: stream_protocol::successful_result_settles_the_run_and_a_new_run_restarts
	p := newStreamProtocol()
	events := decodeLines(t, p,
		streamDelta(map[string]any{"type": "text_delta", "text": "a"}),
		resultLine("success", false),
		streamDelta(map[string]any{"type": "text_delta", "text": "b"}),
	)
	require.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.TextDelta{Delta: "a"},
		models.RunSettled{},
		models.RunStarted{},
		models.TextDelta{Delta: "b"},
	}, events)
}

func TestStreamProtocolRejectedRateLimitFollowedByErrorResultReportsTheUsageLimit(t *testing.T) {
	// Rust: stream_protocol::rejected_rate_limit_followed_by_error_result_reports_the_usage_limit
	p := newStreamProtocol()
	events := decodeLines(t, p,
		map[string]any{"type": "rate_limit_event", "rate_limit_info": map[string]any{
			"status": "rejected", "resetsAt": 1784283600, "rateLimitType": "five_hour"}},
		map[string]any{"type": "result", "subtype": "success", "is_error": true,
			"result": "You've hit your session limit · resets 3:45pm"},
	)
	require.Equal(t, []models.AgentEvent{
		models.UsageLimitReached{ResetsAt: ptrTo(int64(1784283600)), Message: "You've hit your session limit · resets 3:45pm"},
		models.RunSettled{},
	}, events)
}

func TestStreamProtocolLimitTextAloneIsDetectedWithoutAResetTime(t *testing.T) {
	// Rust: stream_protocol::limit_text_alone_is_detected_without_a_reset_time
	p := newStreamProtocol()
	events := decodeLines(t, p, map[string]any{"type": "result", "subtype": "error", "is_error": true,
		"result": "Claude AI usage limit reached"})
	require.NotEmpty(t, events)
	assert.Equal(t, models.UsageLimitReached{ResetsAt: nil, Message: "Claude AI usage limit reached"}, events[0])
}

func TestStreamProtocolAllowedRateLimitAndSuccessfulResultsDoNotReportALimit(t *testing.T) {
	// Rust: stream_protocol::allowed_rate_limit_and_successful_results_do_not_report_a_limit
	p := newStreamProtocol()
	events := decodeLines(t, p,
		map[string]any{"type": "rate_limit_event", "rate_limit_info": map[string]any{"status": "rejected", "resetsAt": 5}},
		map[string]any{"type": "rate_limit_event", "rate_limit_info": map[string]any{"status": "allowed"}},
		resultLine("error_max_turns", true),
	)
	require.Equal(t, []models.AgentEvent{
		models.Notice{Level: models.NoticeError, Message: "boom"},
		models.RunSettled{},
	}, events)
}

func TestStreamProtocolAbortedResultWarnsAndFailedResultReportsTheError(t *testing.T) {
	// Rust: stream_protocol::aborted_result_warns_and_failed_result_reports_the_error
	p := newStreamProtocol()
	aborted := decodeLines(t, p, map[string]any{"type": "result", "subtype": "error_during_execution",
		"is_error": true, "terminal_reason": "aborted_streaming"})
	require.Equal(t, []models.AgentEvent{
		models.Notice{Level: models.NoticeWarning, Message: "ejecución interrumpida"},
		models.RunSettled{},
	}, aborted)

	failed := decodeLines(t, p, resultLine("error_max_turns", true))
	require.Equal(t, []models.AgentEvent{
		models.Notice{Level: models.NoticeError, Message: "boom"},
		models.RunSettled{},
	}, failed)
}

func TestStreamProtocolLongToolOutputIsTruncated(t *testing.T) {
	// Rust: stream_protocol::long_tool_output_is_truncated
	p := newStreamProtocol()
	big := strings.Repeat("x", maxToolTextChars+50)
	events := decodeLines(t, p, map[string]any{"type": "user", "message": map[string]any{"content": []any{
		map[string]any{"type": "tool_result", "tool_use_id": "t", "content": big},
	}}})
	require.Len(t, events, 1)
	output := events[0].(models.ToolFinished).Output
	assert.Equal(t, strings.Repeat("x", maxToolTextChars), output)
}

func TestStreamProtocolPromptIsAUserMessageWithPlainText(t *testing.T) {
	// Rust: stream_protocol::prompt_is_a_user_message_with_plain_text
	p := newStreamProtocol()
	line := mustEncode(t, p, models.PromptCommand{Message: "hola"}, "id")
	assert.Equal(t, map[string]any{"type": "user", "message": map[string]any{"role": "user", "content": "hola"}},
		parseLine(t, line))
	assert.NotContains(t, line, "\n")
}

func TestStreamProtocolPromptWithImagesUsesContentBlocks(t *testing.T) {
	// Rust: stream_protocol::prompt_with_images_uses_content_blocks
	p := newStreamProtocol()
	line := mustEncode(t, p, models.PromptCommand{
		Message: "mira",
		Images:  []models.ImageAttachment{{Name: "a.png", MimeType: "image/png", Data: "QUJD"}},
	}, "id")
	assert.Equal(t, []any{
		map[string]any{"type": "text", "text": "mira"},
		map[string]any{"type": "image", "source": map[string]any{"type": "base64", "media_type": "image/png", "data": "QUJD"}},
	}, parseLine(t, line)["message"].(map[string]any)["content"])
}

func TestStreamProtocolSetModeIsAPermissionModeControlRequest(t *testing.T) {
	// Rust: stream_protocol::set_mode_is_a_permission_mode_control_request
	p := newStreamProtocol()
	planLine := mustEncode(t, p, models.SetModeCommand{Mode: models.AgentModePlan}, "req-3")
	assert.Equal(t, map[string]any{"type": "control_request", "request_id": "req-3",
		"request": map[string]any{"subtype": "set_permission_mode", "mode": "plan"}}, parseLine(t, planLine))

	agentLine := mustEncode(t, p, models.SetModeCommand{Mode: models.AgentModeAgent}, "req-3")
	assert.Equal(t, "acceptEdits", parseLine(t, agentLine)["request"].(map[string]any)["mode"])
}

func TestStreamProtocolInitAndStatusReportTheCurrentMode(t *testing.T) {
	// Rust: stream_protocol::init_and_status_report_the_current_mode
	p := newStreamProtocol()
	events := decodeLines(t,
		p,
		map[string]any{"type": "system", "subtype": "init", "permissionMode": "acceptEdits"},
		map[string]any{"type": "system", "subtype": "status", "status": nil, "permissionMode": "plan"},
		map[string]any{"type": "system", "subtype": "status", "permissionMode": "bypassPermissions"},
		map[string]any{"type": "system", "subtype": "hook_started"},
	)
	require.Equal(t, []models.AgentEvent{
		models.StatusChanged{Key: "mode", Text: ptrTo("agent")},
		models.StatusChanged{Key: "mode", Text: ptrTo("plan")},
		models.StatusChanged{Key: "mode", Text: ptrTo("bypassPermissions")},
	}, events)
}

func TestStreamProtocolAbortIsAnInterruptControlRequest(t *testing.T) {
	// Rust: stream_protocol::abort_is_an_interrupt_control_request
	p := newStreamProtocol()
	line := mustEncode(t, p, models.AbortCommand{}, "req-9")
	assert.Equal(t, map[string]any{"type": "control_request", "request_id": "req-9",
		"request": map[string]any{"subtype": "interrupt"}}, parseLine(t, line))
}

func TestStreamProtocolConfirmedDialogAllowsTheToolWithItsOriginalInput(t *testing.T) {
	// Rust: stream_protocol::confirmed_dialog_allows_the_tool_with_its_original_input
	p := newStreamProtocol()
	requestPermission(t, p)
	line := mustEncode(t, p, answerCommand("r1", confirmedAnswer(true)), "x")
	assert.Equal(t, map[string]any{"type": "control_response", "response": map[string]any{
		"subtype": "success", "request_id": "r1",
		"response": map[string]any{"behavior": "allow", "updatedInput": map[string]any{"command": "ls"}}},
	}, parseLine(t, line))
}

func TestStreamProtocolDeclinedOrCancelledDialogDeniesTheTool(t *testing.T) {
	// Rust: stream_protocol::declined_or_cancelled_dialog_denies_the_tool
	for _, answer := range []models.DialogAnswer{confirmedAnswer(false), models.CancelledAnswer{}} {
		p := newStreamProtocol()
		requestPermission(t, p)
		line := mustEncode(t, p, answerCommand("r1", answer), "x")
		assert.Equal(t, map[string]any{"behavior": "deny", "message": denyMessage},
			parseLine(t, line)["response"].(map[string]any)["response"], "%T", answer)
	}
}

func TestStreamProtocolAnsweringAnUnknownOrAlreadyAnsweredDialogFails(t *testing.T) {
	// Rust: stream_protocol::answering_an_unknown_or_already_answered_dialog_fails
	p := newStreamProtocol()
	requestPermission(t, p)
	command := answerCommand("r1", confirmedAnswer(true))
	_ = mustEncode(t, p, command, "x")
	_, _, err := p.Encode(command, "x")
	require.Error(t, err)
}

// Rule from source: stream_protocol.rs::permission_message falls back to the whole input, as compact
// JSON, when the tool has no known summary key.
func TestStreamProtocolPermissionFallsBackToTheWholeInputForUnknownTools(t *testing.T) {
	p := newStreamProtocol()
	events := decodeLines(t, p, controlRequest("r1", "Custom", map[string]any{"a": "1", "b": map[string]any{}}))
	require.Equal(t, []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    "r1",
		Title: "Permitir Custom",
		Kind:  models.ConfirmDialog{Message: ptrTo(`{"a":"1","b":{}}`)},
	}}}, events)
}

// Rule from source: stream_protocol.rs::parse_questions rejects the whole request when any option lacks a
// label, so the request falls back to a plain confirmation.
func TestStreamProtocolQuestionWithAnUnlabelledOptionFallsBackToAConfirmation(t *testing.T) {
	p := newStreamProtocol()
	events := decodeLines(t, p, controlRequest("r1", "AskUserQuestion", map[string]any{"questions": []any{
		map[string]any{"question": "¿Qué?", "options": []any{
			map[string]any{"label": "A"}, map[string]any{"label": "", "description": "vacío"}}},
	}}))
	require.Len(t, events, 1)
	_, isConfirm := events[0].(models.DialogRequested).Request.Kind.(models.ConfirmDialog)
	assert.True(t, isConfirm)
}

// Rule from source: stream_protocol.rs::answer_dialog keeps the question pending when the index is out of
// range, so the valid answers that follow still complete the request.
func TestStreamProtocolOutOfRangeQuestionAnswerKeepsThePendingRequest(t *testing.T) {
	p := newStreamProtocol()
	decodeLines(t, p, controlRequest("r1", "AskUserQuestion", twoQuestions()))

	_, ok, err := p.Encode(answerCommand("r1#7", valueAnswer("Uno")), "x")
	require.ErrorIs(t, err, werrors.Runtime("no hay un permiso pendiente con id r1#7"))
	assert.False(t, ok)

	_, partialOK, partialErr := p.Encode(answerCommand("r1#0", valueAnswer("Rojo")), "x")
	require.NoError(t, partialErr)
	assert.False(t, partialOK, "a partial answer produces no line")
	line := mustEncode(t, p, answerCommand("r1#1", valueAnswer("Uno")), "x")
	assert.Equal(t, "allow", parseLine(t, line)["response"].(map[string]any)["response"].(map[string]any)["behavior"])
}

// Rule from source: stream_protocol.rs::decode_result resolves the pending dialogs in a stable order. The
// Rust HashMap drain is unordered; the Go port sorts by request id so the event log is deterministic.
func TestStreamProtocolFinishedRunResolvesPendingDialogsInRequestIDOrder(t *testing.T) {
	p := newStreamProtocol()
	decodeLines(t, p,
		controlRequest("r2", "Write", map[string]any{"file_path": "/w/b.txt"}),
		controlRequest("r1", "Write", map[string]any{"file_path": "/w/a.txt"}),
	)
	events := decodeLines(t, p, resultLine("success", false))
	require.Equal(t, []models.AgentEvent{
		models.RunSettled{},
		models.DialogResolved{DialogID: "r1"},
		models.DialogResolved{DialogID: "r2"},
	}, events)
}

// Rule from source: stream_protocol.rs::decode keeps the JSON numbers of tool inputs as written and does not
// HTML-escape them, so the tool summary shows what the agent sent.
func TestStreamProtocolToolArgsKeepNumbersAndHTMLCharactersAsWritten(t *testing.T) {
	p := newStreamProtocol()
	raw := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"t1","name":"Bash",` +
		`"input":{"command":"a < b && c","n":1.0,"big":12345678901234567890}}]}}`
	events := p.Decode(testTaskID(t), raw)
	require.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.ToolStarted{CallID: "t1", ToolName: "Bash",
			Args: `{"big":12345678901234567890,"command":"a < b && c","n":1.0}`},
	}, events)
}

// Rule from source: stream_protocol.rs::note_rate_limit keeps resetsAt only when it is an integer.
func TestStreamProtocolRateLimitResetsAtMustBeAnInteger(t *testing.T) {
	p := newStreamProtocol()
	decodeLines(t, p, map[string]any{"type": "rate_limit_event", "rate_limit_info": map[string]any{
		"status": "rejected", "resetsAt": 1.5}})
	events := decodeLines(t, p, map[string]any{"type": "result", "subtype": "success", "is_error": true,
		"result": "boom"})
	require.NotEmpty(t, events)
	assert.Equal(t, models.UsageLimitReached{ResetsAt: nil, Message: "boom"}, events[0])
}

// Rule from source: stream_protocol.rs::encode rejects an unknown command kind with Unsupported.
func TestStreamProtocolEncodeRejectsAnUnknownCommand(t *testing.T) {
	p := newStreamProtocol()
	_, ok, err := p.Encode(nil, "x")
	require.ErrorIs(t, err, werrors.Unsupported("comando de agente no soportado: <nil>"))
	assert.False(t, ok)
}
