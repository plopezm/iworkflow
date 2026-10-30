//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"slices"
	"strings"

	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

// ListAgentsFiles lists the AGENTS.md references sorted by description, case-insensitive.
type ListAgentsFiles struct {
	repository outbound.AgentsFileRepository
}

// NewListAgentsFiles builds the use case.
func NewListAgentsFiles(repository outbound.AgentsFileRepository) *ListAgentsFiles {
	return &ListAgentsFiles{repository: repository}
}

// Execute returns the stored references in description order.
func (u *ListAgentsFiles) Execute(ctx context.Context) ([]models.AgentsFile, error) {
	files, err := u.repository.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(files, func(a models.AgentsFile, b models.AgentsFile) int {
		return strings.Compare(strings.ToLower(a.Description), strings.ToLower(b.Description))
	})
	return files, nil
}
