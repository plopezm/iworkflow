package cursor

import (
	"encoding/json"
	"os"
	"sort"
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

// ptrTo returns a pointer to a copy of value, for the optional fields of the session config.
func ptrTo[T any](value T) *T {
	return &value
}

// newTestProtocol builds a protocol that works in /work, the folder every test uses.
func newTestProtocol(config acpSessionConfig) *acpProtocol {
	config.cwd = "/work"
	return newACPProtocol(config)
}

// feed decodes one agent record and returns the events it produced and the lines written back.
func feed(t *testing.T, p *acpProtocol, record map[string]any) ([]models.AgentEvent, []map[string]any) {
	t.Helper()
	raw, err := json.Marshal(record)
	require.NoError(t, err)
	events := p.Decode(testTaskID(t), string(raw))
	return events, sent(t, p)
}

// sent takes the lines written to stdin so far and parses them back, the way the agent reads them.
func sent(t *testing.T, p *acpProtocol) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range p.TakeOutgoing() {
		records = append(records, parseLine(t, line))
	}
	return records
}

// parseLine decodes one line written to stdin.
func parseLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &record), line)
	return record
}

// respond builds a successful reply of the agent to a request of the protocol.
func respond(id uint64, result map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

// respondError builds an error reply of the agent to a request of the protocol.
func respondError(id uint64, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

// prompt builds a prompt command without images.
func prompt(message string) models.AgentCommand {
	return models.PromptCommand{Message: message}
}

// command encodes a command that writes nothing itself and returns the lines it queued on stdin.
func command(t *testing.T, p *acpProtocol, cmd models.AgentCommand) []map[string]any {
	t.Helper()
	line, ok, err := p.Encode(cmd, "r")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, line)
	return sent(t, p)
}

// newReadyProtocol returns a protocol whose session s1 is open; ids 1 to 3 are consumed.
func newReadyProtocol(t *testing.T, config acpSessionConfig) *acpProtocol {
	t.Helper()
	p := newTestProtocol(config)
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	feed(t, p, respond(2, map[string]any{}))
	feed(t, p, respond(3, map[string]any{"sessionId": "s1"}))
	sent(t, p)
	return p
}

// sessionUpdate wraps an update of the session s1 in the notification the agent sends.
func sessionUpdate(update map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "method": "session/update",
		"params": map[string]any{"sessionId": "s1", "update": update}}
}

// permission builds a request_permission of the agent for a tool of the given kind.
func permission(id uint64, kind string, input map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": "session/request_permission",
		"params": map[string]any{"sessionId": "s1",
			"toolCall": map[string]any{"toolCallId": "c", "title": "Run it", "kind": kind, "rawInput": input},
			"options": []any{
				map[string]any{"optionId": "allow-once", "name": "Allow", "kind": "allow_once"},
				map[string]any{"optionId": "reject-once", "name": "Reject", "kind": "reject_once"},
			}}}
}

// requestedDialogIDs lists the ids of the dialogs opened in events, in order.
func requestedDialogIDs(events []models.AgentEvent) []string {
	var ids []string
	for _, event := range events {
		if requested, ok := event.(models.DialogRequested); ok {
			ids = append(ids, requested.Request.ID)
		}
	}
	return ids
}

// modeStatusEvent is the status line that reports the working mode.
func modeStatusEvent(mode string) models.StatusChanged {
	return models.StatusChanged{Key: "mode", Text: &mode}
}

// idOf returns the id of a record the protocol wrote, as the number it was sent with.
func idOf(t *testing.T, record map[string]any) uint64 {
	t.Helper()
	id, ok := record["id"].(float64)
	require.True(t, ok, "the record has no numeric id: %v", record)
	return uint64(id)
}

// promptBlocks returns the content blocks of a session/prompt request.
func promptBlocks(t *testing.T, record map[string]any) []any {
	t.Helper()
	params, ok := record["params"].(map[string]any)
	require.True(t, ok, "the request has no params: %v", record)
	blocks, ok := params["prompt"].([]any)
	require.True(t, ok, "the request has no prompt blocks: %v", record)
	return blocks
}

// requireJSONEqual compares actual, once marshalled, with the JSON text expected.
func requireJSONEqual(t *testing.T, expected string, actual any) {
	t.Helper()
	raw, err := json.Marshal(actual)
	require.NoError(t, err)
	assert.JSONEq(t, expected, string(raw))
}

// eventKind names the kind of an event, to compare the sequence of a whole session.
func eventKind(event models.AgentEvent) string {
	switch event.(type) {
	case models.RunStarted:
		return "run_started"
	case models.ThinkingDelta:
		return "thinking"
	case models.TextDelta:
		return "text"
	case models.AssistantMessage:
		return "message"
	case models.ToolStarted:
		return "tool_started"
	case models.ToolFinished:
		return "tool_finished"
	case models.RunSettled:
		return "settled"
	default:
		return "other"
	}
}

// anyEvent reports whether one of events satisfies match.
func anyEvent(events []models.AgentEvent, match func(models.AgentEvent) bool) bool {
	for _, event := range events {
		if match(event) {
			return true
		}
	}
	return false
}

// Rust: acp_protocol::tests::opening_a_session_reports_the_current_mode
func TestAcpProtocolOpeningASessionReportsTheCurrentMode(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	feed(t, p, respond(2, map[string]any{}))
	events, _ := feed(t, p, respond(3, map[string]any{
		"sessionId": "s1",
		"modes": map[string]any{"currentModeId": "agent",
			"availableModes": []any{map[string]any{"id": "agent"}, map[string]any{"id": "plan"}}},
	}))
	assert.Equal(t, []models.AgentEvent{modeStatusEvent("agent")}, events)
}

// Rust: acp_protocol::tests::set_mode_is_confirmed_before_it_is_reported
func TestAcpProtocolSetModeIsConfirmedBeforeItIsReported(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	out := command(t, p, models.SetModeCommand{Mode: models.AgentModePlan})
	require.Len(t, out, 1)
	assert.Equal(t, "session/set_mode", out[0]["method"])
	requireJSONEqual(t, `{"sessionId":"s1","modeId":"plan"}`, out[0]["params"])

	events, _ := feed(t, p, respond(idOf(t, out[0]), map[string]any{}))
	assert.Equal(t, []models.AgentEvent{modeStatusEvent("plan")}, events)
}

// Rust: acp_protocol::tests::rejected_set_mode_warns_and_keeps_the_mode
func TestAcpProtocolRejectedSetModeWarnsAndKeepsTheMode(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	out := command(t, p, models.SetModeCommand{Mode: models.AgentModePlan})
	require.Len(t, out, 1)

	events, _ := feed(t, p, respondError(idOf(t, out[0]), -1, "no"))

	assert.Equal(t, []models.AgentEvent{
		models.Notice{Level: models.NoticeWarning, Message: "no se pudo cambiar el modo: no"},
	}, events)
}

// Rust: acp_protocol::tests::overlapping_mode_changes_are_confirmed_each_with_its_own_mode
func TestAcpProtocolOverlappingModeChangesAreConfirmedEachWithItsOwnMode(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	plan := command(t, p, models.SetModeCommand{Mode: models.AgentModePlan})
	agent := command(t, p, models.SetModeCommand{Mode: models.AgentModeAgent})
	require.Len(t, plan, 1)
	require.Len(t, agent, 1)

	first, _ := feed(t, p, respond(idOf(t, plan[0]), map[string]any{}))
	second, _ := feed(t, p, respond(idOf(t, agent[0]), map[string]any{}))

	assert.Equal(t, []models.AgentEvent{modeStatusEvent("plan")}, first)
	assert.Equal(t, []models.AgentEvent{modeStatusEvent("agent")}, second)
}

// Rust: acp_protocol::tests::mode_updates_from_the_agent_are_reported
func TestAcpProtocolModeUpdatesFromTheAgentAreReported(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "current_mode_update", "currentModeId": "ask"}))

	assert.Equal(t, []models.AgentEvent{modeStatusEvent("ask")}, events)
}

// Rust: acp_protocol::tests::set_mode_before_the_session_is_open_is_an_error
func TestAcpProtocolSetModeBeforeTheSessionIsOpenIsAnError(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})

	_, _, err := p.Encode(models.SetModeCommand{Mode: models.AgentModePlan}, "r")

	assert.ErrorIs(t, err, werrors.Runtime(""))
}

// Rust: acp_protocol::tests::start_sends_only_initialize
func TestAcpProtocolStartSendsOnlyInitialize(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})

	lines := p.Start()

	require.Len(t, lines, 1)
	first := parseLine(t, lines[0])
	assert.Equal(t, "initialize", first["method"])
	assert.Equal(t, float64(1), first["id"])
	params, ok := first["params"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(1), params["protocolVersion"])
}

// Rust: acp_protocol::tests::handshake_walks_initialize_authenticate_and_session_new
func TestAcpProtocolHandshakeWalksInitializeAuthenticateAndSessionNew(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})
	p.Start()

	_, afterInit := feed(t, p, respond(1, map[string]any{"authMethods": []any{}}))
	_, afterAuth := feed(t, p, respond(2, map[string]any{}))

	require.Len(t, afterInit, 1)
	assert.Equal(t, "authenticate", afterInit[0]["method"])
	requireJSONEqual(t, `{"methodId":"cursor_login"}`, afterInit[0]["params"])
	require.Len(t, afterAuth, 1)
	assert.Equal(t, "session/new", afterAuth[0]["method"])
	requireJSONEqual(t, `{"cwd":"/work","mcpServers":[]}`, afterAuth[0]["params"])
}

// Rust: acp_protocol::tests::a_prompt_is_held_until_session_new_answers
func TestAcpProtocolAPromptIsHeldUntilSessionNewAnswers(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	feed(t, p, respond(2, map[string]any{}))

	early := command(t, p, prompt("hola"))
	_, released := feed(t, p, respond(3, map[string]any{"sessionId": "s1"}))

	assert.Empty(t, early)
	require.Len(t, released, 1)
	assert.Equal(t, "session/prompt", released[0]["method"])
	requireJSONEqual(t, `{"sessionId":"s1","prompt":[{"type":"text","text":"hola"}]}`, released[0]["params"])
}

// Rust: acp_protocol::tests::the_model_is_applied_before_the_held_prompt_is_released
func TestAcpProtocolTheModelIsAppliedBeforeTheHeldPromptIsReleased(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{model: ptrTo("gpt-5.5[fast=false]")})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	feed(t, p, respond(2, map[string]any{}))
	command(t, p, prompt("hola"))

	_, afterSession := feed(t, p, respond(3, map[string]any{"sessionId": "s1"}))
	_, afterModel := feed(t, p, respond(4, map[string]any{}))

	require.Len(t, afterSession, 1)
	assert.Equal(t, "session/set_model", afterSession[0]["method"])
	requireJSONEqual(t, `{"sessionId":"s1","modelId":"gpt-5.5[fast=false]"}`, afterSession[0]["params"])
	require.Len(t, afterModel, 1)
	assert.Equal(t, "session/prompt", afterModel[0]["method"])
}

// Rust: acp_protocol::tests::a_failed_model_switch_warns_and_still_releases_the_prompt
func TestAcpProtocolAFailedModelSwitchWarnsAndStillReleasesThePrompt(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{model: ptrTo("x")})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	feed(t, p, respond(2, map[string]any{}))
	feed(t, p, respond(3, map[string]any{"sessionId": "s1"}))
	command(t, p, prompt("hola"))

	events, out := feed(t, p, respondError(4, -1, "no model"))

	require.NotEmpty(t, events)
	notice, ok := events[0].(models.Notice)
	require.True(t, ok)
	assert.Equal(t, models.NoticeWarning, notice.Level)
	assert.Contains(t, notice.Message, "no model")
	require.Len(t, out, 1)
	assert.Equal(t, "session/prompt", out[0]["method"])
}

// Rust: acp_protocol::tests::resume_loads_the_saved_session_and_drops_its_replayed_history
func TestAcpProtocolResumeLoadsTheSavedSessionAndDropsItsReplayedHistory(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{resumeSession: ptrTo("old")})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	_, load := feed(t, p, respond(2, map[string]any{}))
	replayed, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "agent_message_chunk",
		"content": map[string]any{"type": "text", "text": "historia"}}))
	_, afterLoad := feed(t, p, respond(3, map[string]any{}))
	command(t, p, prompt("sigue"))

	require.Len(t, load, 1)
	assert.Equal(t, "session/load", load[0]["method"])
	requireJSONEqual(t, `{"sessionId":"old","cwd":"/work","mcpServers":[]}`, load[0]["params"])
	assert.Empty(t, replayed)
	assert.Empty(t, afterLoad)
	require.NotNil(t, p.sessionID)
	assert.Equal(t, "old", *p.sessionID)
}

// Rust: acp_protocol::tests::a_failed_load_falls_back_to_a_new_session_with_a_warning
func TestAcpProtocolAFailedLoadFallsBackToANewSessionWithAWarning(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{resumeSession: ptrTo("old")})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	feed(t, p, respond(2, map[string]any{}))

	events, out := feed(t, p, respondError(3, -1, "gone"))

	require.NotEmpty(t, events)
	notice, ok := events[0].(models.Notice)
	require.True(t, ok)
	assert.Equal(t, models.NoticeWarning, notice.Level)
	assert.Contains(t, notice.Message, "gone")
	require.Len(t, out, 1)
	assert.Equal(t, "session/new", out[0]["method"])
}

// Rust: acp_protocol::tests::the_session_listener_receives_the_new_session_id
func TestAcpProtocolTheSessionListenerReceivesTheNewSessionID(t *testing.T) {
	var seen []string
	p := newACPProtocol(acpSessionConfig{}).withSessionListener(func(id string) {
		seen = append(seen, id)
	})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	feed(t, p, respond(2, map[string]any{}))
	feed(t, p, respond(3, map[string]any{"sessionId": "s9"}))

	assert.Equal(t, []string{"s9"}, seen)
}

// Rust: acp_protocol::tests::rejected_authentication_reports_an_error_and_drops_the_prompt
func TestAcpProtocolRejectedAuthenticationReportsAnErrorAndDropsThePrompt(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})
	p.Start()
	feed(t, p, respond(1, map[string]any{}))
	command(t, p, prompt("hola"))

	events, out := feed(t, p, respondError(2, -32000, "login"))

	require.Len(t, events, 2)
	notice, ok := events[0].(models.Notice)
	require.True(t, ok)
	assert.Equal(t, models.NoticeError, notice.Level)
	assert.Contains(t, notice.Message, "agent login")
	assert.Equal(t, models.RunSettled{}, events[1])
	assert.Empty(t, out)
	_, _, err := p.Encode(prompt("otra"), "r")
	assert.Error(t, err)
}

// Rust: acp_protocol::tests::streamed_text_and_thinking_become_one_message_when_the_turn_settles
func TestAcpProtocolStreamedTextAndThinkingBecomeOneMessageWhenTheTurnSettles(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))

	thought, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "agent_thought_chunk",
		"content": map[string]any{"type": "text", "text": "pienso"}}))
	chunk, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "agent_message_chunk",
		"content": map[string]any{"type": "text", "text": "hola"}}))
	settled, out := feed(t, p, respond(4, map[string]any{"stopReason": "end_turn"}))

	thinking := "pienso"
	assert.Equal(t, []models.AgentEvent{models.RunStarted{}, models.ThinkingDelta{Delta: "pienso"}}, thought)
	assert.Equal(t, []models.AgentEvent{models.TextDelta{Delta: "hola"}}, chunk)
	assert.Equal(t, []models.AgentEvent{
		models.AssistantMessage{Text: "hola", Thinking: &thinking},
		models.RunSettled{},
	}, settled)
	assert.Empty(t, out)
}

// Rust: acp_protocol::tests::a_cancelled_turn_warns_and_a_queued_prompt_is_sent_after_the_current_one
func TestAcpProtocolACancelledTurnWarnsAndAQueuedPromptIsSentAfterTheCurrentOne(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("uno"))
	second := command(t, p, prompt("dos"))

	events, out := feed(t, p, respond(4, map[string]any{"stopReason": "cancelled"}))

	assert.Empty(t, second)
	assert.Contains(t, events, models.Notice{Level: models.NoticeWarning, Message: interruptedMessage})
	require.NotEmpty(t, events)
	assert.Equal(t, models.RunSettled{}, events[len(events)-1])
	require.Len(t, out, 1)
	blocks := promptBlocks(t, out[0])
	require.NotEmpty(t, blocks)
	assert.Equal(t, "dos", blocks[0].(map[string]any)["text"])
}

// Rust: acp_protocol::tests::a_prompt_rejected_for_usage_limit_reports_the_limit_and_settles
func TestAcpProtocolAPromptRejectedForUsageLimitReportsTheLimitAndSettles(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))
	const message = "You've hit your usage limit. Upgrade your plan"

	events, _ := feed(t, p, respondError(4, -1, message))

	assert.Equal(t, []models.AgentEvent{
		models.RunStarted{},
		models.UsageLimitReached{Message: message},
		models.RunSettled{},
	}, events)
}

// Rust: acp_protocol::tests::a_failed_prompt_reports_the_error_and_settles
func TestAcpProtocolAFailedPromptReportsTheErrorAndSettles(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))

	events, _ := feed(t, p, respondError(4, -1, "boom"))

	assert.Contains(t, events, models.Notice{Level: models.NoticeError, Message: "boom"})
	require.NotEmpty(t, events)
	assert.Equal(t, models.RunSettled{}, events[len(events)-1])
}

// Rust: acp_protocol::tests::images_are_sent_as_acp_image_blocks
func TestAcpProtocolImagesAreSentAsAcpImageBlocks(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	out := command(t, p, models.PromptCommand{
		Message: "mira",
		Images:  []models.ImageAttachment{{Name: "a.png", MimeType: "image/png", Data: "QUJD"}},
	})

	require.Len(t, out, 1)
	blocks := promptBlocks(t, out[0])
	require.Len(t, blocks, 2)
	requireJSONEqual(t, `{"type":"image","mimeType":"image/png","data":"QUJD"}`, blocks[1])
}

// Rust: acp_protocol::tests::steer_and_follow_up_are_unsupported
func TestAcpProtocolSteerAndFollowUpAreUnsupported(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	for _, cmd := range []models.AgentCommand{
		models.SteerCommand{Message: "x"},
		models.FollowUpCommand{Message: "x"},
	} {
		_, _, err := p.Encode(cmd, "r")
		assert.ErrorIs(t, err, werrors.Unsupported(""))
	}
}

// Rust: acp_protocol::tests::a_tool_is_started_once_its_arguments_are_known_and_finished_with_its_output
func TestAcpProtocolAToolIsStartedOnceItsArgumentsAreKnownAndFinishedWithItsOutput(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))
	feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "agent_message_chunk",
		"content": map[string]any{"type": "text", "text": "voy"}}))

	created, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "c1",
		"title": "Edit File", "kind": "edit", "status": "pending", "rawInput": map[string]any{}}))
	known, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "c1",
		"title": "Edit `a.txt`", "rawInput": map[string]any{"path": "a.txt"}}))
	running, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "c1",
		"status": "in_progress"}))
	done, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "c1",
		"status":  "completed",
		"content": []any{map[string]any{"type": "diff", "path": "a.txt", "oldText": "", "newText": "hi"}}}))

	assert.Empty(t, created)
	assert.Equal(t, []models.AgentEvent{
		models.AssistantMessage{Text: "voy"},
		models.ToolStarted{CallID: "c1", ToolName: "edit", Args: `{"path":"a.txt"}`},
	}, known)
	assert.Empty(t, running)
	assert.Equal(t, []models.AgentEvent{
		models.ToolFinished{CallID: "c1", ToolName: "edit", Output: "a.txt\nhi", IsError: false},
	}, done)
}

// Rust: acp_protocol::tests::a_tool_created_with_arguments_starts_immediately_and_shell_output_is_joined
func TestAcpProtocolAToolCreatedWithArgumentsStartsImmediatelyAndShellOutputIsJoined(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))

	created, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "tool_call", "toolCallId": "c2",
		"title": "`ls`", "kind": "execute", "status": "pending",
		"rawInput": map[string]any{"command": "ls"}}))
	done, _ := feed(t, p, sessionUpdate(map[string]any{"sessionUpdate": "tool_call_update", "toolCallId": "c2",
		"status":    "completed",
		"rawOutput": map[string]any{"exitCode": 2, "stdout": "out", "stderr": "err"}}))

	require.Len(t, created, 2)
	assert.Equal(t, models.ToolStarted{CallID: "c2", ToolName: "execute", Args: `{"command":"ls"}`}, created[1])
	assert.Equal(t, []models.AgentEvent{
		models.ToolFinished{CallID: "c2", ToolName: "execute", Output: "out\nerr", IsError: true},
	}, done)
}

// Rust: acp_protocol::tests::edit_permissions_are_accepted_without_asking
func TestAcpProtocolEditPermissionsAreAcceptedWithoutAsking(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, out := feed(t, p, permission(7, "edit", map[string]any{"path": "a"}))

	assert.Empty(t, events)
	requireJSONEqual(t, `[{"jsonrpc":"2.0","id":7,
		"result":{"outcome":{"outcome":"selected","optionId":"allow-once"}}}]`, out)
}

// Rust: acp_protocol::tests::other_permissions_ask_with_the_real_command_and_follow_the_answer
func TestAcpProtocolOtherPermissionsAskWithTheRealCommandAndFollowTheAnswer(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, out := feed(t, p, permission(8, "execute", map[string]any{"command": "rm -rf build"}))
	allowed := command(t, p, models.AnswerDialogCommand{
		DialogID: "perm-8",
		Answer:   models.ConfirmedAnswer{Confirmed: true},
	})
	feed(t, p, permission(9, "execute", map[string]any{"command": "x"}))
	rejected := command(t, p, models.AnswerDialogCommand{
		DialogID: "perm-9",
		Answer:   models.ConfirmedAnswer{Confirmed: false},
	})

	assert.Empty(t, out)
	message := "rm -rf build"
	assert.Equal(t, []models.AgentEvent{models.DialogRequested{Request: models.DialogRequest{
		ID:    "perm-8",
		Title: "Run it",
		Kind:  models.ConfirmDialog{Message: &message},
	}}}, events)
	require.Len(t, allowed, 1)
	require.Len(t, rejected, 1)
	requireJSONEqual(t, `{"outcome":{"outcome":"selected","optionId":"allow-once"}}`, allowed[0]["result"])
	requireJSONEqual(t, `{"outcome":{"outcome":"selected","optionId":"reject-once"}}`, rejected[0]["result"])
}

// Rust: acp_protocol::tests::answering_an_unknown_dialog_is_an_error
func TestAcpProtocolAnsweringAnUnknownDialogIsAnError(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	_, _, err := p.Encode(models.AnswerDialogCommand{DialogID: "perm-1", Answer: models.CancelledAnswer{}}, "r")

	assert.ErrorIs(t, err, werrors.Runtime(""))
}

// Rust: acp_protocol::tests::settling_the_turn_resolves_dialogs_the_agent_no_longer_awaits
func TestAcpProtocolSettlingTheTurnResolvesDialogsTheAgentNoLongerAwaits(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))
	feed(t, p, permission(8, "execute", map[string]any{"command": "ls"}))

	events, _ := feed(t, p, respond(4, map[string]any{"stopReason": "end_turn"}))

	assert.Contains(t, events, models.DialogResolved{DialogID: "perm-8"})
}

// Rust: acp_protocol::tests::settling_the_turn_resolves_every_question_of_a_pending_request
func TestAcpProtocolSettlingTheTurnResolvesEveryQuestionOfAPendingRequest(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))
	feed(t, p, map[string]any{"jsonrpc": "2.0", "id": 5, "method": "cursor/ask_question", "params": map[string]any{
		"questions": []any{
			map[string]any{"id": "q1", "prompt": "?", "options": []any{map[string]any{"id": "a", "label": "A"}}},
			map[string]any{"id": "q2", "prompt": "??", "options": []any{map[string]any{"id": "b", "label": "B"}}},
		}}})

	events, _ := feed(t, p, respond(4, map[string]any{"stopReason": "end_turn"}))

	var resolved []string
	for _, event := range events {
		if closed, ok := event.(models.DialogResolved); ok {
			resolved = append(resolved, closed.DialogID)
		}
	}
	sort.Strings(resolved)
	assert.Equal(t, []string{"ask-5#0", "ask-5#1"}, resolved)
}

// Rust: acp_protocol::tests::aborting_without_a_turn_in_flight_does_not_leak_into_the_next_turn
func TestAcpProtocolAbortingWithoutATurnInFlightDoesNotLeakIntoTheNextTurn(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	feed(t, p, map[string]any{"jsonrpc": "2.0", "id": 5, "method": "cursor/ask_question", "params": map[string]any{
		"questions": []any{map[string]any{"id": "q1", "prompt": "?", "options": []any{map[string]any{"id": "a", "label": "A"}}}}}})
	command(t, p, models.AbortCommand{})
	command(t, p, prompt("hola"))

	events, _ := feed(t, p, respond(4, map[string]any{"stopReason": "end_turn"}))

	assert.False(t, anyEvent(events, func(event models.AgentEvent) bool {
		_, resolved := event.(models.DialogResolved)
		return resolved
	}))
}

// Rust: acp_protocol::tests::abort_resolves_the_cancelled_dialogs_when_the_turn_settles
func TestAcpProtocolAbortResolvesTheCancelledDialogsWhenTheTurnSettles(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))
	feed(t, p, permission(8, "execute", map[string]any{"command": "ls"}))
	command(t, p, models.AbortCommand{})

	events, _ := feed(t, p, respond(4, map[string]any{"stopReason": "cancelled"}))

	assert.Contains(t, events, models.DialogResolved{DialogID: "perm-8"})
}

// Rust: acp_protocol::tests::abort_cancels_the_turn_and_every_pending_request
func TestAcpProtocolAbortCancelsTheTurnAndEveryPendingRequest(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	command(t, p, prompt("hola"))
	feed(t, p, permission(8, "execute", map[string]any{"command": "ls"}))

	out := command(t, p, models.AbortCommand{})

	require.Len(t, out, 2)
	requireJSONEqual(t, `{"jsonrpc":"2.0","id":8,"result":{"outcome":{"outcome":"cancelled"}}}`, out[0])
	assert.Equal(t, "session/cancel", out[1]["method"])
	_, hasID := out[1]["id"]
	assert.False(t, hasID)
	requireJSONEqual(t, `{"sessionId":"s1"}`, out[1]["params"])
}

// Rust: acp_protocol::tests::ask_question_becomes_select_dialogs_answered_together
func TestAcpProtocolAskQuestionBecomesSelectDialogsAnsweredTogether(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, out := feed(t, p, map[string]any{"jsonrpc": "2.0", "id": "q", "method": "cursor/ask_question", "params": map[string]any{
		"toolCallId": "t",
		"questions": []any{
			map[string]any{"id": "q1", "prompt": "Modo", "options": []any{
				map[string]any{"id": "a", "label": "Agent"}, map[string]any{"id": "p", "label": "Plan"}}},
			map[string]any{"id": "q2", "prompt": "Rama", "options": []any{map[string]any{"id": "m", "label": "main"}}},
		}}})
	first := command(t, p, models.AnswerDialogCommand{
		DialogID: "ask-q#1",
		Answer:   models.ValueAnswer{Value: "main"},
	})
	last := command(t, p, models.AnswerDialogCommand{
		DialogID: "ask-q#0",
		Answer:   models.ValueAnswer{Value: "Plan"},
	})

	assert.Empty(t, out)
	assert.Equal(t, []string{"ask-q#0", "ask-q#1"}, requestedDialogIDs(events))
	require.NotEmpty(t, events)
	requested, ok := events[0].(models.DialogRequested)
	require.True(t, ok)
	assert.Equal(t, "Modo", requested.Request.Title)
	assert.Equal(t, models.SelectDialog{Options: []string{"Agent", "Plan"}}, requested.Request.Kind)
	assert.Empty(t, first)
	require.Len(t, last, 1)
	requireJSONEqual(t, `{"outcome":"answered","answers":[
		{"questionId":"q1","selectedOptionIds":["p"]},
		{"questionId":"q2","selectedOptionIds":["m"]}]}`, last[0]["result"].(map[string]any)["outcome"])
}

// Rust: acp_protocol::tests::an_invalid_answer_keeps_the_question_pending_for_a_valid_one
func TestAcpProtocolAnInvalidAnswerKeepsTheQuestionPendingForAValidOne(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	feed(t, p, map[string]any{"jsonrpc": "2.0", "id": 5, "method": "cursor/ask_question", "params": map[string]any{
		"questions": []any{map[string]any{"id": "q1", "prompt": "?", "options": []any{map[string]any{"id": "a", "label": "A"}}}}}})

	_, _, err := p.Encode(models.AnswerDialogCommand{
		DialogID: "ask-5#0",
		Answer:   models.ValueAnswer{Value: "no existe"},
	}, "r")
	assert.Error(t, err)
	assert.Empty(t, sent(t, p))

	out := command(t, p, models.AnswerDialogCommand{
		DialogID: "ask-5#0",
		Answer:   models.ValueAnswer{Value: "A"},
	})
	require.Len(t, out, 1)
	requireJSONEqual(t, `{"outcome":"answered","answers":[{"questionId":"q1","selectedOptionIds":["a"]}]}`,
		out[0]["result"].(map[string]any)["outcome"])
}

// Rust: acp_protocol::tests::cancelling_a_question_cancels_the_whole_request
func TestAcpProtocolCancellingAQuestionCancelsTheWholeRequest(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	feed(t, p, map[string]any{"jsonrpc": "2.0", "id": 5, "method": "cursor/ask_question", "params": map[string]any{
		"questions": []any{map[string]any{"id": "q1", "prompt": "?", "options": []any{map[string]any{"id": "a", "label": "A"}}}}}})

	out := command(t, p, models.AnswerDialogCommand{DialogID: "ask-5#0", Answer: models.CancelledAnswer{}})

	require.Len(t, out, 1)
	requireJSONEqual(t, `{"outcome":"cancelled"}`, out[0]["result"].(map[string]any)["outcome"])
}

// Rust: acp_protocol::tests::create_plan_asks_for_approval_with_the_plan_text
func TestAcpProtocolCreatePlanAsksForApprovalWithThePlanText(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, _ := feed(t, p, map[string]any{"jsonrpc": "2.0", "id": 6, "method": "cursor/create_plan",
		"params": map[string]any{"name": "Refactor", "plan": "1. hacer", "todos": []any{}}})
	accepted := command(t, p, models.AnswerDialogCommand{
		DialogID: "plan-6",
		Answer:   models.ConfirmedAnswer{Confirmed: true},
	})

	require.Len(t, events, 1)
	requested, ok := events[0].(models.DialogRequested)
	require.True(t, ok)
	assert.Equal(t, "Refactor", requested.Request.Title)
	plan := "1. hacer"
	assert.Equal(t, models.ConfirmDialog{Message: &plan}, requested.Request.Kind)
	require.Len(t, accepted, 1)
	requireJSONEqual(t, `{"outcome":"accepted"}`, accepted[0]["result"].(map[string]any)["outcome"])
}

// Rust: acp_protocol::tests::todo_updates_replace_or_merge_into_a_widget
func TestAcpProtocolTodoUpdatesReplaceOrMergeIntoAWidget(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	todos := func(items []any, merge bool) map[string]any {
		return map[string]any{"jsonrpc": "2.0", "method": "cursor/update_todos",
			"params": map[string]any{"toolCallId": "t", "todos": items, "merge": merge}}
	}

	first, _ := feed(t, p, todos([]any{
		map[string]any{"id": "1", "content": "uno", "status": "pending"},
		map[string]any{"id": "2", "content": "dos", "status": "pending"},
	}, false))
	second, _ := feed(t, p, todos([]any{
		map[string]any{"id": "1", "content": "uno", "status": "completed"},
	}, true))

	assert.Equal(t, []models.AgentEvent{
		models.WidgetChanged{Key: "todos", Lines: &[]string{"[ ] uno", "[ ] dos"}},
	}, first)
	assert.Equal(t, []models.AgentEvent{
		models.WidgetChanged{Key: "todos", Lines: &[]string{"[x] uno", "[ ] dos"}},
	}, second)
}

// Rust: acp_protocol::tests::unknown_agent_requests_get_method_not_found_and_noise_is_ignored
func TestAcpProtocolUnknownAgentRequestsGetMethodNotFoundAndNoiseIsIgnored(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, out := feed(t, p, map[string]any{"jsonrpc": "2.0", "id": 3, "method": "fs/read_text_file", "params": map[string]any{}})
	garbage := p.Decode(testTaskID(t), "no es json")

	assert.Empty(t, events)
	require.Len(t, out, 1)
	errorObject, ok := out[0]["error"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(-32601), errorObject["code"])
	assert.Equal(t, float64(3), out[0]["id"])
	assert.Empty(t, garbage)
}

// Rust: acp_protocol::tests::a_real_captured_session_maps_to_the_expected_events
func TestAcpProtocolARealCapturedSessionMapsToTheExpectedEvents(t *testing.T) {
	recording, err := os.ReadFile("testdata/cursor_acp_session.jsonl")
	require.NoError(t, err)
	p := newTestProtocol(acpSessionConfig{})
	p.Start()
	var events []models.AgentEvent
	prompted := false
	for _, raw := range strings.Split(string(recording), "\n") {
		line, isAgentLine := strings.CutPrefix(strings.TrimSuffix(raw, "\r"), "<< ")
		if !isAgentLine {
			continue
		}
		events = append(events, p.Decode(testTaskID(t), line)...)
		if p.phase == phaseReady && !prompted {
			// First moment with the session open: the user's prompt arrives.
			prompted = true
			_, _, encodeErr := p.Encode(prompt("crea hello.txt"), "r")
			require.NoError(t, encodeErr)
		}
		p.TakeOutgoing()
	}

	// Opening the session, Cursor declares its current mode.
	require.NotEmpty(t, events)
	assert.Equal(t, modeStatusEvent("agent"), events[0])
	var kinds []string
	for _, event := range events {
		if _, isStatus := event.(models.StatusChanged); !isStatus {
			kinds = append(kinds, eventKind(event))
		}
	}
	require.NotEmpty(t, kinds)
	assert.Equal(t, "run_started", kinds[0])
	assert.Equal(t, "settled", kinds[len(kinds)-1])
	assert.Equal(t, 2, countKind(kinds, "tool_started"))
	assert.Equal(t, 2, countKind(kinds, "tool_finished"))
	assert.True(t, anyEvent(events, func(event models.AgentEvent) bool {
		finished, ok := event.(models.ToolFinished)
		return ok && finished.ToolName == "execute" && !finished.IsError && strings.Contains(finished.Output, "hello.txt")
	}))
	assert.True(t, anyEvent(events, func(event models.AgentEvent) bool {
		started, ok := event.(models.ToolStarted)
		return ok && started.ToolName == "edit" && strings.Contains(started.Args, "/tmp/acp/hello.txt")
	}))
	assert.True(t, anyEvent(events, func(event models.AgentEvent) bool {
		message, ok := event.(models.AssistantMessage)
		return ok && strings.HasPrefix(message.Text, "Cre") && message.Thinking != nil
	}))
}

// countKind counts how many of kinds equal kind.
func countKind(kinds []string, kind string) int {
	count := 0
	for _, candidate := range kinds {
		if candidate == kind {
			count++
		}
	}
	return count
}
