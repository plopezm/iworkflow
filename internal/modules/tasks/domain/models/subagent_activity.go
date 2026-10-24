package models

// SubagentActivity is the activity of the subagents the main agent of a task started.
type SubagentActivity struct {
	// Active counts subagents running or queued.
	Active uint32
	// Waiting counts subagents blocked on an answer from the user.
	Waiting uint32
}

// NewSubagentActivity builds the activity from its two counters.
func NewSubagentActivity(active uint32, waiting uint32) SubagentActivity {
	return SubagentActivity{Active: active, Waiting: waiting}
}

// IsWorking reports whether at least one subagent is active.
func (a SubagentActivity) IsWorking() bool {
	return a.Active > 0
}
