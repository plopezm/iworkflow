package state

import (
	"math"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// TranscriptItem is one closed entry of the conversation. The implementations are the closed set below.
type TranscriptItem interface {
	isTranscriptItem()
}

// UserItem is a message of the user, with the number of images attached to it.
type UserItem struct {
	Text   string
	Images uint32
}

// AssistantItem is a complete message of the agent.
type AssistantItem struct {
	Text     string
	Thinking *string
	Error    *string
}

// ToolItem is one tool call. IsError is nil while the tool is still running.
type ToolItem struct {
	CallID  string
	Name    string
	Args    string
	Output  string
	IsError *bool
}

// NoticeItem is a message for the user.
type NoticeItem struct {
	Level   string
	Message string
}

// StatusItem is a change of the status of the task.
type StatusItem struct {
	Status string
	Error  *string
}

// DialogItem is a dialog the agent opened. Resolved is true once it was answered or closed. ClosedByTurn is true when
// the turn ended before the dialog was answered: the user did not answer it.
type DialogItem struct {
	Request      dto.TranscriptDialog
	Resolved     bool
	ClosedByTurn bool
}

// ExitItem records that the agent process ended. Code is nil when the process had no exit code.
type ExitItem struct {
	Code *int32
}

func (UserItem) isTranscriptItem()      {}
func (AssistantItem) isTranscriptItem() {}
func (ToolItem) isTranscriptItem()      {}
func (NoticeItem) isTranscriptItem()    {}
func (StatusItem) isTranscriptItem()    {}
func (DialogItem) isTranscriptItem()    {}
func (ExitItem) isTranscriptItem()      {}

// Transcript is the conversation of a task, folded from its events: the closed items, the message being streamed,
// the extension statuses and widgets, the queued messages and the subagents. It is the Go counterpart of the Rust
// state, and Apply follows the same rules.
type Transcript struct {
	Items []TranscriptItem
	// StreamingText and StreamingThinking hold the message being generated, until the final message settles it.
	StreamingText     string
	StreamingThinking string
	// Statuses are the status lines of the extensions, and Widgets their widgets, by key.
	Statuses map[string]string
	Widgets  map[string][]string
	// QueuedSteering and QueuedFollowUp are the messages waiting to be sent to the agent.
	QueuedSteering []string
	QueuedFollowUp []string
	// SubagentsVersion grows with every subagents change, so the list of subagents knows it is stale.
	SubagentsVersion uint32
	// SubagentsActive is the last count of subagents queued, running or waiting.
	SubagentsActive uint32
}

// Apply folds one event into the transcript. Board notices carry no transcript entry and never reach it.
func (t *Transcript) Apply(event dto.TranscriptEventDTO) {
	if event.Status != nil {
		t.applyStatus(*event.Status)
		return
	}
	if event.Agent != nil {
		t.applyAgent(event.Agent)
	}
}

func (t *Transcript) applyStatus(status dto.TranscriptStatusDTO) {
	// Without a live agent nobody can answer: a server restarted does not emit ProcessExited, so the status change
	// closes the dialogs left open.
	if !status.HasLiveAgent {
		t.CloseTurn()
	}
	t.Items = append(t.Items, StatusItem{Status: status.Status, Error: status.Error})
}

func (t *Transcript) applyAgent(event dto.TranscriptAgentEvent) {
	switch typed := event.(type) {
	case dto.TranscriptRunStarted:
	case dto.TranscriptRunSettled:
		t.clearStreaming()
	case dto.TranscriptUserMessage:
		t.CloseTurn()
		t.Items = append(t.Items, UserItem{Text: typed.Text, Images: typed.Images})
	case dto.TranscriptAssistantMessage:
		t.clearStreaming()
		t.CloseTurn()
		t.Items = append(t.Items, AssistantItem{Text: typed.Text, Thinking: typed.Thinking, Error: typed.Error})
	case dto.TranscriptTextDelta:
		t.StreamingText += typed.Delta
	case dto.TranscriptThinkingDelta:
		t.StreamingThinking += typed.Delta
	case dto.TranscriptToolStarted:
		// The tools start after the text block before them is closed.
		t.clearStreaming()
		t.Items = append(t.Items, ToolItem{CallID: typed.CallID, Name: typed.ToolName, Args: typed.Args})
	case dto.TranscriptToolUpdated:
		if index, tool, found := t.toolAt(typed.CallID); found {
			tool.Output = typed.Output
			t.Items[index] = tool
		}
	case dto.TranscriptToolFinished:
		isError := typed.IsError
		if index, tool, found := t.toolAt(typed.CallID); found {
			tool.Output = typed.Output
			tool.IsError = &isError
			t.Items[index] = tool
			return
		}
		t.Items = append(t.Items, ToolItem{CallID: typed.CallID, Name: typed.ToolName, Output: typed.Output, IsError: &isError})
	case dto.TranscriptQueueChanged:
		t.QueuedSteering = typed.Steering
		t.QueuedFollowUp = typed.FollowUp
	case dto.TranscriptDialogRequested:
		t.Items = append(t.Items, DialogItem{Request: typed.Request})
	case dto.TranscriptDialogResolved:
		t.resolveDialog(typed.DialogID)
	case dto.TranscriptNotice:
		t.Items = append(t.Items, NoticeItem{Level: typed.Level, Message: typed.Message})
	case dto.TranscriptStatusChanged:
		if typed.Text != nil && *typed.Text != "" {
			t.setStatus(typed.Key, *typed.Text)
		} else {
			delete(t.Statuses, typed.Key)
		}
	case dto.TranscriptWidgetChanged:
		if len(typed.Lines) > 0 {
			t.setWidget(typed.Key, typed.Lines)
		} else {
			delete(t.Widgets, typed.Key)
		}
	case dto.TranscriptSubagentsChanged:
		t.SubagentsVersion++
		t.SubagentsActive = saturatingAdd(typed.Active, typed.Waiting)
	case dto.TranscriptUsageLimitReached:
		t.Items = append(t.Items, NoticeItem{Level: dto.NoticeLevelWarning, Message: typed.Message})
	case dto.TranscriptProcessExited:
		t.clearStreaming()
		t.CloseTurn()
		t.QueuedSteering = nil
		t.QueuedFollowUp = nil
		t.Statuses = nil
		t.Widgets = nil
		t.Items = append(t.Items, ExitItem{Code: typed.Code})
	}
}

// Dialog returns the dialog with the id, answered or not. When the id repeats, the latest one is returned; the boolean
// is false when the transcript holds no dialog with that id.
func (t Transcript) Dialog(id string) (DialogItem, bool) {
	for index := len(t.Items) - 1; index >= 0; index-- {
		if dialog, ok := t.Items[index].(DialogItem); ok && dialog.Request.ID == id {
			return dialog, true
		}
	}
	return DialogItem{}, false
}

// PendingDialog returns the most recent dialog that is still open, if there is one.
func (t Transcript) PendingDialog() (dto.TranscriptDialog, bool) {
	for index := len(t.Items) - 1; index >= 0; index-- {
		if dialog, ok := t.Items[index].(DialogItem); ok && !dialog.Resolved {
			return dialog.Request, true
		}
	}
	return dto.TranscriptDialog{}, false
}

// IsStreaming reports whether a message is being generated.
func (t Transcript) IsStreaming() bool {
	return t.StreamingText != "" || t.StreamingThinking != ""
}

func (t *Transcript) clearStreaming() {
	t.StreamingText = ""
	t.StreamingThinking = ""
}

func (t *Transcript) setStatus(key string, text string) {
	if t.Statuses == nil {
		t.Statuses = map[string]string{}
	}
	t.Statuses[key] = text
}

func (t *Transcript) setWidget(key string, lines []string) {
	if t.Widgets == nil {
		t.Widgets = map[string][]string{}
	}
	t.Widgets[key] = lines
}

// toolAt returns the position and the value of the last tool call with the id.
func (t *Transcript) toolAt(callID string) (int, ToolItem, bool) {
	for index := len(t.Items) - 1; index >= 0; index-- {
		if tool, ok := t.Items[index].(ToolItem); ok && tool.CallID == callID {
			return index, tool, true
		}
	}
	return -1, ToolItem{}, false
}

// CloseTurn ends the turn: nobody can answer the dialogs it left open, so each one closes with the turn. A dialog
// answered before keeps its answer, and is not closed by the turn.
func (t *Transcript) CloseTurn() {
	for index, item := range t.Items {
		dialog, ok := item.(DialogItem)
		if !ok || dialog.Resolved {
			continue
		}
		dialog.Resolved = true
		dialog.ClosedByTurn = true
		t.Items[index] = dialog
	}
}

// resolveDialog marks as answered the dialog with the id.
func (t *Transcript) resolveDialog(dialogID string) {
	for index, item := range t.Items {
		dialog, ok := item.(DialogItem)
		if !ok || dialog.Request.ID != dialogID {
			continue
		}
		dialog.Resolved = true
		t.Items[index] = dialog
	}
}

// saturatingAdd adds two counters and stops at the largest value instead of wrapping around.
func saturatingAdd(left uint32, right uint32) uint32 {
	if left > math.MaxUint32-right {
		return math.MaxUint32
	}
	return left + right
}

// FoldTranscript builds the transcript of a history, in order. It also returns the cursor of the live stream: the last
// sequence number that was persisted, nil when there is none.
func FoldTranscript(history []dto.TranscriptEventDTO) (Transcript, *int64) {
	var transcript Transcript
	var cursor *int64
	for _, event := range history {
		transcript.Apply(event)
		if event.Seq != nil {
			seq := *event.Seq
			cursor = &seq
		}
	}
	return transcript, cursor
}
