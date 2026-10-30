//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// GetWorkspace returns one workspace by identifier.
type GetWorkspace struct {
	repository outbound.WorkspaceRepository
}

// NewGetWorkspace builds the use case.
func NewGetWorkspace(repository outbound.WorkspaceRepository) *GetWorkspace {
	return &GetWorkspace{repository: repository}
}

// Execute returns the workspace, or NotFound when there is none.
func (u *GetWorkspace) Execute(ctx context.Context, id models.WorkspaceID) (models.Workspace, error) {
	found, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return models.Workspace{}, err
	}
	if found == nil {
		return models.Workspace{}, werrors.NotFound(id.String())
	}
	return *found, nil
}
