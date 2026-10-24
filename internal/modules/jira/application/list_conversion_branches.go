//nolint:wrapcheck // Port errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"strings"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// ListConversionBranches lists the branches of the chosen workspace a converted task can start from.
type ListConversionBranches struct {
	workspaces outbound.WorkspaceCatalog
}

// NewListConversionBranches builds the use case.
func NewListConversionBranches(workspaces outbound.WorkspaceCatalog) *ListConversionBranches {
	return &ListConversionBranches{workspaces: workspaces}
}

// Execute returns the branches; a blank workspace has none and the catalog is not asked.
func (u *ListConversionBranches) Execute(ctx context.Context, workspaceID string) ([]models.BranchOption, error) {
	trimmed := strings.TrimSpace(workspaceID)
	if trimmed == "" {
		return []models.BranchOption{}, nil
	}
	return u.workspaces.Branches(ctx, trimmed)
}
