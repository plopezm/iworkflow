// Package dto holds the client/server shapes of the tasks module. They are plain data with JSON tags
// matching the Rust serde output, and the mapping from and to the domain lives next to them.
package dto

import (
	"encoding/json"
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskEventDTO is one event as the live streams send it to the browser. Seq is absent for the events
// that are not persisted.
//
// This is not the stored form: the database keeps the flat agent events (see persistence), while the
// browser gets DialogRequested nested under "request" and UsageLimitReached with an RFC 3339 instant.
type TaskEventDTO struct {
	Seq       *int64        `json:"seq"`
	CreatedAt string        `json:"created_at"`
	Kind      TaskEventKind `json:"kind"`
}

// TaskEventKind is the payload of an event. Source selects the variant: "agent" carries Event,
// "status" carries Status and Error, and "changed" and "removed" carry nothing.
type TaskEventKind struct {
	Source string
	Event  map[string]any
	Status string
	Error  *string
}

// MarshalJSON writes the variant the way serde does, with explicit nulls and no omitted fields.
func (k TaskEventKind) MarshalJSON() ([]byte, error) {
	var wire map[string]any
	switch k.Source {
	case "agent":
		wire = map[string]any{"source": "agent", "event": k.Event}
	case "status":
		wire = map[string]any{"source": "status", "status": k.Status, "error": k.Error}
	default:
		wire = map[string]any{"source": k.Source}
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode task event kind: %w", err)
	}
	return data, nil
}

// TaskEventFromDomain maps an event to its browser form.
func TaskEventFromDomain(event models.TaskEvent) (TaskEventDTO, error) {
	kind, err := kindFromDomain(event.Kind)
	if err != nil {
		return TaskEventDTO{}, err
	}
	return TaskEventDTO{
		Seq:       event.Seq,
		CreatedAt: event.CreatedAt.UTC().Format(timestampLayout),
		Kind:      kind,
	}, nil
}

func kindFromDomain(kind models.TaskEventKind) (TaskEventKind, error) {
	switch typed := kind.(type) {
	case models.AgentEventKind:
		event, err := agentEventToWire(typed.Event)
		if err != nil {
			return TaskEventKind{}, err
		}
		return TaskEventKind{Source: "agent", Event: event}, nil
	case models.StatusEventKind:
		return TaskEventKind{Source: "status", Status: typed.Status.String(), Error: typed.Error}, nil
	case models.ChangedEventKind:
		return TaskEventKind{Source: "changed"}, nil
	case models.RemovedEventKind:
		return TaskEventKind{Source: "removed"}, nil
	default:
		return TaskEventKind{}, fmt.Errorf("unknown task event kind %T", kind)
	}
}

// agentEventToWire builds the browser form of an agent event. The keys are the ones the stream
// protocol has always used; the nested maps keep the nulls and the empty lists explicit.
func agentEventToWire(event models.AgentEvent) (map[string]any, error) {
	switch typed := event.(type) {
	case models.RunStarted:
		return map[string]any{"type": "run_started"}, nil
	case models.RunSettled:
		return map[string]any{"type": "run_settled"}, nil
	case models.UserMessage:
		return map[string]any{"type": "user_message", "text": typed.Text, "images": typed.Images}, nil
	case models.AssistantMessage:
		return map[string]any{"type": "assistant_message", "text": typed.Text, "thinking": typed.Thinking, "error": typed.Error}, nil
	case models.TextDelta:
		return map[string]any{"type": "text_delta", "delta": typed.Delta}, nil
	case models.ThinkingDelta:
		return map[string]any{"type": "thinking_delta", "delta": typed.Delta}, nil
	case models.ToolStarted:
		return map[string]any{"type": "tool_started", "call_id": typed.CallID, "tool_name": typed.ToolName, "args": typed.Args}, nil
	case models.ToolUpdated:
		return map[string]any{"type": "tool_updated", "call_id": typed.CallID, "output": typed.Output}, nil
	case models.ToolFinished:
		return map[string]any{"type": "tool_finished", "call_id": typed.CallID, "tool_name": typed.ToolName, "output": typed.Output, "is_error": typed.IsError}, nil
	case models.QueueChanged:
		return map[string]any{"type": "queue_changed", "steering": nonNil(typed.Steering), "follow_up": nonNil(typed.FollowUp)}, nil
	case models.DialogRequested:
		kind, err := dialogKindToWire(typed.Request.Kind)
		if err != nil {
			return nil, err
		}
		request := map[string]any{"id": typed.Request.ID, "title": typed.Request.Title, "kind": kind, "timeout_ms": typed.Request.TimeoutMS}
		return map[string]any{"type": "dialog_requested", "request": request}, nil
	case models.DialogResolved:
		return map[string]any{"type": "dialog_resolved", "dialog_id": typed.DialogID}, nil
	case models.Notice:
		return map[string]any{"type": "notice", "level": string(typed.Level), "message": typed.Message}, nil
	case models.StatusChanged:
		return map[string]any{"type": "status_changed", "key": typed.Key, "text": typed.Text}, nil
	case models.WidgetChanged:
		return map[string]any{"type": "widget_changed", "key": typed.Key, "lines": typed.Lines}, nil
	case models.SubagentsChanged:
		return map[string]any{"type": "subagents_changed", "active": typed.Active, "waiting": typed.Waiting}, nil
	case models.UsageLimitReached:
		return map[string]any{"type": "usage_limit_reached", "resets_at": resetsAtText(typed.ResetsAt), "message": typed.Message}, nil
	case models.ProcessExited:
		return map[string]any{"type": "process_exited", "code": typed.Code}, nil
	default:
		return nil, fmt.Errorf("unknown agent event %T", event)
	}
}

// resetsAtText converts the epoch seconds of the provider to RFC 3339; nil when absent or out of range.
func resetsAtText(epochSecs *int64) *string {
	if epochSecs == nil {
		return nil
	}
	at, ok := models.FromEpochSecs(*epochSecs)
	if !ok {
		return nil
	}
	text := at.Format(timestampLayout)
	return &text
}

func dialogKindToWire(kind models.DialogKind) (map[string]any, error) {
	switch typed := kind.(type) {
	case models.SelectDialog:
		return map[string]any{"kind": "select", "options": nonNil(typed.Options)}, nil
	case models.ConfirmDialog:
		return map[string]any{"kind": "confirm", "message": typed.Message}, nil
	case models.InputDialog:
		return map[string]any{"kind": "input", "placeholder": typed.Placeholder}, nil
	case models.EditorDialog:
		return map[string]any{"kind": "editor", "prefill": typed.Prefill}, nil
	default:
		return nil, fmt.Errorf("unknown dialog kind %T", kind)
	}
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
