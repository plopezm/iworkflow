//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/domain/models"
	"iworkflow/internal/modules/agents_files/domain/ports/outbound"
)

// ApplyAgentsFileToProject adds a stored AGENTS.md to an existing project.
type ApplyAgentsFileToProject struct {
	projects outbound.ProjectLookup
	install  *InstallAgentsFile
}

// NewApplyAgentsFileToProject builds the use case.
func NewApplyAgentsFileToProject(projects outbound.ProjectLookup, install *InstallAgentsFile) *ApplyAgentsFileToProject {
	return &ApplyAgentsFileToProject{projects: projects, install: install}
}

// Execute installs the file into the project's root folder.
func (u *ApplyAgentsFileToProject) Execute(ctx context.Context, id models.AgentsFileID, projectID string, overwrite bool) (models.InstallOutcome, error) {
	project, err := u.projects.Find(ctx, projectID)
	if err != nil {
		return models.InstallOutcome{}, err
	}
	if project == nil {
		return models.InstallOutcome{}, errors.ProjectNotFound(projectID)
	}
	return u.install.Execute(ctx, id, project.Path, overwrite)
}
