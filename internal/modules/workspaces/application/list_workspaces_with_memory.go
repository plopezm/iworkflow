package application

import (
	"context"
	"log/slog"

	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// ListWorkspacesWithMemory lists the registered workspaces with what the memory provider remembers about each
// one. The provider is read once per page, for all the rows together. A memory failure never fails the list: the
// rows read as unavailable instead.
type ListWorkspacesWithMemory struct {
	list   *ListWorkspaces
	memory outbound.MemorySummaryPort
	logger *slog.Logger
}

// NewListWorkspacesWithMemory builds the use case over the workspace listing, the memory port and the project logger.
// A nil port means no provider is wired: every row is MemoryDisabled and no memory is read. A nil logger falls back
// to the default one.
func NewListWorkspacesWithMemory(list *ListWorkspaces, memory outbound.MemorySummaryPort, logger *slog.Logger) *ListWorkspacesWithMemory {
	if logger == nil {
		logger = slog.Default()
	}
	return &ListWorkspacesWithMemory{list: list, memory: memory, logger: logger}
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
// line up with the workspaces, is the unavailable state for every row: the failure is logged once and the page still
// renders. The log line never carries a URL: the error text is redacted here (addresses and credentials), whatever the port put in it.
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
	switch {
	case err != nil:
		u.logger.WarnContext(ctx, "could not read the memory of the workspaces; every row is unavailable",
			"workspaces", len(workspaces), "reason", "read failed", "error", redactAddresses(err.Error()))
	case len(read) != len(workspaces):
		u.logger.WarnContext(ctx, "could not read the memory of the workspaces; every row is unavailable",
			"workspaces", len(workspaces), "reason", "answer does not match the workspaces", "answers", len(read))
	default:
		return read
	}
	for index := range summaries {
		summaries[index] = models.MemorySummary{State: models.MemoryUnavailable}
	}
	return summaries
}
