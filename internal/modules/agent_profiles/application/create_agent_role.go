//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// CreateAgentRole creates a role file, refusing names that already exist.
type CreateAgentRole struct {
	roles outbound.AgentRolePort
}

// NewCreateAgentRole builds the use case.
func NewCreateAgentRole(roles outbound.AgentRolePort) *CreateAgentRole {
	return &CreateAgentRole{roles: roles}
}

// Execute checks the name against the existing roles and creates the file when it is free.
func (u *CreateAgentRole) Execute(ctx context.Context, role models.NewAgentRole) error {
	existing, err := u.roles.List(ctx)
	if err != nil {
		return err
	}
	for _, current := range existing {
		if current.Name == role.Name {
			return agenterrors.RoleAlreadyExists(role.Name.String())
		}
	}
	return u.roles.Create(ctx, role)
}
