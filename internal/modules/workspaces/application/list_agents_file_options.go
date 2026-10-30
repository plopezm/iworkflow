//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// ListAgentsFileOptions lists the stored AGENTS.md files that can be installed in a new project.
type ListAgentsFileOptions struct {
	agentsFiles outbound.AgentsFiles
}

// NewListAgentsFileOptions builds the use case.
func NewListAgentsFileOptions(agentsFiles outbound.AgentsFiles) *ListAgentsFileOptions {
	return &ListAgentsFileOptions{agentsFiles: agentsFiles}
}

// Execute returns the options offered by the agents_files module.
func (u *ListAgentsFileOptions) Execute(ctx context.Context) ([]models.AgentsFileOption, error) {
	return u.agentsFiles.ListOptions(ctx)
}
