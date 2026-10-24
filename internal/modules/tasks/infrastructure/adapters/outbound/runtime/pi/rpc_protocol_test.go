package pi

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// ptrTo returns a pointer to a copy of value.
func ptrTo[T any](value T) *T {
	return &value
}

// mustEncode encodes a command that must produce a line.
func mustEncode(t *testing.T, command models.AgentCommand, requestID string) string {
	t.Helper()
	line, ok, err := encodeCommand(command, requestID)
	require.NoError(t, err)
	require.True(t, ok, "command should produce a line")
	return line
}

// decodeJSONObject parses a JSON object into a generic map.
func decodeJSONObject(t *testing.T, text string) map[string]any {
	t.Helper()
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &record), text)
	return record
}

// decodeOne decodes a line that must produce exactly one event.
func decodeOne(t *testing.T, line string) models.AgentEvent {
	t.Helper()
	events := decode(line)
	require.Len(t, events, 1, "expected one event for %s", line)
	return events[0]
}

// mustMarshal encodes a value as JSON, failing the test on error.
func mustMarshal(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return string(raw)
}

// gentleAgentsWidget builds the setWidget record of the gentle-agents widget with payload as its first line.
func gentleAgentsWidget(t *testing.T, payload string) string {
	t.Helper()
	return mustMarshal(t, map[string]any{
		"type":        "extension_ui_request",
		"id":          "u",
		"method":      "setWidget",
		"widgetKey":   "gentle-agents",
		"widgetLines": []string{payload},
	})
}

// statusEvent is the jobs status line of the agent.
func statusEvent(text *string) models.StatusChanged {
	return models.StatusChanged{Key: "gentle-jobs", Text: text}
}

// lastSubagentsChanged returns the last SubagentsChanged event, if any.
func lastSubagentsChanged(events []models.AgentEvent) (models.SubagentsChanged, bool) {
	for index := len(events) - 1; index >= 0; index-- {
		if changed, ok := events[index].(models.SubagentsChanged); ok {
			return changed, true
		}
	}
	return models.SubagentsChanged{}, false
}

func hasSubagentsChanged(events []models.AgentEvent) bool {
	_, found := lastSubagentsChanged(events)
	return found
}

func TestPiProtocolHasNothingToSendForAModeChange(t *testing.T) {
	// Rust: rpc_protocol::tests::pi_has_nothing_to_send_for_a_mode_change
	_, ok, err := encodeCommand(models.SetModeCommand{Mode: models.AgentModePlan}, "r")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestPiProtocolEncodesCommands(t *testing.T) {
	// Rust: rpc_protocol::tests::encodes_commands
	assert.Equal(t, `{"id":"r1","message":"para","type":"steer"}`,
		mustEncode(t, models.SteerCommand{Message: "para"}, "r1"))
	assert.Equal(t, `{"id":"r2","type":"abort"}`, mustEncode(t, models.AbortCommand{}, "r2"))

	prompt := decodeJSONObject(t, mustEncode(t, models.PromptCommand{Message: "hola"}, "r3"))
	assert.Equal(t, "prompt", prompt["type"])
	assert.Equal(t, "followUp", prompt["streamingBehavior"])
	assert.NotContains(t, prompt, "images")
}

func TestPiProtocolEncodesImagesForPromptSteerAndFollowUp(t *testing.T) {
	// Rust: rpc_protocol::tests::encodes_images_for_prompt_steer_and_follow_up
	images := []models.ImageAttachment{{Name: "a.png", MimeType: "image/png", Data: "aGk="}}
	cases := []struct {
		name    string
		command models.AgentCommand
		kind    string
	}{
		{name: "prompt", command: models.PromptCommand{Message: "m", Images: images}, kind: "prompt"},
		{name: "steer", command: models.SteerCommand{Message: "m", Images: images}, kind: "steer"},
		{name: "follow_up", command: models.FollowUpCommand{Message: "m", Images: images}, kind: "follow_up"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			record := decodeJSONObject(t, mustEncode(t, tt.command, "r"))
			assert.Equal(t, tt.kind, record["type"])
			assert.Equal(t, []any{map[string]any{"type": "image", "data": "aGk=", "mimeType": "image/png"}}, record["images"])
		})
	}
}

func TestPiProtocolEncodesDialogAnswersWithDialogID(t *testing.T) {
	// Rust: rpc_protocol::tests::encodes_dialog_answers_with_dialog_id
	confirmed := decodeJSONObject(t, mustEncode(t, models.AnswerDialogCommand{
		DialogID: "d1",
		Answer:   models.ConfirmedAnswer{Confirmed: false},
	}, "ignored"))
	assert.Equal(t, map[string]any{"type": "extension_ui_response", "id": "d1", "confirmed": false}, confirmed)

	cancelled := decodeJSONObject(t, mustEncode(t, models.AnswerDialogCommand{
		DialogID: "d2",
		Answer:   models.CancelledAnswer{},
	}, "ignored"))
	assert.Equal(t, true, cancelled["cancelled"])
}

func TestPiProtocolEncodesValueAnswersWithValueField(t *testing.T) {
	// Go-only triangulation: the value branch of AnswerDialog has no Rust assertion.
	record := decodeJSONObject(t, mustEncode(t, models.AnswerDialogCommand{
		DialogID: "d3",
		Answer:   models.ValueAnswer{Value: "opción"},
	}, "ignored"))
	assert.Equal(t, map[string]any{"type": "extension_ui_response", "id": "d3", "value": "opción"}, record)
}

func TestPiProtocolDecodesRunLifecycle(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_run_lifecycle
	assert.Equal(t, models.RunStarted{}, decodeOne(t, `{"type":"agent_start"}`))
	assert.Equal(t, models.RunSettled{}, decodeOne(t, `{"type":"agent_settled"}`))
	assert.Empty(t, decode(`{"type":"turn_start"}`))
	assert.Empty(t, decode("no es json"))
}

func TestPiProtocolDecodesAssistantMessageBlocks(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_assistant_message_blocks
	event := decodeOne(t,
		`{"type":"message_end","message":{"role":"assistant","stopReason":"stop","content":[{"type":"thinking","thinking":"pienso"},{"type":"text","text":"Hecho"},{"type":"toolCall","id":"c1"}]}}`)
	assert.Equal(t, models.AssistantMessage{Text: "Hecho", Thinking: ptrTo("pienso"), Error: nil}, event)
}

func TestPiProtocolAssistantErrorAboutAUsageLimitAlsoReportsTheLimit(t *testing.T) {
	// Rust: rpc_protocol::tests::assistant_error_about_a_usage_limit_also_reports_the_limit
	events := decode(
		`{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"429 usage limit reached"}}`)
	require.Len(t, events, 2)
	assert.IsType(t, models.AssistantMessage{}, events[0])
	assert.Equal(t, models.UsageLimitReached{ResetsAt: nil, Message: "429 usage limit reached"}, events[1])

	unrelated := decode(
		`{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"boom"}}`)
	assert.Len(t, unrelated, 1)
}

func TestPiProtocolSkipsToolOnlyAssistantMessagesAndKeepsErrors(t *testing.T) {
	// Rust: rpc_protocol::tests::skips_tool_only_assistant_messages_and_keeps_errors
	assert.Empty(t, decode(`{"type":"message_end","message":{"role":"assistant","content":[{"type":"toolCall"}]}}`))
	aborted := decodeOne(t,
		`{"type":"message_end","message":{"role":"assistant","stopReason":"aborted","content":[]}}`)
	assert.Equal(t, models.AssistantMessage{Error: ptrTo("ejecución interrumpida")}, aborted)
}

func TestPiProtocolDecodesUserMessageWithStringContent(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_user_message_with_string_content
	assert.Equal(t, models.UserMessage{Text: "Arregla", Images: 0},
		decodeOne(t, `{"type":"message_end","message":{"role":"user","content":"Arregla"}}`))
}

func TestPiProtocolDecodesUserMessageCountingImages(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_user_message_counting_images
	assert.Equal(t, models.UserMessage{Text: "mira", Images: 2},
		decodeOne(t, `{"type":"message_end","message":{"role":"user","content":[{"type":"text","text":"mira"},{"type":"image","data":"x","mimeType":"image/png"},{"type":"image","data":"y","mimeType":"image/png"}]}}`))
}

func TestPiProtocolDecodesUserMessageWithImagesButNoText(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_user_message_with_images_but_no_text
	assert.Equal(t, models.UserMessage{Text: "", Images: 1},
		decodeOne(t, `{"type":"message_end","message":{"role":"user","content":[{"type":"image","data":"x","mimeType":"image/png"}]}}`))
}

// Rust: rpc_protocol::tests::legacy_user_message_events_deserialize_without_images is a serde test of the
// persisted AgentEvent JSON. Its Go counterpart is TestDecodeAgentEventDefaultsMissingImageCountToZero in
// domain/models/agent_event_json_test.go, so it is not repeated here.

func TestPiProtocolDecodesDeltas(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_deltas
	assert.Equal(t, models.TextDelta{Delta: "Ho"},
		decodeOne(t, `{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Ho"}}`))
	assert.Empty(t, decode(`{"type":"message_update","assistantMessageEvent":{"type":"toolcall_delta","delta":"{"}}`))
}

func TestPiProtocolDecodesToolLifecycle(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_tool_lifecycle
	assert.Equal(t, models.ToolStarted{CallID: "c1", ToolName: "bash", Args: `{"command":"ls"}`},
		decodeOne(t, `{"type":"tool_execution_start","toolCallId":"c1","toolName":"bash","args":{"command":"ls"}}`))
	assert.Equal(t, models.ToolFinished{CallID: "c1", ToolName: "bash", Output: "a\nb", IsError: true},
		decodeOne(t, `{"type":"tool_execution_end","toolCallId":"c1","toolName":"bash","result":{"content":[{"type":"text","text":"a\nb"}]},"isError":true}`))
}

func TestPiProtocolDecodesDialogsAndNotifications(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_dialogs_and_notifications
	selectEvent := decodeOne(t,
		`{"type":"extension_ui_request","id":"u1","method":"select","title":"¿Permitir?","options":["Sí","No"],"timeout":10000}`)
	assert.Equal(t, models.DialogRequested{Request: models.DialogRequest{
		ID:        "u1",
		Title:     "¿Permitir?",
		Kind:      models.SelectDialog{Options: []string{"Sí", "No"}},
		TimeoutMS: ptrTo(uint64(10000)),
	}}, selectEvent)
	assert.Equal(t, models.Notice{Level: models.NoticeWarning, Message: "ojo"},
		decodeOne(t, `{"type":"extension_ui_request","id":"u2","method":"notify","message":"ojo","notifyType":"warning"}`))
	assert.Empty(t, decode(`{"type":"extension_ui_request","id":"u3","method":"setTitle","title":"x"}`))
}

func TestPiProtocolBackgroundJobsCountAsActiveWork(t *testing.T) {
	// Rust: rpc_protocol::tests::background_jobs_count_as_active_work
	var tracker activityTracker
	events := tracker.fold([]models.AgentEvent{statusEvent(ptrTo("\u29d7 2 jobs"))})
	require.Len(t, events, 2)
	assert.Equal(t, statusEvent(ptrTo("\u29d7 2 jobs")), events[0])
	changed, found := lastSubagentsChanged(events)
	require.True(t, found)
	assert.Equal(t, models.SubagentsChanged{Active: 2, Waiting: 0}, changed)

	cleared := tracker.fold([]models.AgentEvent{statusEvent(nil)})
	changed, found = lastSubagentsChanged(cleared)
	require.True(t, found)
	assert.Equal(t, models.SubagentsChanged{Active: 0, Waiting: 0}, changed)
}

func TestPiProtocolJobsAndSubagentsAreSummedAndSurviveEachOther(t *testing.T) {
	// Rust: rpc_protocol::tests::jobs_and_subagents_are_summed_and_survive_each_other
	var tracker activityTracker
	tracker.fold([]models.AgentEvent{statusEvent(ptrTo("\u29d7 1 job"))})

	events := tracker.fold([]models.AgentEvent{models.SubagentsChanged{Active: 2, Waiting: 1}})
	changed, found := lastSubagentsChanged(events)
	require.True(t, found)
	assert.Equal(t, models.SubagentsChanged{Active: 3, Waiting: 1}, changed)

	events = tracker.fold([]models.AgentEvent{models.SubagentsChanged{Active: 0, Waiting: 0}})
	changed, found = lastSubagentsChanged(events)
	require.True(t, found)
	assert.Equal(t, models.SubagentsChanged{Active: 1, Waiting: 0}, changed)
}

func TestPiProtocolOtherEventsPassThroughUntouched(t *testing.T) {
	// Rust: rpc_protocol::tests::other_events_pass_through_untouched
	var tracker activityTracker
	other := models.StatusChanged{Key: "mcp", Text: ptrTo("ok")}
	assert.Equal(t, []models.AgentEvent{other}, tracker.fold([]models.AgentEvent{other}))
}

func TestPiProtocolDecodesStatusWithoutAnsiAndWidgetClear(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_status_without_ansi_and_widget_clear
	assert.Equal(t, models.StatusChanged{Key: "mcp", Text: ptrTo("MCP ok")},
		decodeOne(t, `{"type":"extension_ui_request","id":"u","method":"setStatus","statusKey":"mcp","statusText":"\u001b[38;2;1;2;3mMCP ok\u001b[39m"}`))
	assert.Equal(t, models.WidgetChanged{Key: "btw", Lines: nil},
		decodeOne(t, `{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"btw"}`))
}

func TestPiProtocolDecodesSubagentActivityFromGentleAgentsWidget(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_subagent_activity_from_gentle_agents_widget
	payload := `{"schema":"gentle-agents.activity/v1","summary":{"running":2,"queued":1,"waiting":1,"finished":4},"tasks":[]}`
	events := decode(gentleAgentsWidget(t, payload))
	require.Len(t, events, 2)
	assert.IsType(t, models.WidgetChanged{}, events[0])
	assert.Equal(t, models.SubagentsChanged{Active: 3, Waiting: 1}, events[1])
}

func TestPiProtocolClearingGentleAgentsWidgetMeansNoSubagents(t *testing.T) {
	// Rust: rpc_protocol::tests::clearing_gentle_agents_widget_means_no_subagents
	events := decode(`{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"gentle-agents"}`)
	require.NotEmpty(t, events)
	assert.Equal(t, models.SubagentsChanged{Active: 0, Waiting: 0}, events[len(events)-1])
}

func TestPiProtocolOtherWidgetsAndMalformedPayloadsDoNotChangeSubagents(t *testing.T) {
	// Rust: rpc_protocol::tests::other_widgets_and_malformed_payloads_do_not_change_subagents
	assert.False(t, hasSubagentsChanged(decode(
		`{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"btw","widgetLines":["x"]}`)))
	assert.False(t, hasSubagentsChanged(decode(
		`{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"gentle-agents","widgetLines":["no json"]}`)))
}

func TestPiProtocolFailedResponsesBecomeErrorNotices(t *testing.T) {
	// Rust: rpc_protocol::tests::failed_responses_become_error_notices
	assert.Equal(t, models.Notice{Level: models.NoticeError, Message: "steer rechazado: not streaming"},
		decodeOne(t, `{"id":"r","type":"response","command":"steer","success":false,"error":"not streaming"}`))
	assert.Empty(t, decode(`{"id":"r","type":"response","command":"prompt","success":true}`))
}

func TestPiProtocolTruncatesLongToolOutput(t *testing.T) {
	// Rust: rpc_protocol::tests::truncates_long_tool_output
	long := strings.Repeat("x", maxToolTextChars+10)
	assert.Equal(t, strings.Repeat("x", maxToolTextChars)+"\n… (truncado)", truncate(long))
}

func TestPiProtocolDecodesTheFullSubagentSnapshot(t *testing.T) {
	// Rust: rpc_protocol::tests::decodes_the_full_subagent_snapshot
	payload := mustMarshal(t, map[string]any{
		"schema":  "gentle-agents.activity/v1",
		"summary": map[string]any{"running": 1, "queued": 0, "waiting": 0, "finished": 1},
		"tasks": []any{
			map[string]any{
				"summary": map[string]any{
					"id": "t1", "agent": "explore", "label": "Mapa", "prompt": "Explora",
					"status": "running", "createdAt": 10, "startedAt": 11, "endedAt": nil,
					"lastStep": "leyendo", "lastActivityAt": 12, "turns": 2,
					"toolCalls": 3, "error": nil,
				},
				"thread": map[string]any{"version": 1, "dropped": 0, "items": []any{
					map[string]any{"kind": "text", "text": "hola"},
					map[string]any{"kind": "thinking", "text": "pienso"},
					map[string]any{"kind": "note", "text": "nota"},
					map[string]any{"kind": "tool", "name": "read", "args": "{}", "running": false,
						"isError": true, "output": "boom"},
				}},
			},
			map[string]any{"summary": map[string]any{"id": "t2", "agent": "plan", "status": "timed_out",
				"error": "tarde"}},
		},
	})

	tasks, ok := decodeSubagentTasks(gentleAgentsWidget(t, payload))
	require.True(t, ok)
	require.Len(t, tasks, 2)

	first := tasks[0]
	assert.Equal(t, "t1", first.ID)
	assert.Equal(t, "explore", first.Agent)
	assert.Equal(t, ptrTo("Mapa"), first.Label)
	assert.Equal(t, ptrTo("Explora"), first.Prompt)
	assert.Equal(t, models.SubagentRunning, first.Status)
	assert.Equal(t, ptrTo(int64(10)), first.CreatedAt)
	assert.Equal(t, ptrTo(int64(11)), first.StartedAt)
	assert.Nil(t, first.EndedAt)
	assert.Equal(t, ptrTo("leyendo"), first.LastStep)
	assert.Equal(t, ptrTo(int64(12)), first.LastActivityAt)
	assert.Equal(t, uint32(2), first.Turns)
	assert.Equal(t, uint32(3), first.ToolCalls)
	assert.Equal(t, []models.SubagentThreadItem{
		models.SubagentText{Text: "hola"},
		models.SubagentThinking{Text: "pienso"},
		models.SubagentNote{Text: "nota"},
		models.SubagentTool{Name: "read", Args: ptrTo("{}"), Running: false, IsError: true, Output: ptrTo("boom")},
	}, first.Thread)

	second := tasks[1]
	assert.Equal(t, models.SubagentTimedOut, second.Status)
	assert.Equal(t, ptrTo("tarde"), second.Error)
	assert.Equal(t, uint32(0), second.Turns)
	assert.Equal(t, uint32(0), second.ToolCalls)
	assert.Empty(t, second.Thread)
}

func TestPiProtocolMapsEveryDocumentedStatusAndFallsBackToUnknown(t *testing.T) {
	// Rust: rpc_protocol::tests::maps_every_documented_status_and_falls_back_to_unknown
	cases := []struct {
		raw      string
		expected models.SubagentStatus
	}{
		{"queued", models.SubagentQueued},
		{"running", models.SubagentRunning},
		{"waiting", models.SubagentWaiting},
		{"completed", models.SubagentCompleted},
		{"failed", models.SubagentFailed},
		{"cancelled", models.SubagentCancelled},
		{"timed_out", models.SubagentTimedOut},
		{"exploded", models.SubagentUnknown},
	}
	for _, tt := range cases {
		t.Run(tt.raw, func(t *testing.T) {
			payload := mustMarshal(t, map[string]any{"tasks": []any{
				map[string]any{"summary": map[string]any{"id": "a", "agent": "x", "status": tt.raw}},
			}})
			tasks, ok := decodeSubagentTasks(gentleAgentsWidget(t, payload))
			require.True(t, ok)
			require.Len(t, tasks, 1)
			assert.Equal(t, tt.expected, tasks[0].Status)
		})
	}

	missing := mustMarshal(t, map[string]any{"tasks": []any{
		map[string]any{"summary": map[string]any{"id": "a", "agent": "x"}},
	}})
	tasks, ok := decodeSubagentTasks(gentleAgentsWidget(t, missing))
	require.True(t, ok)
	require.Len(t, tasks, 1)
	assert.Equal(t, models.SubagentUnknown, tasks[0].Status)
}

func TestPiProtocolUnknownThreadItemsDegradeToNotesOrAreSkipped(t *testing.T) {
	// Rust: rpc_protocol::tests::unknown_thread_items_degrade_to_notes_or_are_skipped
	payload := mustMarshal(t, map[string]any{"tasks": []any{
		map[string]any{
			"summary": map[string]any{"id": "a", "agent": "x"},
			"thread": map[string]any{"items": []any{
				map[string]any{"kind": "hologram", "text": "raro"},
				map[string]any{"kind": "hologram"},
				map[string]any{"kind": "tool"},
				"no soy un objeto",
			}},
		},
	}})

	tasks, ok := decodeSubagentTasks(gentleAgentsWidget(t, payload))
	require.True(t, ok)
	require.Len(t, tasks, 1)
	assert.Equal(t, []models.SubagentThreadItem{
		models.SubagentNote{Text: "raro"},
		models.SubagentTool{Name: "", Args: nil, Running: false, IsError: false, Output: nil},
	}, tasks[0].Thread)
}

func TestPiProtocolTasksWithoutAnIDAreSkipped(t *testing.T) {
	// Rust: rpc_protocol::tests::tasks_without_an_id_are_skipped
	payload := mustMarshal(t, map[string]any{"tasks": []any{
		map[string]any{"summary": map[string]any{"agent": "x"}},
		map[string]any{"summary": map[string]any{"id": "ok", "agent": "x"}},
	}})

	tasks, ok := decodeSubagentTasks(gentleAgentsWidget(t, payload))
	require.True(t, ok)
	require.Len(t, tasks, 1)
	assert.Equal(t, "ok", tasks[0].ID)
}

func TestPiProtocolClearedWidgetIsAnEmptySnapshot(t *testing.T) {
	// Rust: rpc_protocol::tests::cleared_widget_is_an_empty_snapshot
	tasks, ok := decodeSubagentTasks(
		`{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"gentle-agents"}`)
	assert.True(t, ok)
	assert.Empty(t, tasks)
}

func TestPiProtocolMalformedOrForeignLinesAreNotASnapshot(t *testing.T) {
	// Rust: rpc_protocol::tests::malformed_or_foreign_lines_are_not_a_snapshot
	cases := []struct {
		name string
		line string
	}{
		{name: "payload is not json", line: gentleAgentsWidget(t, "no json")},
		{name: "payload without tasks", line: gentleAgentsWidget(t, `{"summary":{}}`)},
		{name: "tasks is not an array", line: gentleAgentsWidget(t, `{"tasks":"x"}`)},
		{name: "foreign schema", line: gentleAgentsWidget(t, `{"schema":"otra/v1","tasks":[]}`)},
		{name: "not json at all", line: "basura"},
		{name: "unrelated record", line: `{"type":"agent_start"}`},
		{name: "other widget with payload", line: `{"type":"extension_ui_request","method":"setWidget","widgetKey":"otro","widgetLines":["{}"]}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := decodeSubagentTasks(tt.line)
			assert.False(t, ok)
		})
	}
}
