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
	// RootCommits lists the root commits (the commits without parent) reachable from HEAD of the repository that
	// contains the folder. It is empty when HEAD is unborn. It fails with PathNotFound or NotAGitRepository.
	RootCommits(ctx context.Context, root string) ([]string, error)
	// HasLinkedWorktrees reports whether the repository has worktrees besides its main one.
	HasLinkedWorktrees(ctx context.Context, root string) (bool, error)
	// RepairWorktrees runs `git worktree repair` in the repository, so git finds its worktrees where they are now.
	RepairWorktrees(ctx context.Context, root string) error
}
