package models

// AgentMode is the working mode of the agent. Its string value is the one sent to and received from the runtime.
type AgentMode string

// Modes.
const (
	AgentModeAgent AgentMode = "agent"
	AgentModePlan  AgentMode = "plan"
)

// ParseAgentMode converts a name to a mode; the boolean is false for unknown names.
func ParseAgentMode(raw string) (AgentMode, bool) {
	switch raw {
	case string(AgentModeAgent):
		return AgentModeAgent, true
	case string(AgentModePlan):
		return AgentModePlan, true
	default:
		return "", false
	}
}
