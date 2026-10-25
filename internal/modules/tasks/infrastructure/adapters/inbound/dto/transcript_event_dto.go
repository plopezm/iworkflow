package dto

import (
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
)

// NoticeLevelWarning is the level of the notice the transcript shows when the provider reached a usage limit.
const NoticeLevelWarning = "warning"

// TranscriptEventDTO is one event of a task as the transcript reads it: the history and the live stream, without
// the board notices. Seq is absent for the events that are not persisted. Exactly one of Agent and Status is set.
type TranscriptEventDTO struct {
	Seq    *int64
	Agent  TranscriptAgentEvent
	Status *TranscriptStatusDTO
}

// TranscriptStatusDTO is a status change of the task. HasLiveAgent is the domain's answer to whether the status keeps
// an agent running.
type TranscriptStatusDTO struct {
	Status       string
	HasLiveAgent bool
	Error        *string
}

// TranscriptAgentEvent is an agent event as the transcript reads it. The implementations are the closed set below.
type TranscriptAgentEvent interface {
	isTranscriptAgentEvent()
}

// TranscriptDialog is the part of a dialog request the transcript shows.
type TranscriptDialog struct {
	ID    string
	Title string
}

// Transcript agent events. Each one mirrors the domain event of the same meaning, with the fields the transcript uses.
type (
	TranscriptRunStarted  struct{}
	TranscriptRunSettled  struct{}
	TranscriptUserMessage struct {
		Text   string
		Images uint32
	}
	TranscriptAssistantMessage struct {
		Text     string
		Thinking *string
		Error    *string
	}
	TranscriptTextDelta     struct{ Delta string }
	TranscriptThinkingDelta struct{ Delta string }
	TranscriptToolStarted   struct {
		CallID   string
		ToolName string
		Args     string
	}
	TranscriptToolUpdated struct {
		CallID string
		Output string
	}
	TranscriptToolFinished struct {
		CallID   string
		ToolName string
		Output   string
		IsError  bool
	}
	TranscriptQueueChanged struct {
		Steering []string
		FollowUp []string
	}
	TranscriptDialogRequested struct{ Request TranscriptDialog }
	TranscriptDialogResolved  struct{ DialogID string }
	TranscriptNotice          struct {
		Level   string
		Message string
	}
	// TranscriptStatusChanged has no Text when the status line is cleared.
	TranscriptStatusChanged struct {
		Key  string
		Text *string
	}
	// TranscriptWidgetChanged has no Lines when the widget is cleared.
	TranscriptWidgetChanged struct {
		Key   string
		Lines []string
	}
	TranscriptSubagentsChanged struct {
		Active  uint32
		Waiting uint32
	}
	TranscriptUsageLimitReached struct{ Message string }
	TranscriptProcessExited     struct{ Code *int32 }
)

func (TranscriptRunStarted) isTranscriptAgentEvent()        {}
func (TranscriptRunSettled) isTranscriptAgentEvent()        {}
func (TranscriptUserMessage) isTranscriptAgentEvent()       {}
func (TranscriptAssistantMessage) isTranscriptAgentEvent()  {}
func (TranscriptTextDelta) isTranscriptAgentEvent()         {}
func (TranscriptThinkingDelta) isTranscriptAgentEvent()     {}
func (TranscriptToolStarted) isTranscriptAgentEvent()       {}
func (TranscriptToolUpdated) isTranscriptAgentEvent()       {}
func (TranscriptToolFinished) isTranscriptAgentEvent()      {}
func (TranscriptQueueChanged) isTranscriptAgentEvent()      {}
func (TranscriptDialogRequested) isTranscriptAgentEvent()   {}
func (TranscriptDialogResolved) isTranscriptAgentEvent()    {}
func (TranscriptNotice) isTranscriptAgentEvent()            {}
func (TranscriptStatusChanged) isTranscriptAgentEvent()     {}
func (TranscriptWidgetChanged) isTranscriptAgentEvent()     {}
func (TranscriptSubagentsChanged) isTranscriptAgentEvent()  {}
func (TranscriptUsageLimitReached) isTranscriptAgentEvent() {}
func (TranscriptProcessExited) isTranscriptAgentEvent()     {}

// TranscriptEventsFromDomain maps the events of a task to their transcript form, in order. Board notices are left
// out: they only tell the board to refetch.
func TranscriptEventsFromDomain(events []models.TaskEvent) ([]TranscriptEventDTO, error) {
	mapped := make([]TranscriptEventDTO, 0, len(events))
	for _, event := range events {
		item, shown, err := transcriptEventFromDomain(event)
		if err != nil {
			return nil, err
		}
		if shown {
			mapped = append(mapped, item)
		}
	}
	return mapped, nil
}

func transcriptEventFromDomain(event models.TaskEvent) (TranscriptEventDTO, bool, error) {
	switch kind := event.Kind.(type) {
	case models.AgentEventKind:
		agent, err := transcriptAgentFromDomain(kind.Event)
		if err != nil {
			return TranscriptEventDTO{}, false, err
		}
		return TranscriptEventDTO{Seq: event.Seq, Agent: agent}, true, nil
	case models.StatusEventKind:
		status := &TranscriptStatusDTO{Status: kind.Status.String(), HasLiveAgent: kind.Status.HasLiveAgent(), Error: kind.Error}
		return TranscriptEventDTO{Seq: event.Seq, Status: status}, true, nil
	case models.ChangedEventKind, models.RemovedEventKind:
		return TranscriptEventDTO{}, false, nil
	default:
		return TranscriptEventDTO{}, false, fmt.Errorf("unknown task event kind %T", kind)
	}
}

func transcriptAgentFromDomain(event models.AgentEvent) (TranscriptAgentEvent, error) {
	switch typed := event.(type) {
	case models.RunStarted:
		return TranscriptRunStarted{}, nil
	case models.RunSettled:
		return TranscriptRunSettled{}, nil
	case models.UserMessage:
		return TranscriptUserMessage{Text: typed.Text, Images: typed.Images}, nil
	case models.AssistantMessage:
		return TranscriptAssistantMessage{Text: typed.Text, Thinking: typed.Thinking, Error: typed.Error}, nil
	case models.TextDelta:
		return TranscriptTextDelta{Delta: typed.Delta}, nil
	case models.ThinkingDelta:
		return TranscriptThinkingDelta{Delta: typed.Delta}, nil
	case models.ToolStarted:
		return TranscriptToolStarted{CallID: typed.CallID, ToolName: typed.ToolName, Args: typed.Args}, nil
	case models.ToolUpdated:
		return TranscriptToolUpdated{CallID: typed.CallID, Output: typed.Output}, nil
	case models.ToolFinished:
		return TranscriptToolFinished{CallID: typed.CallID, ToolName: typed.ToolName, Output: typed.Output, IsError: typed.IsError}, nil
	case models.QueueChanged:
		return TranscriptQueueChanged{Steering: typed.Steering, FollowUp: typed.FollowUp}, nil
	case models.DialogRequested:
		request := TranscriptDialog{ID: typed.Request.ID, Title: typed.Request.Title}
		return TranscriptDialogRequested{Request: request}, nil
	case models.DialogResolved:
		return TranscriptDialogResolved{DialogID: typed.DialogID}, nil
	case models.Notice:
		return TranscriptNotice{Level: string(typed.Level), Message: typed.Message}, nil
	case models.StatusChanged:
		return TranscriptStatusChanged{Key: typed.Key, Text: typed.Text}, nil
	case models.WidgetChanged:
		var lines []string
		if typed.Lines != nil {
			lines = *typed.Lines
		}
		return TranscriptWidgetChanged{Key: typed.Key, Lines: lines}, nil
	case models.SubagentsChanged:
		return TranscriptSubagentsChanged{Active: typed.Active, Waiting: typed.Waiting}, nil
	case models.UsageLimitReached:
		return TranscriptUsageLimitReached{Message: typed.Message}, nil
	case models.ProcessExited:
		return TranscriptProcessExited{Code: typed.Code}, nil
	default:
		return nil, fmt.Errorf("unknown agent event %T", event)
	}
}
