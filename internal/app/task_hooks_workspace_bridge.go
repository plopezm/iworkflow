package app

import (
	"context"
	"fmt"

	hookapp "iworkflow/internal/modules/task_hooks/application"
	hooksoutbound "iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	workspacesapp "iworkflow/internal/modules/workspaces/application"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
)

// taskHooksWorkspaces gives the task_hooks module the working directory of a workspace, where a test of a hook runs.
// It is built over the workspaces use cases, so the hooks module never imports the workspaces module.
type taskHooksWorkspaces struct {
	get *workspacesapp.GetWorkspace
}

var _ hooksoutbound.WorkspaceLookup = (*taskHooksWorkspaces)(nil)

// Path returns the directory of the workspace. An unknown or malformed id is an error.
func (w *taskHooksWorkspaces) Path(ctx context.Context, workspaceID string) (string, error) {
	id, err := workspacesmodels.ParseWorkspaceID(workspaceID)
	if err != nil {
		return "", fmt.Errorf("parse workspace id: %w", err)
	}
	workspace, err := w.get.Execute(ctx, id)
	if err != nil {
		return "", fmt.Errorf("get workspace: %w", err)
	}
	return workspace.Path, nil
}

// taskHooksProjects gives the task_hooks module the projects a hook can be assigned to. It is built over the workspaces
// list, so the hooks module never imports the workspaces module.
type taskHooksProjects struct {
	list *workspacesapp.ListWorkspaces
}

var _ hookapp.WorkspaceLister = (*taskHooksProjects)(nil)

// Projects returns every workspace as a project, in the order the workspaces module lists them.
func (p *taskHooksProjects) Projects(ctx context.Context) ([]hookapp.ProjectOption, error) {
	workspaces, err := p.list.Execute(ctx)
	if err != nil {
		return nil, fmt.Errorf("list workspaces: %w", err)
	}
	projects := make([]hookapp.ProjectOption, 0, len(workspaces))
	for _, workspace := range workspaces {
		projects = append(projects, hookapp.ProjectOption{ID: workspace.ID.String(), Name: workspace.Name})
	}
	return projects, nil
}
