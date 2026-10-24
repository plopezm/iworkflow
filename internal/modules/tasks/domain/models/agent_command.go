package models

// AgentCommand is an order sent to the agent of a task. The implementations are the closed set below.
type AgentCommand interface {
	isAgentCommand()
}

// PromptCommand starts a new turn with a message.
type PromptCommand struct {
	Message string
	Images  []ImageAttachment
}

// SteerCommand injects a message into the running turn.
type SteerCommand struct {
	Message string
	Images  []ImageAttachment
}

// FollowUpCommand queues a message for after the running turn.
type FollowUpCommand struct {
	Message string
	Images  []ImageAttachment
}

// AbortCommand stops the running turn.
type AbortCommand struct{}

// SetModeCommand switches the working mode.
type SetModeCommand struct {
	Mode AgentMode
}

// AnswerDialogCommand answers a dialog the agent opened.
type AnswerDialogCommand struct {
	DialogID string
	Answer   DialogAnswer
}

func (PromptCommand) isAgentCommand()       {}
func (SteerCommand) isAgentCommand()        {}
func (FollowUpCommand) isAgentCommand()     {}
func (AbortCommand) isAgentCommand()        {}
func (SetModeCommand) isAgentCommand()      {}
func (AnswerDialogCommand) isAgentCommand() {}
