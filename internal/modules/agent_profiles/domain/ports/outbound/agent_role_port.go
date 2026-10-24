package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_profiles/domain/models"
)

// AgentRolePort gives access to the agents folder, where every role is a `<role>.md` file.
type AgentRolePort interface {
	// List returns the readable roles sorted by name. A missing folder is an empty list.
	List(ctx context.Context) ([]models.AgentRole, error)
	// Create writes a new role file. It never overwrites an existing role.
	Create(ctx context.Context, role models.NewAgentRole) error
}
