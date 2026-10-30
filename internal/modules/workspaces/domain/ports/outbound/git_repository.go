package outbound

import (
	"context"

	"iworkflow/internal/modules/workspaces/domain/models"
)

// GitRepository inspects and initializes git repositories.
type GitRepository interface {
	// Inspect fails with PathNotFound or NotAGitRepository when appropriate.
	Inspect(ctx context.Context, path string) (models.GitRepositoryInfo, error)
	BranchExists(ctx context.Context, root string, branch string) (bool, error)
	// InitRepository creates the folder when it is missing and runs `git init` on the given branch.
	InitRepository(ctx context.Context, path string, initialBranch string) error
	// CreateInitialCommit creates an empty commit without touching the index, so HEAD is no longer unborn.
	CreateInitialCommit(ctx context.Context, root string) error
}
