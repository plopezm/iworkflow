package models

// AgentCapabilities says which commands and options a concrete agent supports. Prompt and abort are
// the minimum every agent offers, so they are not declared.
type AgentCapabilities struct {
	Steer          bool
	FollowUp       bool
	Dialogs        bool
	ModelSelection bool
	ThinkingLevels bool
	Resume         bool
	// Subagents exposes the activity of the subagents the agent started.
	Subagents bool
	// Modes allows switching between agent and plan mode while the agent runs.
	Modes bool
}

// FullAgentCapabilities is an agent with every command and option available.
func FullAgentCapabilities() AgentCapabilities {
	return AgentCapabilities{
		Steer:          true,
		FollowUp:       true,
		Dialogs:        true,
		ModelSelection: true,
		ThinkingLevels: true,
		Resume:         true,
		Subagents:      true,
		Modes:          true,
	}
}

// MinimalAgentCapabilities supports only prompt and abort.
func MinimalAgentCapabilities() AgentCapabilities {
	return AgentCapabilities{}
}
