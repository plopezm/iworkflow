package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// TaskEventKind is the content of a task event. The implementations are the closed set below.
type TaskEventKind interface {
	isTaskEventKind()
	// IsPersistent reports whether the event is stored in the history (and so gets a sequence number).
	IsPersistent() bool
}

// AgentEventKind wraps an event of the agent.
type AgentEventKind struct{ Event AgentEvent }

// StatusEventKind records a status change of the task.
type StatusEventKind struct {
	Status TaskStatus
	Error  *string
}

// ChangedEventKind tells the board that a backlog task was edited. It is not persisted.
type ChangedEventKind struct{}

// RemovedEventKind tells the board that a backlog task was deleted. It is not persisted.
type RemovedEventKind struct{}

func (AgentEventKind) isTaskEventKind()   {}
func (StatusEventKind) isTaskEventKind()  {}
func (ChangedEventKind) isTaskEventKind() {}
func (RemovedEventKind) isTaskEventKind() {}

// IsPersistent follows the agent event for agent events; status changes are always stored.
func (k AgentEventKind) IsPersistent() bool { return k.Event.IsPersistent() }

// IsPersistent is always true for a status change.
func (StatusEventKind) IsPersistent() bool { return true }

// IsPersistent is false: changes of backlog tasks are live notifications only.
func (ChangedEventKind) IsPersistent() bool { return false }

// IsPersistent is false: removals of backlog tasks are live notifications only.
func (RemovedEventKind) IsPersistent() bool { return false }

// StatusKind is a shortcut to build a status event.
func StatusKind(status TaskStatus, errorText *string) TaskEventKind {
	return StatusEventKind{Status: status, Error: errorText}
}

// TaskEvent is an event of a task. Seq exists only for persisted events (a global increasing order).
type TaskEvent struct {
	TaskID    TaskID
	Seq       *int64
	Kind      TaskEventKind
	CreatedAt time.Time
}

// EphemeralTaskEvent builds an event that is published but not persisted.
func EphemeralTaskEvent(taskID TaskID, kind TaskEventKind, now time.Time) TaskEvent {
	return TaskEvent{TaskID: taskID, Kind: kind, CreatedAt: now}
}

// sourceWire reads the discriminator of a persisted task event kind ({"source": "...", ...}).
type sourceWire struct {
	Source string `json:"source"`
}

type agentSourceWire struct {
	Source string          `json:"source"`
	Event  json.RawMessage `json:"event"`
}

type statusSourceWire struct {
	Source string  `json:"source"`
	Status string  `json:"status"`
	Error  *string `json:"error"`
}

// EncodeTaskEventKind returns the persisted JSON form of a kind.
func EncodeTaskEventKind(kind TaskEventKind) ([]byte, error) {
	var wire any
	switch typed := kind.(type) {
	case AgentEventKind:
		event, err := EncodeAgentEvent(typed.Event)
		if err != nil {
			return nil, err
		}
		wire = agentSourceWire{Source: "agent", Event: event}
	case StatusEventKind:
		wire = statusSourceWire{Source: "status", Status: string(typed.Status), Error: typed.Error}
	case ChangedEventKind:
		wire = sourceWire{Source: "changed"}
	case RemovedEventKind:
		wire = sourceWire{Source: "removed"}
	default:
		return nil, fmt.Errorf("unknown task event kind %T", kind)
	}
	data, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("encode task event kind: %w", err)
	}
	return data, nil
}

// DecodeTaskEventKind reads the persisted JSON form of a kind.
func DecodeTaskEventKind(data []byte) (TaskEventKind, error) {
	var header sourceWire
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, fmt.Errorf("decode task event source: %w", err)
	}
	switch header.Source {
	case "agent":
		var wire agentSourceWire
		if err := json.Unmarshal(data, &wire); err != nil {
			return nil, fmt.Errorf("decode task event: %w", err)
		}
		event, err := DecodeAgentEvent(wire.Event)
		if err != nil {
			return nil, err
		}
		return AgentEventKind{Event: event}, nil
	case "status":
		var wire statusSourceWire
		if err := json.Unmarshal(data, &wire); err != nil {
			return nil, fmt.Errorf("decode task event: %w", err)
		}
		return StatusEventKind{Status: TaskStatus(wire.Status), Error: wire.Error}, nil
	case "changed":
		return ChangedEventKind{}, nil
	case "removed":
		return RemovedEventKind{}, nil
	default:
		return nil, fmt.Errorf("unknown task event source %q", header.Source)
	}
}
