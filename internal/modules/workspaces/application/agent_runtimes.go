package application

import (
	"context"
	"slices"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// ListAgentRuntimes lists the agents a workspace can take as its default.
type ListAgentRuntimes struct {
	runtimes outbound.AgentRuntimes
}

// NewListAgentRuntimes builds the use case.
func NewListAgentRuntimes(runtimes outbound.AgentRuntimes) *ListAgentRuntimes {
	return &ListAgentRuntimes{runtimes: runtimes}
}

// Execute returns the identifiers of the existing agents.
func (u *ListAgentRuntimes) Execute(ctx context.Context) []string {
	return u.runtimes.Installed(ctx)
}

// resolveDefaultRuntime turns the requested default agent into the stored value: empty means the system
// agent (nil); anything else must be an existing agent.
func resolveDefaultRuntime(ctx context.Context, runtimes outbound.AgentRuntimes, requested string) (*string, error) {
	if requested == "" {
		return nil, nil //nolint:nilnil // nil is the stored "system agent"
	}
	if !slices.Contains(runtimes.Installed(ctx), requested) {
		return nil, werrors.InvalidRuntime(requested) //nolint:wrapcheck // domain errors are returned unwrapped: their text is user-visible
	}
	return &requested, nil
}
