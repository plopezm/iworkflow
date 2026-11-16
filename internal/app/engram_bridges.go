package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	engrammodels "iworkflow/internal/modules/engram/domain/models"
	engramdi "iworkflow/internal/modules/engram/infrastructure/di"
	memoriesoutbound "iworkflow/internal/modules/memories/domain/ports/outbound"
	tasksmodels "iworkflow/internal/modules/tasks/domain/models"
	tasksoutbound "iworkflow/internal/modules/tasks/domain/ports/outbound"
	workspacesmodels "iworkflow/internal/modules/workspaces/domain/models"
	workspacesoutbound "iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// composeEngram builds the Engram module and its ports. Engram is always wired: its address is the saved setting, else
// IWF_ENGRAM_URL, else Engram's default, and it can change at runtime from Configuración. An invalid IWF_ENGRAM_URL is
// ignored with one warning that never echoes the value. Nothing here fails the start or touches the network: the
// client has no address until App.Start applies the settings.
func composeEngram(db *sql.DB, raw string, logger *slog.Logger) (*engramdi.Module, workspacesoutbound.MemorySummaryPort, tasksoutbound.TaskMemory, memoriesoutbound.MemoryProvider) {
	envURL := ""
	if _, err := engramdi.ResolveBaseURL(raw); err == nil {
		envURL = raw
	} else if !errors.Is(err, engramdi.ErrNotConfigured) {
		logger.Warn("IWF_ENGRAM_URL ignored: it is not a valid http or https URL", "error", err)
	}
	module := engramdi.NewModule(engramdi.Deps{DB: db, EnvURL: envURL, Logger: logger})
	return module, newEngramMemoryBridge(module.ListWorkspacesMemory), newEngramTaskMemoryBridge(module.RecentMemories),
		newEngramMemoryProvider(module.ListMemories, module.ReadMemory)
}

// workspacesMemoryReader is the part of the engram ListWorkspacesMemory use case the memory bridge needs.
type workspacesMemoryReader interface {
	Execute(ctx context.Context, workspaces []engrammodels.Workspace) ([]engrammodels.WorkspaceMemory, error)
}

// engramMemoryBridge gives the workspaces module the memory summaries of its rows, through the engram module. It
// lives in the composition root, so the workspaces module never imports engram.
type engramMemoryBridge struct {
	summaries workspacesMemoryReader
}

var _ workspacesoutbound.MemorySummaryPort = (*engramMemoryBridge)(nil)

func newEngramMemoryBridge(summaries workspacesMemoryReader) *engramMemoryBridge {
	return &engramMemoryBridge{summaries: summaries}
}

// Summaries asks Engram once about all the workspaces. An Engram that cannot answer is the unavailable state, not an
// error; only an unexpected failure, or an answer that does not line up with the workspaces, is returned, wrapped.
func (b *engramMemoryBridge) Summaries(ctx context.Context, workspaces []workspacesmodels.Workspace) ([]workspacesmodels.MemorySummary, error) {
	engramWorkspaces := make([]engrammodels.Workspace, 0, len(workspaces))
	for _, workspace := range workspaces {
		engramWorkspaces = append(engramWorkspaces, engrammodels.NewWorkspace(workspace.Name, workspace.Path))
	}
	memories, err := b.summaries.Execute(ctx, engramWorkspaces)
	if err != nil {
		return nil, fmt.Errorf("read engram memories of %d workspaces: %w", len(workspaces), err)
	}
	if len(memories) != len(workspaces) {
		return nil, fmt.Errorf("engram answered %d memories for %d workspaces", len(memories), len(workspaces))
	}
	summaries := make([]workspacesmodels.MemorySummary, 0, len(memories))
	for _, memory := range memories {
		summaries = append(summaries, workspaceMemorySummary(memory))
	}
	return summaries, nil
}

// workspaceMemorySummary maps the Engram state of a workspace to the one the workspaces screens show. A matching
// project without memories and a workspace with no matching project both read as empty; an unknown state is treated
// as unavailable, so it never claims memories it cannot show.
func workspaceMemorySummary(memory engrammodels.WorkspaceMemory) workspacesmodels.MemorySummary {
	switch memory.State {
	case engrammodels.StateAvailable:
		if memory.Count == 0 {
			return workspacesmodels.MemorySummary{State: workspacesmodels.MemoryEmpty}
		}
		return workspacesmodels.MemorySummary{State: workspacesmodels.MemoryAvailable, Count: memory.Count}
	case engrammodels.StateNoMatch:
		return workspacesmodels.MemorySummary{State: workspacesmodels.MemoryEmpty}
	default:
		return workspacesmodels.MemorySummary{State: workspacesmodels.MemoryUnavailable}
	}
}

// recentMemoriesReader is the part of the engram RecentMemories use case the task memory bridge needs.
type recentMemoriesReader interface {
	Execute(ctx context.Context, workspace engrammodels.Workspace) (engrammodels.MemoryFeed, error)
}

// engramTaskMemoryBridge gives the tasks module the Memory panel of a task's workspace, through the engram module. It
// lives in the composition root, so the tasks module never imports engram.
type engramTaskMemoryBridge struct {
	recent recentMemoriesReader
}

var _ tasksoutbound.TaskMemory = (*engramTaskMemoryBridge)(nil)

func newEngramTaskMemoryBridge(recent recentMemoriesReader) *engramTaskMemoryBridge {
	return &engramTaskMemoryBridge{recent: recent}
}

// Recent asks Engram for the recent memories of the workspace. Only a matching project that answers is available; an
// Engram that cannot answer, or a workspace without a project, is an empty feed that is not available. Only an unexpected
// failure is returned, wrapped. The feed carries the type, the title and the date of each memory, never its content.
func (b *engramTaskMemoryBridge) Recent(ctx context.Context, workspaceName string, workspacePath string) (tasksmodels.TaskMemoryFeed, error) {
	feed, err := b.recent.Execute(ctx, engrammodels.NewWorkspace(workspaceName, workspacePath))
	if err != nil {
		return tasksmodels.TaskMemoryFeed{}, fmt.Errorf("read engram recent memories of %s: %w", workspaceName, err)
	}
	if feed.State != engrammodels.StateAvailable {
		return tasksmodels.TaskMemoryFeed{}, nil
	}
	var memories []tasksmodels.TaskMemory
	for _, memory := range feed.Memories {
		memories = append(memories, tasksmodels.TaskMemory{Type: memory.Type, Title: memory.Title, LastSeenAt: memory.LastSeenAt})
	}
	return tasksmodels.TaskMemoryFeed{Available: true, Memories: memories}, nil
}
