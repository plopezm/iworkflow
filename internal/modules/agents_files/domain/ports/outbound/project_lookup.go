package outbound

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/models"
)

// ProjectLookup gives read-only access to the existing projects. Another module (workspaces)
// satisfies it; this module only defines the port.
type ProjectLookup interface {
	List(ctx context.Context) ([]models.ProjectRef, error)
	// Find returns nil and no error when the identifier is unknown.
	Find(ctx context.Context, id string) (*models.ProjectRef, error)
}
