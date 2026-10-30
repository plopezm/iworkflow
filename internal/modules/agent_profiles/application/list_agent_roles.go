//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// ListAgentRoles lists the agent roles that exist in the agents folder.
type ListAgentRoles struct {
	roles outbound.AgentRolePort
}

// NewListAgentRoles builds the use case.
func NewListAgentRoles(roles outbound.AgentRolePort) *ListAgentRoles {
	return &ListAgentRoles{roles: roles}
}

// Execute returns the roles sorted by name.
func (u *ListAgentRoles) Execute(ctx context.Context) ([]models.AgentRole, error) {
	return u.roles.List(ctx)
}
