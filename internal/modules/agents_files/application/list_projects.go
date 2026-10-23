//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

// ListProjects lists the projects an AGENTS.md can be added to.
type ListProjects struct {
	projects outbound.ProjectLookup
}

// NewListProjects builds the use case.
func NewListProjects(projects outbound.ProjectLookup) *ListProjects {
	return &ListProjects{projects: projects}
}

// Execute returns the projects reported by the lookup port.
func (u *ListProjects) Execute(ctx context.Context) ([]models.ProjectRef, error) {
	return u.projects.List(ctx)
}
