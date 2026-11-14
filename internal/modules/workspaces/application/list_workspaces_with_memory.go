package application

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// ListWorkspacesWithMemory lists the registered workspaces with what the memory provider remembers about each
// one. The provider is read once per page, for all the rows together. A memory failure never fails the list: the
// rows read as unavailable instead.
type ListWorkspacesWithMemory struct {
	list   *ListWorkspaces
	memory outbound.MemorySummaryPort
}

// NewListWorkspacesWithMemory builds the use case over the workspace listing and the memory port. A nil port means
// no provider is wired: every row is MemoryDisabled and no memory is read.
func NewListWorkspacesWithMemory(list *ListWorkspaces, memory outbound.MemorySummaryPort) *ListWorkspacesWithMemory {
	return &ListWorkspacesWithMemory{list: list, memory: memory}
}

// Execute returns the workspaces in the order ListWorkspaces gives them. Repository errors are returned as
// they are; memory errors are not.
func (u *ListWorkspacesWithMemory) Execute(ctx context.Context) ([]models.WorkspaceWithMemory, error) {
	workspaces, err := u.list.Execute(ctx)
	if err != nil {
		return nil, err
	}
	summaries := u.summaries(ctx, workspaces)
	entries := make([]models.WorkspaceWithMemory, 0, len(workspaces))
	for index, workspace := range workspaces {
		entries = append(entries, models.WorkspaceWithMemory{Workspace: workspace, Memory: summaries[index]})
	}
	return entries, nil
}

// summaries reads the memory of every workspace with one port call. A failing port, or an answer that does not
// line up with the workspaces, is the unavailable state for every row: the error is deliberately folded into the
// state, because the page must still render.
func (u *ListWorkspacesWithMemory) summaries(ctx context.Context, workspaces []models.Workspace) []models.MemorySummary {
	summaries := make([]models.MemorySummary, len(workspaces))
	if u.memory == nil {
		for index := range summaries {
			summaries[index] = models.MemorySummary{State: models.MemoryDisabled}
		}
		return summaries
	}
	if len(workspaces) == 0 {
		return summaries
	}
	read, err := u.memory.Summaries(ctx, workspaces)
	if err != nil || len(read) != len(workspaces) {
		for index := range summaries {
			summaries[index] = models.MemorySummary{State: models.MemoryUnavailable}
		}
		return summaries
	}
	return read
}
