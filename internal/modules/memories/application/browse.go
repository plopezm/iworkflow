// Package application holds the use cases of the memories module.
package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/memories/domain/models"
	"iworkflow/internal/modules/memories/domain/ports/outbound"
)

// ListWorkspaceMemories lists the memories of one workspace. A provider that is not configured (nil) reads as unavailable.
type ListWorkspaceMemories struct {
	workspaces outbound.WorkspaceLookup
	provider   outbound.MemoryProvider
}

// NewListWorkspaceMemories builds the use case. provider may be nil.
func NewListWorkspaceMemories(workspaces outbound.WorkspaceLookup, provider outbound.MemoryProvider) *ListWorkspaceMemories {
	return &ListWorkspaceMemories{workspaces: workspaces, provider: provider}
}

// Execute returns the workspace and its memory feed.
func (u *ListWorkspaceMemories) Execute(ctx context.Context, workspaceID string) (models.Workspace, models.Feed, error) {
	workspace, err := u.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return models.Workspace{}, models.Feed{}, fmt.Errorf("find workspace %s: %w", workspaceID, err)
	}
	if u.provider == nil {
		return workspace, models.Feed{State: models.StateUnavailable}, nil
	}
	feed, err := u.provider.List(ctx, workspace)
	if err != nil {
		return models.Workspace{}, models.Feed{}, fmt.Errorf("list memories of %s: %w", workspace.Name, err)
	}
	return workspace, feed, nil
}

// ReadWorkspaceMemory reads one memory of one workspace.
type ReadWorkspaceMemory struct {
	workspaces outbound.WorkspaceLookup
	provider   outbound.MemoryProvider
}

// NewReadWorkspaceMemory builds the use case. provider may be nil.
func NewReadWorkspaceMemory(workspaces outbound.WorkspaceLookup, provider outbound.MemoryProvider) *ReadWorkspaceMemory {
	return &ReadWorkspaceMemory{workspaces: workspaces, provider: provider}
}

// Execute returns the workspace and the memory with its content.
func (u *ReadWorkspaceMemory) Execute(ctx context.Context, workspaceID string, memoryID int) (models.Workspace, models.Entry, error) {
	workspace, err := u.workspaces.Get(ctx, workspaceID)
	if err != nil {
		return models.Workspace{}, models.Entry{}, fmt.Errorf("find workspace %s: %w", workspaceID, err)
	}
	if u.provider == nil {
		return workspace, models.Entry{State: models.StateUnavailable}, nil
	}
	entry, err := u.provider.Read(ctx, workspace, memoryID)
	if err != nil {
		return models.Workspace{}, models.Entry{}, fmt.Errorf("read memory %d of %s: %w", memoryID, workspace.Name, err)
	}
	return workspace, entry, nil
}
