package agy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

func testTaskID(t *testing.T) models.TaskID {
	t.Helper()
	taskID, err := models.NewTaskID()
	require.NoError(t, err)
	return taskID
}

func parseLine(t *testing.T, line string) map[string]any {
	t.Helper()
	var record map[string]any
	require.NoError(t, json.Unmarshal([]byte(line), &record), line)
	return record
}

func sent(t *testing.T, p *acpProtocol) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range p.TakeOutgoing() {
		records = append(records, parseLine(t, line))
	}
	return records
}

func feed(t *testing.T, p *acpProtocol, record map[string]any) ([]models.AgentEvent, []map[string]any) {
	t.Helper()
	raw, err := json.Marshal(record)
	require.NoError(t, err)
	events := p.Decode(testTaskID(t), string(raw))
	return events, sent(t, p)
}

func respond(id uint64, result map[string]any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
}

func respondError(id uint64, code int, message string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": code, "message": message}}
}

func newTestProtocol(config acpSessionConfig) *acpProtocol {
	config.cwd = "/work"
	return newACPProtocol(config)
}

func newReadyProtocol(t *testing.T, config acpSessionConfig) *acpProtocol {
	t.Helper()
	p := newTestProtocol(config)
	initLines := p.Start()
	require.Len(t, initLines, 1)
	// Response to initialize triggers session/new
	feed(t, p, respond(1, map[string]any{"protocolVersion": 1}))
	// Response to session/new gives sessionId
	feed(t, p, respond(2, map[string]any{"sessionId": "sess-agy-1"}))
	sent(t, p)
	return p
}

func sessionUpdate(update map[string]any) map[string]any {
	return map[string]any{
		"jsonrpc": "2.0",
		"method":  "session/update",
		"params":  map[string]any{"sessionId": "sess-agy-1", "update": update},
	}
}

func TestHandshakeInitializesAndOpensSession(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})
	initLines := p.Start()
	require.Len(t, initLines, 1)

	initReq := parseLine(t, initLines[0])
	assert.Equal(t, "initialize", initReq["method"])
	assert.Equal(t, float64(1), initReq["id"])

	// Agent responds to initialize -> client sends session/new
	_, out := feed(t, p, respond(1, map[string]any{"protocolVersion": 1}))
	require.Len(t, out, 1)
	assert.Equal(t, "session/new", out[0]["method"])
	assert.Equal(t, float64(2), out[0]["id"])

	// Agent responds to session/new -> session becomes ready
	events, _ := feed(t, p, respond(2, map[string]any{
		"sessionId": "sess-1",
		"modes":     map[string]any{"currentModeId": "default"},
	}))
	assert.Equal(t, phaseReady, p.phase)
	assert.Equal(t, "sess-1", *p.sessionID)
	mode := "default"
	assert.Contains(t, events, models.StatusChanged{Key: "mode", Text: &mode})
}

func TestHandshakeResumesSessionWhenConfigured(t *testing.T) {
	saved := "sess-saved-1"
	p := newTestProtocol(acpSessionConfig{resumeSession: &saved})
	p.Start()

	// Agent responds to initialize -> client sends session/load
	_, out := feed(t, p, respond(1, map[string]any{"protocolVersion": 1}))
	require.Len(t, out, 1)
	assert.Equal(t, "session/load", out[0]["method"])
	params := out[0]["params"].(map[string]any)
	assert.Equal(t, "sess-saved-1", params["sessionId"])

	// When session/load succeeds -> session ready
	feed(t, p, respond(2, map[string]any{"sessionId": "sess-saved-1"}))
	assert.Equal(t, phaseReady, p.phase)
	assert.Equal(t, "sess-saved-1", *p.sessionID)
}

func TestHandshakeResumeFailureFallsBackToNewSession(t *testing.T) {
	saved := "sess-stale"
	p := newTestProtocol(acpSessionConfig{resumeSession: &saved})
	p.Start()
	feed(t, p, respond(1, map[string]any{"protocolVersion": 1}))

	// session/load fails with error -> falls back to session/new
	events, out := feed(t, p, respondError(2, -32000, "session not found"))
	require.Len(t, events, 1)
	notice, isNotice := events[0].(models.Notice)
	require.True(t, isNotice)
	assert.Equal(t, models.NoticeWarning, notice.Level)

	require.Len(t, out, 1)
	assert.Equal(t, "session/new", out[0]["method"])
}

func TestPromptHoldsUntilSessionReady(t *testing.T) {
	p := newTestProtocol(acpSessionConfig{})
	p.Start()

	// Encode prompt before session is ready
	line, ok, err := p.Encode(models.PromptCommand{Message: "hola"}, "req-1")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, line)
	assert.Len(t, p.queued, 1)

	// Complete handshake
	feed(t, p, respond(1, map[string]any{"protocolVersion": 1}))
	_, out := feed(t, p, respond(2, map[string]any{"sessionId": "sess-1"}))

	// Prompt was flushed upon ready
	require.Len(t, out, 1)
	assert.Equal(t, "session/prompt", out[0]["method"])
	assert.Equal(t, float64(3), out[0]["id"])
}

func TestStreamingDeltasAndTurnSettling(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	// Encode prompt
	line, ok, err := p.Encode(models.PromptCommand{Message: "test"}, "req-1")
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Empty(t, line)

	out := sent(t, p)
	require.Len(t, out, 1)
	assert.Equal(t, "session/prompt", out[0]["method"])
	assert.Equal(t, float64(3), out[0]["id"])

	// Receive thought chunk
	thoughtEvts, _ := feed(t, p, sessionUpdate(map[string]any{
		"sessionUpdate": "agent_thought_chunk",
		"content":       map[string]any{"type": "text", "text": "pensando..."},
	}))
	assert.Contains(t, thoughtEvts, models.RunStarted{})
	assert.Contains(t, thoughtEvts, models.ThinkingDelta{Delta: "pensando..."})

	// Receive message chunk
	msgEvts, _ := feed(t, p, sessionUpdate(map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content":       map[string]any{"type": "text", "text": "hola mundo"},
	}))
	assert.Contains(t, msgEvts, models.TextDelta{Delta: "hola mundo"})

	// Turn finishes
	thinking := "pensando..."
	turnEvts, _ := feed(t, p, respond(3, map[string]any{"stopReason": "end_turn"}))
	assert.Contains(t, turnEvts, models.AssistantMessage{Text: "hola mundo", Thinking: &thinking})
	assert.Contains(t, turnEvts, models.RunSettled{})
}

func TestToolCallLifecycle(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})
	_, _, err := p.Encode(models.PromptCommand{Message: "test"}, "req-1")
	require.NoError(t, err)

	// Tool call starts
	startEvts, _ := feed(t, p, sessionUpdate(map[string]any{
		"sessionUpdate": "tool_call",
		"toolCallId":    "call-1",
		"kind":          "edit",
		"title":         "Edit File",
		"rawInput":      map[string]any{"path": "main.go"},
		"status":        "in_progress",
	}))
	require.Len(t, startEvts, 2) // RunStarted + ToolStarted
	toolStarted, ok := startEvts[1].(models.ToolStarted)
	require.True(t, ok)
	assert.Equal(t, "call-1", toolStarted.CallID)
	assert.Equal(t, "Edit File", toolStarted.ToolName)

	// Tool call finishes
	finishEvts, _ := feed(t, p, sessionUpdate(map[string]any{
		"sessionUpdate": "tool_call_update",
		"toolCallId":    "call-1",
		"status":        "completed",
		"output":        "success",
	}))
	require.Len(t, finishEvts, 1)
	toolFinished, ok := finishEvts[0].(models.ToolFinished)
	require.True(t, ok)
	assert.Equal(t, "call-1", toolFinished.CallID)
	assert.False(t, toolFinished.IsError)
}

func TestRequestPermissionAutoAllowsEdits(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, out := feed(t, p, map[string]any{
		"jsonrpc": "2.0",
		"id":      10,
		"method":  "session/request_permission",
		"params": map[string]any{
			"sessionId": "sess-agy-1",
			"toolCall":  map[string]any{"kind": "edit", "title": "Edit main.go"},
			"options": []any{
				map[string]any{"kind": "allow_once", "id": "allow-1"},
				map[string]any{"kind": "reject_once", "id": "reject-1"},
			},
		},
	})
	assert.Empty(t, events)
	require.Len(t, out, 1)
	assert.Equal(t, float64(10), out[0]["id"])
	res := out[0]["result"].(map[string]any)["outcome"].(map[string]any)
	assert.Equal(t, "selected", res["outcome"])
	assert.Equal(t, "allow-1", res["optionId"])
}

func TestRequestPermissionAsksConfirmationForOtherTools(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	events, out := feed(t, p, map[string]any{
		"jsonrpc": "2.0",
		"id":      11,
		"method":  "session/request_permission",
		"params": map[string]any{
			"sessionId": "sess-agy-1",
			"toolCall":  map[string]any{"kind": "bash", "title": "Run rm -rf"},
			"options": []any{
				map[string]any{"kind": "allow_once", "id": "allow-btn"},
				map[string]any{"kind": "reject_once", "id": "reject-btn"},
			},
		},
	})
	assert.Empty(t, out)
	require.Len(t, events, 1)
	req, ok := events[0].(models.DialogRequested)
	require.True(t, ok)
	assert.Equal(t, "perm-11", req.Request.ID)

	// Answering the dialog sends selection
	_, _, err := p.Encode(models.AnswerDialogCommand{
		DialogID: "perm-11",
		Answer:   models.ConfirmedAnswer{Confirmed: true},
	}, "ans-1")
	require.NoError(t, err)

	replies := sent(t, p)
	require.Len(t, replies, 1)
	assert.Equal(t, float64(11), replies[0]["id"])
	res := replies[0]["result"].(map[string]any)["outcome"].(map[string]any)
	assert.Equal(t, "allow-btn", res["optionId"])
}

func TestSetModeCommand(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	_, _, err := p.Encode(models.SetModeCommand{Mode: models.AgentMode("accept-edits")}, "mode-1")
	require.NoError(t, err)

	out := sent(t, p)
	require.Len(t, out, 1)
	assert.Equal(t, "session/set_mode", out[0]["method"])
	params := out[0]["params"].(map[string]any)
	assert.Equal(t, "accept-edits", params["modeId"])

	// Response confirms mode
	mode := "accept-edits"
	events, _ := feed(t, p, respond(3, map[string]any{}))
	require.Len(t, events, 1)
	assert.Equal(t, models.StatusChanged{Key: "mode", Text: &mode}, events[0])
}

func TestAbortCommand(t *testing.T) {
	p := newReadyProtocol(t, acpSessionConfig{})

	_, _, err := p.Encode(models.AbortCommand{}, "abort-1")
	require.NoError(t, err)

	out := sent(t, p)
	require.Len(t, out, 1)
	assert.Equal(t, "session/cancel", out[0]["method"])
	params := out[0]["params"].(map[string]any)
	assert.Equal(t, "sess-agy-1", params["sessionId"])
}
