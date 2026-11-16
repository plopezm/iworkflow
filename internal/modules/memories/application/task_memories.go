package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/memories/domain/models"
	"iworkflow/internal/modules/memories/domain/ports/outbound"
)

// ListTaskMemories lists the memories of a task's workspace, split into the ones written since the task started (its
// branch) and the earlier ones. A provider that is not configured (nil) reads as unavailable.
type ListTaskMemories struct {
	tasks      outbound.TaskLookup
	workspaces outbound.WorkspaceLookup
	provider   outbound.MemoryProvider
}

// NewListTaskMemories builds the use case. provider may be nil.
func NewListTaskMemories(tasks outbound.TaskLookup, workspaces outbound.WorkspaceLookup, provider outbound.MemoryProvider) *ListTaskMemories {
	return &ListTaskMemories{tasks: tasks, workspaces: workspaces, provider: provider}
}

// Execute returns the task, its workspace and the split feed.
func (u *ListTaskMemories) Execute(ctx context.Context, taskID string) (models.TaskRef, models.Workspace, models.TaskFeed, error) {
	task, err := u.tasks.Get(ctx, taskID)
	if err != nil {
		return models.TaskRef{}, models.Workspace{}, models.TaskFeed{}, fmt.Errorf("find task %s: %w", taskID, err)
	}
	workspace, err := u.workspaces.Get(ctx, task.WorkspaceID)
	if err != nil {
		return models.TaskRef{}, models.Workspace{}, models.TaskFeed{}, fmt.Errorf("find workspace of task %s: %w", taskID, err)
	}
	if u.provider == nil {
		return task, workspace, models.TaskFeed{State: models.StateUnavailable}, nil
	}
	feed, err := u.provider.List(ctx, workspace)
	if err != nil {
		return models.TaskRef{}, models.Workspace{}, models.TaskFeed{}, fmt.Errorf("list memories of %s: %w", workspace.Name, err)
	}
	return task, workspace, models.SplitSince(feed, task), nil
}
