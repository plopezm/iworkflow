package outbound

import (
	"context"

	"github.com/google/uuid"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// WorkspaceRootPort resolves the directory a project scope is rooted at.
type WorkspaceRootPort interface {
	// Resolve returns the name and root directory, or WorkspaceNotFound.
	Resolve(ctx context.Context, scope models.ProjectScope) (models.WorkspaceRoot, error)
}

// WorkspaceRootLookup finds the root of a workspace or a task by identifier. The composition root
// wires it from the workspaces and tasks modules; this module only defines the port.
type WorkspaceRootLookup interface {
	// Find returns nil and no error when the identifier is unknown.
	Find(ctx context.Context, id uuid.UUID) (*models.WorkspaceRoot, error)
}
