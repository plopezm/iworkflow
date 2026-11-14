package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskMemory reads the most recent memories of the Engram project that matches a workspace. It is consumer-side:
// the composition root implements it with the engram module, so tasks never imports engram. It is optional: when it is
// not set, the detail page shows no memory panel. The port is read-only (no save, update or delete).
type TaskMemory interface {
	// Recent returns the feed of the workspace, newest first. An Engram that cannot answer is Available false, not an
	// error; an error is an unexpected failure.
	Recent(ctx context.Context, workspaceName string, workspacePath string) (models.TaskMemoryFeed, error)
}
