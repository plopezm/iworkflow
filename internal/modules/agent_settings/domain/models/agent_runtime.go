// Package models holds the agent settings entities and value objects. It is plain Go: no I/O.
package models

import agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"

// AgentRuntime is the agent implementation that new tasks are launched with.
type AgentRuntime string

// Agent runtimes. The string value is the persisted identifier shown in the selector.
const (
	RuntimeGentleShell AgentRuntime = "gentle-shell"
	RuntimePi          AgentRuntime = "pi"
	RuntimeClaude      AgentRuntime = "claude"
	RuntimeCursor      AgentRuntime = "cursor"
)

// AllRuntimes lists every runtime in the order the selector shows them.
func AllRuntimes() []AgentRuntime {
	return []AgentRuntime{RuntimeGentleShell, RuntimePi, RuntimeClaude, RuntimeCursor}
}

// ParseAgentRuntime validates a runtime identifier.
func ParseAgentRuntime(raw string) (AgentRuntime, error) {
	switch AgentRuntime(raw) {
	case RuntimeGentleShell, RuntimePi, RuntimeClaude, RuntimeCursor:
		return AgentRuntime(raw), nil
	default:
		return "", agenterrors.InvalidRuntime(raw) //nolint:wrapcheck // domain constructor returns a domain error
	}
}

// RuntimeOrDefault returns the runtime, or gentle-shell when none is set.
func RuntimeOrDefault(runtime *AgentRuntime) AgentRuntime {
	if runtime == nil {
		return RuntimeGentleShell
	}
	return *runtime
}
