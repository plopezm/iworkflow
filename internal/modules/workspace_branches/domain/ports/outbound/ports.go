// Package outbound declares the ports the workspace_branches use cases need from the outside world.
package outbound

import (
	"context"

	"iworkflow/internal/modules/workspace_branches/domain/models"
)

// BranchGit reads and deletes the branches of a repository. The adapter validates every name it receives.
type BranchGit interface {
	// ListBranches returns the local and remote-tracking branches of the repository at repoPath, with the
	// current and checked-out flags set. Protection is decided by the use cases, not here.
	ListBranches(ctx context.Context, repoPath string) ([]models.Branch, error)
	// ListRemotes returns the names of the remotes configured in the repository at repoPath.
	ListRemotes(ctx context.Context, repoPath string) ([]string, error)
	// DeleteLocal force-deletes a local branch (git branch -D).
	DeleteLocal(ctx context.Context, repoPath string, name string) error
	// DeleteRemote deletes a branch on a remote (git push <remote> --delete <name>).
	DeleteRemote(ctx context.Context, repoPath string, remote string, name string) error
}

// WorkspaceTarget is the workspace data the branch screens need.
type WorkspaceTarget struct {
	ID         string
	Name       string
	Path       string
	BaseBranch string
}

// WorkspaceResolver gives the path and the base branch of a workspace. An unknown workspace yields an error
// wrapping ErrWorkspaceNotFound.
type WorkspaceResolver interface {
	Resolve(ctx context.Context, workspaceID string) (WorkspaceTarget, error)
}

// ProtectedBranches lists the branch names of the live (unfinished) tasks of a workspace.
type ProtectedBranches interface {
	InUse(ctx context.Context, workspaceID string) ([]string, error)
}
