package outbound

import "context"

// AgentRuntimes lists the agents that exist, so a workspace can choose one as its default. It is
// implemented over the agent_settings module and wired by the composition root: this module never
// imports it.
type AgentRuntimes interface {
	// Installed returns the identifiers of the agents that can be launched, in selector order.
	Installed(ctx context.Context) []string
}
