package models

// SubagentStatus is the state of a subagent started by the main agent.
type SubagentStatus string

// Subagent states.
const (
	SubagentQueued    SubagentStatus = "queued"
	SubagentRunning   SubagentStatus = "running"
	SubagentWaiting   SubagentStatus = "waiting"
	SubagentCompleted SubagentStatus = "completed"
	SubagentFailed    SubagentStatus = "failed"
	SubagentCancelled SubagentStatus = "cancelled"
	SubagentTimedOut  SubagentStatus = "timed_out"
	SubagentUnknown   SubagentStatus = "unknown"
)

// IsActive reports whether the subagent is queued, running or waiting.
func (s SubagentStatus) IsActive() bool {
	return s == SubagentQueued || s == SubagentRunning || s == SubagentWaiting
}

// SubagentThreadItem is one entry of the transcript of a subagent. The implementations are the closed set below.
type SubagentThreadItem interface {
	isSubagentThreadItem()
}

// SubagentText is assistant text.
type SubagentText struct{ Text string }

// SubagentThinking is reasoning text.
type SubagentThinking struct{ Text string }

// SubagentNote is an informative line.
type SubagentNote struct{ Text string }

// SubagentTool is a tool call, running or finished.
type SubagentTool struct {
	Name    string
	Args    *string
	Running bool
	IsError bool
	Output  *string
}

func (SubagentText) isSubagentThreadItem()     {}
func (SubagentThinking) isSubagentThreadItem() {}
func (SubagentNote) isSubagentThreadItem()     {}
func (SubagentTool) isSubagentThreadItem()     {}

// SubagentTask is the last known state of a subagent of a task.
type SubagentTask struct {
	ID             string
	Agent          string
	Label          *string
	Prompt         *string
	Status         SubagentStatus
	CreatedAt      *int64
	StartedAt      *int64
	EndedAt        *int64
	LastStep       *string
	LastActivityAt *int64
	Turns          uint32
	ToolCalls      uint32
	Error          *string
	Thread         []SubagentThreadItem
}
