package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskFile reads and overwrites single text files inside a task worktree. Declared for slice D.
type TaskFile interface {
	// Read reads rel (relative to root). It rejects paths that escape root, .git, oversized and non-UTF-8 files.
	Read(ctx context.Context, root string, rel string) (models.TaskFileContent, error)
	// Write overwrites the existing file rel. It never creates files or folders.
	Write(ctx context.Context, root string, rel string, content string) error
}
