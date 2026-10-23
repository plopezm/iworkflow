package outbound

import (
	"context"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// GitStatusPort reads and changes the git state of a project.
type GitStatusPort interface {
	// Changes returns the paths under root that differ from HEAD. It is empty when root is not a
	// git repository.
	Changes(ctx context.Context, root string) ([]models.GitChange, error)

	// BaseContent returns the content of relative at HEAD, or nil when git does not know the file
	// (new file, or not a repository).
	BaseContent(ctx context.Context, root string, relative string) (*string, error)

	// Stage adds paths (relative to root; empty means everything) to the index.
	Stage(ctx context.Context, root string, paths []string) error

	// Unstage removes paths (relative to root; empty means everything) from the index, keeping the
	// working tree untouched.
	Unstage(ctx context.Context, root string, paths []string) error

	// Commit records what is staged in the index with message; it fails when nothing is staged.
	Commit(ctx context.Context, root string, message string) error
}
