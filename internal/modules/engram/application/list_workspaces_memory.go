package application

import (
	"context"

	"iworkflow/internal/modules/engram/domain/models"
	"iworkflow/internal/modules/engram/domain/ports/outbound"
)

// ListWorkspacesMemory summarises what Engram remembers about many workspaces in one read: the projects are
// asked for once, and every workspace is matched against them. A page of N rows costs one GET /projects.
type ListWorkspacesMemory struct {
	store outbound.MemoryStore
}

// NewListWorkspacesMemory builds the use case.
func NewListWorkspacesMemory(store outbound.MemoryStore) *ListWorkspacesMemory {
	return &ListWorkspacesMemory{store: store}
}

// Execute returns one summary per workspace, in the order of the input. Workspaces that match the same project
// share its count. An unreachable Engram marks every workspace unavailable, with a nil error. Other failures are
// returned wrapped. No workspace means no request.
func (u *ListWorkspacesMemory) Execute(ctx context.Context, workspaces []models.Workspace) ([]models.WorkspaceMemory, error) {
	summaries := make([]models.WorkspaceMemory, 0, len(workspaces))
	if len(workspaces) == 0 {
		return summaries, nil
	}
	projects, state, err := listProjects(ctx, u.store)
	if err != nil {
		return nil, err
	}
	for _, workspace := range workspaces {
		summaries = append(summaries, summarize(projects, state, workspace))
	}
	return summaries, nil
}

// summarize maps one workspace to its summary, given the projects read once for the page.
func summarize(projects []models.MemoryProject, state models.MemoryState, workspace models.Workspace) models.WorkspaceMemory {
	if state != models.StateAvailable {
		return models.WorkspaceMemory{State: state}
	}
	project, ok := models.MatchProject(projects, workspace)
	if !ok {
		return models.WorkspaceMemory{State: models.StateNoMatch}
	}
	return models.WorkspaceMemory{State: models.StateAvailable, Count: project.ObservationCount}
}
