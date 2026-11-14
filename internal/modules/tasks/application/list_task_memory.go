package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// MaxTaskMemories is how many memories the memory panel of a task shows.
const MaxTaskMemories = 5

// TaskMemoryInput names the task whose workspace the memory panel is read for.
type TaskMemoryInput struct {
	WorkspaceID string
}

// ListTaskMemory reads the memory panel of a task: the most recent memories of the Engram project of its workspace.
type ListTaskMemory struct {
	memory     outbound.TaskMemory
	workspaces outbound.WorkspaceLookup
}

// NewListTaskMemory builds the use case over the memory port and the workspace lookup.
func NewListTaskMemory(memory outbound.TaskMemory, workspaces outbound.WorkspaceLookup) *ListTaskMemory {
	return &ListTaskMemory{memory: memory, workspaces: workspaces}
}

// Execute returns the memory feed of the workspace, at most MaxTaskMemories memories, newest first. A workspace that
// does not exist yields an empty feed and no error. A failed lookup or port read is returned wrapped; the caller
// decides how the page degrades.
func (u *ListTaskMemory) Execute(ctx context.Context, input TaskMemoryInput) (models.TaskMemoryFeed, error) {
	workspace, err := u.workspaces.Find(ctx, input.WorkspaceID)
	if err != nil {
		return models.TaskMemoryFeed{}, fmt.Errorf("find workspace of the task: %w", err)
	}
	if workspace == nil {
		return models.TaskMemoryFeed{}, nil
	}
	feed, err := u.memory.Recent(ctx, workspace.Name, workspace.Path)
	if err != nil {
		return models.TaskMemoryFeed{}, fmt.Errorf("read recent memories of the task: %w", err)
	}
	if len(feed.Memories) > MaxTaskMemories {
		feed.Memories = feed.Memories[:MaxTaskMemories]
	}
	return feed, nil
}
