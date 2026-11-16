// Package outbound holds the ports the memories application layer depends on.
package outbound

import (
	"context"

	"iworkflow/internal/modules/memories/domain/models"
)

// MemoryProvider is the facade over a memory system (Engram today). It is read-only by design: there is no method
// to write, change or delete a memory. Implementations return an unavailable state, not an error, when the system
// cannot answer, and an error wrapping domain/errors.ErrMemoryNotFound for a memory that is not the workspace's.
type MemoryProvider interface {
	// List returns the memories of the workspace, newest first, without their content.
	List(ctx context.Context, workspace models.Workspace) (models.Feed, error)
	// Read returns one memory of the workspace with its content.
	Read(ctx context.Context, workspace models.Workspace, id int) (models.Entry, error)
}

// WorkspaceLookup finds a registered workspace by its id. It returns an error wrapping
// domain/errors.ErrWorkspaceNotFound when there is none.
type WorkspaceLookup interface {
	Get(ctx context.Context, id string) (models.Workspace, error)
}

// TaskLookup finds a task by its id. It returns an error wrapping domain/errors.ErrTaskNotFound when there is none.
type TaskLookup interface {
	Get(ctx context.Context, id string) (models.TaskRef, error)
}
