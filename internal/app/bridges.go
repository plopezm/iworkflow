//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package app

import (
	"context"
	"errors"
	"fmt"

	agentsfileserrors "iworkflow/internal/modules/agents_files/domain/errors"
	agentsfilesmodels "iworkflow/internal/modules/agents_files/domain/models"
	agentsfilesoutbound "iworkflow/internal/modules/agents_files/domain/ports/outbound"
	agentsfilesdi "iworkflow/internal/modules/agents_files/infrastructure/di"
	workspacesapp "iworkflow/internal/modules/workspaces/application"
	workspaceserrors "iworkflow/internal/modules/workspaces/domain/errors"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
	workspacesoutbound "iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// projectLookup presents the workspaces as the projects an AGENTS.md can be installed in. The
// workspaces module needs the agents_files module and the other way round, so the lookup is built
// before both and bound to the workspaces use cases once they exist (see New).
type projectLookup struct {
	list *workspacesapp.ListWorkspaces
	get  *workspacesapp.GetWorkspace
}

var _ agentsfilesoutbound.ProjectLookup = (*projectLookup)(nil)

// List returns every workspace as a project.
func (p *projectLookup) List(ctx context.Context) ([]agentsfilesmodels.ProjectRef, error) {
	workspaces, err := p.list.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workspaces as projects: %w", err)
	}
	projects := make([]agentsfilesmodels.ProjectRef, 0, len(workspaces))
	for _, workspace := range workspaces {
		projects = append(projects, projectOf(workspace))
	}
	return projects, nil
}

// Find returns the workspace with the given identifier as a project. An unknown identifier yields
// nil and no error, as the port requires.
func (p *projectLookup) Find(ctx context.Context, id string) (*agentsfilesmodels.ProjectRef, error) {
	workspaceID, err := workspacesmodels.ParseWorkspaceID(id)
	if err != nil {
		return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
	}
	workspace, err := p.get.Execute(ctx, workspaceID)
	if errors.Is(err, workspaceserrors.NotFound("")) {
		return nil, nil //nolint:nilnil // The port defines "unknown identifier" as nil and no error.
	}
	if err != nil {
		return nil, fmt.Errorf("find workspace as project: %w", err)
	}
	project := projectOf(workspace)
	return &project, nil
}

// projectOf maps a workspace to the project view the agents_files module uses.
func projectOf(workspace workspacesmodels.Workspace) agentsfilesmodels.ProjectRef {
	return agentsfilesmodels.ProjectRef{
		ID:   workspace.ID.String(),
		Name: workspace.Name,
		Path: workspace.Path,
	}
}

// agentsFilesBridge gives the workspaces module the stored AGENTS.md files, as the outbound port
// defines them, through the agents_files module.
type agentsFilesBridge struct {
	module *agentsfilesdi.Module
}

var _ workspacesoutbound.AgentsFiles = (*agentsFilesBridge)(nil)

// ListOptions returns the stored AGENTS.md files a workspace can install.
func (b *agentsFilesBridge) ListOptions(ctx context.Context) ([]workspacesmodels.AgentsFileOption, error) {
	files, err := b.module.List.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("list agents files: %w", err)
	}
	options := make([]workspacesmodels.AgentsFileOption, 0, len(files))
	for _, file := range files {
		options = append(options, workspacesmodels.AgentsFileOption{
			ID:          file.ID.String(),
			Description: file.Description,
		})
	}
	return options, nil
}

// Install writes the stored AGENTS.md into root. Errors are mapped to the workspaces error kinds.
func (b *agentsFilesBridge) Install(ctx context.Context, agentsFileID string, root string, overwrite bool) error {
	id, err := agentsfilesmodels.ParseAgentsFileID(agentsFileID)
	if err != nil {
		return workspaceserrors.AgentsFileNotFound(agentsFileID)
	}
	if _, err := b.module.Install.Execute(ctx, id, root, overwrite); err != nil {
		return mapAgentsFileError(err, agentsFileID, root)
	}
	return nil
}

// mapAgentsFileError translates an agents_files failure into the workspaces error the caller expects.
func mapAgentsFileError(err error, agentsFileID string, root string) error {
	switch {
	case errors.Is(err, agentsfileserrors.NotFound("")):
		return workspaceserrors.AgentsFileNotFound(agentsFileID)
	case errors.Is(err, agentsfileserrors.TargetExists("")):
		return workspaceserrors.AgentsFileTargetExists(root)
	default:
		return workspaceserrors.AgentsFile(err.Error())
	}
}
