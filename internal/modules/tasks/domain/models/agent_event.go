package models

// NoticeLevel is the severity of a notice shown to the user.
type NoticeLevel string

// Notice levels.
const (
	NoticeInfo    NoticeLevel = "info"
	NoticeWarning NoticeLevel = "warning"
	NoticeError   NoticeLevel = "error"
)

// DialogKind is the kind of dialog the agent (or an extension) asks the user. The implementations are the closed set below.
type DialogKind interface {
	isDialogKind()
}

// SelectDialog asks the user to pick one of the options.
type SelectDialog struct{ Options []string }

// ConfirmDialog asks a yes or no question.
type ConfirmDialog struct{ Message *string }

// InputDialog asks for a line of text.
type InputDialog struct{ Placeholder *string }

// EditorDialog asks for a multi-line text.
type EditorDialog struct{ Prefill *string }

func (SelectDialog) isDialogKind()  {}
func (ConfirmDialog) isDialogKind() {}
func (InputDialog) isDialogKind()   {}
func (EditorDialog) isDialogKind()  {}

// DialogRequest is a dialog the agent opened.
type DialogRequest struct {
	ID    string
	Title string
	Kind  DialogKind
	// TimeoutMS, when set, is how long the agent waits before it considers the dialog expired.
	TimeoutMS *uint64
}

// DialogAnswer is the user's answer to a dialog. The implementations are the closed set below.
type DialogAnswer interface {
	isDialogAnswer()
}

// ValueAnswer is a text answer (select, input, editor).
type ValueAnswer struct{ Value string }

// ConfirmedAnswer is the answer to a confirm dialog.
type ConfirmedAnswer struct{ Confirmed bool }

// CancelledAnswer means the user closed the dialog without answering.
type CancelledAnswer struct{}

func (ValueAnswer) isDialogAnswer()     {}
func (ConfirmedAnswer) isDialogAnswer() {}
func (CancelledAnswer) isDialogAnswer() {}

// AgentEvent is an event normalized from an agent running in RPC mode. The implementations are the
// closed set below; their JSON form is in agent_event_json.go and must not change (it is persisted).
type AgentEvent interface {
	isAgentEvent()
	// IsPersistent reports whether the event belongs to the history. The rest is only emitted live.
	IsPersistent() bool
}

// RunStarted means the agent started a turn.
type RunStarted struct{}

// RunSettled means the agent finished a turn.
type RunSettled struct{}

// UserMessage is a message sent by the user.
type UserMessage struct {
	Text string
	// Images is the number of images attached to the message.
	Images uint32
}

// AssistantMessage is a complete message of the agent.
type AssistantMessage struct {
	Text     string
	Thinking *string
	Error    *string
}

// TextDelta is a piece of an assistant message being streamed.
type TextDelta struct{ Delta string }

// ThinkingDelta is a piece of reasoning being streamed.
type ThinkingDelta struct{ Delta string }

// ToolStarted means a tool call began.
type ToolStarted struct {
	CallID   string
	ToolName string
	Args     string
}

// ToolUpdated carries partial output of a running tool call.
type ToolUpdated struct {
	CallID string
	Output string
}

// ToolFinished means a tool call ended.
type ToolFinished struct {
	CallID   string
	ToolName string
	Output   string
	IsError  bool
}

// QueueChanged is the list of messages waiting to be sent to the agent.
type QueueChanged struct {
	Steering []string
	FollowUp []string
}

// DialogRequested means the agent opened a dialog.
type DialogRequested struct{ Request DialogRequest }

// DialogResolved means a dialog was answered or closed.
type DialogResolved struct{ DialogID string }

// Notice is a message for the user.
type Notice struct {
	Level   NoticeLevel
	Message string
}

// StatusChanged updates a status line of the agent.
type StatusChanged struct {
	Key  string
	Text *string
}

// WidgetChanged updates a widget of the agent.
type WidgetChanged struct {
	Key   string
	Lines *[]string
}

// SubagentsChanged is the new activity of the subagents.
type SubagentsChanged struct {
	Active  uint32
	Waiting uint32
}

// UsageLimitReached means the provider rejected the request for a usage limit. ResetsAt is the renewal
// instant as epoch seconds, when the provider gave one.
type UsageLimitReached struct {
	ResetsAt *int64
	Message  string
}

// ProcessExited means the agent process ended.
type ProcessExited struct {
	Code *int32
}

func (RunStarted) isAgentEvent()        {}
func (RunSettled) isAgentEvent()        {}
func (UserMessage) isAgentEvent()       {}
func (AssistantMessage) isAgentEvent()  {}
func (TextDelta) isAgentEvent()         {}
func (ThinkingDelta) isAgentEvent()     {}
func (ToolStarted) isAgentEvent()       {}
func (ToolUpdated) isAgentEvent()       {}
func (ToolFinished) isAgentEvent()      {}
func (QueueChanged) isAgentEvent()      {}
func (DialogRequested) isAgentEvent()   {}
func (DialogResolved) isAgentEvent()    {}
func (Notice) isAgentEvent()            {}
func (StatusChanged) isAgentEvent()     {}
func (WidgetChanged) isAgentEvent()     {}
func (SubagentsChanged) isAgentEvent()  {}
func (UsageLimitReached) isAgentEvent() {}
func (ProcessExited) isAgentEvent()     {}

// IsPersistent is false for streaming deltas and transient state; the rest form the history.
func (RunStarted) IsPersistent() bool        { return true }
func (RunSettled) IsPersistent() bool        { return true }
func (UserMessage) IsPersistent() bool       { return true }
func (AssistantMessage) IsPersistent() bool  { return true }
func (TextDelta) IsPersistent() bool         { return false }
func (ThinkingDelta) IsPersistent() bool     { return false }
func (ToolStarted) IsPersistent() bool       { return true }
func (ToolUpdated) IsPersistent() bool       { return false }
func (ToolFinished) IsPersistent() bool      { return true }
func (QueueChanged) IsPersistent() bool      { return false }
func (DialogRequested) IsPersistent() bool   { return true }
func (DialogResolved) IsPersistent() bool    { return true }
func (Notice) IsPersistent() bool            { return true }
func (StatusChanged) IsPersistent() bool     { return false }
func (WidgetChanged) IsPersistent() bool     { return false }
func (SubagentsChanged) IsPersistent() bool  { return false }
func (UsageLimitReached) IsPersistent() bool { return true }
func (ProcessExited) IsPersistent() bool     { return true }
