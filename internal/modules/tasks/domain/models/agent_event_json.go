package models

import (
	"encoding/json"
	"fmt"
)

// The JSON shapes below reproduce the serde output of the Rust version, field by field: absent
// options are written as null, lists as [] and the discriminator is "type" (events) or "kind"
// (dialogs and answers). Rows written by one version must be readable by the other, so do not change them.

type userMessageWire struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Images uint32 `json:"images"`
}

type assistantMessageWire struct {
	Type     string  `json:"type"`
	Text     string  `json:"text"`
	Thinking *string `json:"thinking"`
	Error    *string `json:"error"`
}

type deltaWire struct {
	Type  string `json:"type"`
	Delta string `json:"delta"`
}

type toolStartedWire struct {
	Type     string `json:"type"`
	CallID   string `json:"call_id"`
	ToolName string `json:"tool_name"`
	Args     string `json:"args"`
}

type toolUpdatedWire struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}

type toolFinishedWire struct {
	Type     string `json:"type"`
	CallID   string `json:"call_id"`
	ToolName string `json:"tool_name"`
	Output   string `json:"output"`
	IsError  bool   `json:"is_error"`
}

type queueChangedWire struct {
	Type     string   `json:"type"`
	Steering []string `json:"steering"`
	FollowUp []string `json:"follow_up"`
}

type dialogRequestedWire struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Title     string          `json:"title"`
	Kind      json.RawMessage `json:"kind"`
	TimeoutMS *uint64         `json:"timeout_ms"`
}

type dialogResolvedWire struct {
	Type     string `json:"type"`
	DialogID string `json:"dialog_id"`
}

type noticeWire struct {
	Type    string      `json:"type"`
	Level   NoticeLevel `json:"level"`
	Message string      `json:"message"`
}

type statusChangedWire struct {
	Type string  `json:"type"`
	Key  string  `json:"key"`
	Text *string `json:"text"`
}

type widgetChangedWire struct {
	Type  string    `json:"type"`
	Key   string    `json:"key"`
	Lines *[]string `json:"lines"`
}

type subagentsChangedWire struct {
	Type    string `json:"type"`
	Active  uint32 `json:"active"`
	Waiting uint32 `json:"waiting"`
}

type usageLimitReachedWire struct {
	Type     string `json:"type"`
	ResetsAt *int64 `json:"resets_at"`
	Message  string `json:"message"`
}

type processExitedWire struct {
	Type string `json:"type"`
	Code *int32 `json:"code"`
}

type typeOnlyWire struct {
	Type string `json:"type"`
}

// EncodeAgentEvent returns the persisted JSON form of an event.
func EncodeAgentEvent(event AgentEvent) ([]byte, error) {
	wire, err := agentEventWire(event)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode agent event: %w", err)
	}
	return data, nil
}

func agentEventWire(event AgentEvent) (any, error) {
	switch typed := event.(type) {
	case RunStarted:
		return typeOnlyWire{Type: "run_started"}, nil
	case RunSettled:
		return typeOnlyWire{Type: "run_settled"}, nil
	case UserMessage:
		return userMessageWire{Type: "user_message", Text: typed.Text, Images: typed.Images}, nil
	case AssistantMessage:
		return assistantMessageWire{Type: "assistant_message", Text: typed.Text, Thinking: typed.Thinking, Error: typed.Error}, nil
	case TextDelta:
		return deltaWire{Type: "text_delta", Delta: typed.Delta}, nil
	case ThinkingDelta:
		return deltaWire{Type: "thinking_delta", Delta: typed.Delta}, nil
	case ToolStarted:
		return toolStartedWire{Type: "tool_started", CallID: typed.CallID, ToolName: typed.ToolName, Args: typed.Args}, nil
	case ToolUpdated:
		return toolUpdatedWire{Type: "tool_updated", CallID: typed.CallID, Output: typed.Output}, nil
	case ToolFinished:
		return toolFinishedWire{Type: "tool_finished", CallID: typed.CallID, ToolName: typed.ToolName, Output: typed.Output, IsError: typed.IsError}, nil
	case QueueChanged:
		return queueChangedWire{Type: "queue_changed", Steering: nonNilStrings(typed.Steering), FollowUp: nonNilStrings(typed.FollowUp)}, nil
	case DialogRequested:
		kind, err := encodeDialogKind(typed.Request.Kind)
		if err != nil {
			return nil, err
		}
		return dialogRequestedWire{Type: "dialog_requested", ID: typed.Request.ID, Title: typed.Request.Title, Kind: kind, TimeoutMS: typed.Request.TimeoutMS}, nil
	case DialogResolved:
		return dialogResolvedWire{Type: "dialog_resolved", DialogID: typed.DialogID}, nil
	case Notice:
		return noticeWire{Type: "notice", Level: typed.Level, Message: typed.Message}, nil
	case StatusChanged:
		return statusChangedWire{Type: "status_changed", Key: typed.Key, Text: typed.Text}, nil
	case WidgetChanged:
		return widgetChangedWire{Type: "widget_changed", Key: typed.Key, Lines: typed.Lines}, nil
	case SubagentsChanged:
		return subagentsChangedWire{Type: "subagents_changed", Active: typed.Active, Waiting: typed.Waiting}, nil
	case UsageLimitReached:
		return usageLimitReachedWire{Type: "usage_limit_reached", ResetsAt: typed.ResetsAt, Message: typed.Message}, nil
	case ProcessExited:
		return processExitedWire{Type: "process_exited", Code: typed.Code}, nil
	default:
		return nil, fmt.Errorf("unknown agent event %T", event)
	}
}

// DecodeAgentEvent reads the persisted JSON form of an event.
func DecodeAgentEvent(data []byte) (AgentEvent, error) {
	var header typeOnlyWire
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("decode agent event type: %w", err)
	}
	switch header.Type {
	case "run_started":
		return RunStarted{}, nil
	case "run_settled":
		return RunSettled{}, nil
	case "user_message":
		var wire userMessageWire
		return decodeInto(data, &wire, func() AgentEvent {
			return UserMessage{Text: wire.Text, Images: wire.Images}
		})
	case "assistant_message":
		var wire assistantMessageWire
		return decodeInto(data, &wire, func() AgentEvent {
			return AssistantMessage{Text: wire.Text, Thinking: wire.Thinking, Error: wire.Error}
		})
	case "text_delta":
		var wire deltaWire
		return decodeInto(data, &wire, func() AgentEvent { return TextDelta{Delta: wire.Delta} })
	case "thinking_delta":
		var wire deltaWire
		return decodeInto(data, &wire, func() AgentEvent { return ThinkingDelta{Delta: wire.Delta} })
	case "tool_started":
		var wire toolStartedWire
		return decodeInto(data, &wire, func() AgentEvent {
			return ToolStarted{CallID: wire.CallID, ToolName: wire.ToolName, Args: wire.Args}
		})
	case "tool_updated":
		var wire toolUpdatedWire
		return decodeInto(data, &wire, func() AgentEvent { return ToolUpdated{CallID: wire.CallID, Output: wire.Output} })
	case "tool_finished":
		var wire toolFinishedWire
		return decodeInto(data, &wire, func() AgentEvent {
			return ToolFinished{CallID: wire.CallID, ToolName: wire.ToolName, Output: wire.Output, IsError: wire.IsError}
		})
	case "queue_changed":
		var wire queueChangedWire
		return decodeInto(data, &wire, func() AgentEvent {
			return QueueChanged{Steering: wire.Steering, FollowUp: wire.FollowUp}
		})
	case "dialog_requested":
		var wire dialogRequestedWire
		return decodeDialogRequested(data, &wire)
	case "dialog_resolved":
		var wire dialogResolvedWire
		return decodeInto(data, &wire, func() AgentEvent { return DialogResolved{DialogID: wire.DialogID} })
	case "notice":
		var wire noticeWire
		return decodeInto(data, &wire, func() AgentEvent { return Notice{Level: wire.Level, Message: wire.Message} })
	case "status_changed":
		var wire statusChangedWire
		return decodeInto(data, &wire, func() AgentEvent { return StatusChanged{Key: wire.Key, Text: wire.Text} })
	case "widget_changed":
		var wire widgetChangedWire
		return decodeInto(data, &wire, func() AgentEvent { return WidgetChanged{Key: wire.Key, Lines: wire.Lines} })
	case "subagents_changed":
		var wire subagentsChangedWire
		return decodeInto(data, &wire, func() AgentEvent {
			return SubagentsChanged{Active: wire.Active, Waiting: wire.Waiting}
		})
	case "usage_limit_reached":
		var wire usageLimitReachedWire
		return decodeInto(data, &wire, func() AgentEvent {
			return UsageLimitReached{ResetsAt: wire.ResetsAt, Message: wire.Message}
		})
	case "process_exited":
		var wire processExitedWire
		return decodeInto(data, &wire, func() AgentEvent { return ProcessExited{Code: wire.Code} })
	default:
		return nil, fmt.Errorf("unknown agent event type %q", header.Type)
	}
}

// decodeInto unmarshals data into wire and builds the event from it.
func decodeInto(data []byte, wire any, build func() AgentEvent) (AgentEvent, error) {
	if err := json.Unmarshal(data, wire); err != nil {
		return nil, fmt.Errorf("decode agent event: %w", err)
	}
	return build(), nil
}

func decodeDialogRequested(data []byte, wire *dialogRequestedWire) (AgentEvent, error) {
	if err := json.Unmarshal(data, wire); err != nil {
		return nil, fmt.Errorf("decode agent event: %w", err)
	}
	kind, err := decodeDialogKind(wire.Kind)
	if err != nil {
		return nil, err
	}
	return DialogRequested{Request: DialogRequest{ID: wire.ID, Title: wire.Title, Kind: kind, TimeoutMS: wire.TimeoutMS}}, nil
}

type dialogKindHeader struct {
	Kind string `json:"kind"`
}

type selectDialogWire struct {
	Kind    string   `json:"kind"`
	Options []string `json:"options"`
}

type confirmDialogWire struct {
	Kind    string  `json:"kind"`
	Message *string `json:"message"`
}

type inputDialogWire struct {
	Kind        string  `json:"kind"`
	Placeholder *string `json:"placeholder"`
}

type editorDialogWire struct {
	Kind    string  `json:"kind"`
	Prefill *string `json:"prefill"`
}

func encodeDialogKind(kind DialogKind) (json.RawMessage, error) {
	var wire any
	switch typed := kind.(type) {
	case SelectDialog:
		wire = selectDialogWire{Kind: "select", Options: nonNilStrings(typed.Options)}
	case ConfirmDialog:
		wire = confirmDialogWire{Kind: "confirm", Message: typed.Message}
	case InputDialog:
		wire = inputDialogWire{Kind: "input", Placeholder: typed.Placeholder}
	case EditorDialog:
		wire = editorDialogWire{Kind: "editor", Prefill: typed.Prefill}
	default:
		return nil, fmt.Errorf("unknown dialog kind %T", kind)
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode dialog kind: %w", err)
	}
	return data, nil
}

func decodeDialogKind(data json.RawMessage) (DialogKind, error) {
	var header dialogKindHeader
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("decode dialog kind: %w", err)
	}
	switch header.Kind {
	case "select":
		var wire selectDialogWire
		if err := json.Unmarshal(data, &wire); err != nil {
			return nil, fmt.Errorf("decode dialog kind: %w", err)
		}
		return SelectDialog{Options: wire.Options}, nil
	case "confirm":
		var wire confirmDialogWire
		if err := json.Unmarshal(data, &wire); err != nil {
			return nil, fmt.Errorf("decode dialog kind: %w", err)
		}
		return ConfirmDialog{Message: wire.Message}, nil
	case "input":
		var wire inputDialogWire
		if err := json.Unmarshal(data, &wire); err != nil {
			return nil, fmt.Errorf("decode dialog kind: %w", err)
		}
		return InputDialog{Placeholder: wire.Placeholder}, nil
	case "editor":
		var wire editorDialogWire
		if err := json.Unmarshal(data, &wire); err != nil {
			return nil, fmt.Errorf("decode dialog kind: %w", err)
		}
		return EditorDialog{Prefill: wire.Prefill}, nil
	default:
		return nil, fmt.Errorf("unknown dialog kind %q", header.Kind)
	}
}

// nonNilStrings keeps empty lists as [] in the JSON output, never null.
func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
