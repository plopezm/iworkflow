//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"sort"
	"strings"

	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// ListWorkspaces returns the registered workspaces sorted by name, case-insensitively.
type ListWorkspaces struct {
	repository outbound.WorkspaceRepository
}

// NewListWorkspaces builds the use case.
func NewListWorkspaces(repository outbound.WorkspaceRepository) *ListWorkspaces {
	return &ListWorkspaces{repository: repository}
}

// Execute lists the workspaces. The sort is stable, so ties keep the repository order.
func (u *ListWorkspaces) Execute(ctx context.Context) ([]models.Workspace, error) {
	workspaces, err := u.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(workspaces, func(i int, j int) bool {
		return strings.ToLower(workspaces[i].Name) < strings.ToLower(workspaces[j].Name)
	})
	return workspaces, nil
}
