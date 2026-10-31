package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// Worktree runs the git operations on the worktrees of the tasks. Declared for slice E.
type Worktree interface {
	// WorktreePath returns the folder where the worktree of a task of that workspace is created.
	WorktreePath(workspaceName string, dirName string) string
	// ListBranches returns the local and remote branches, without the task branches and origin/HEAD:
	// local first, each group sorted by name.
	ListBranches(ctx context.Context, repo string) ([]models.BranchOption, error)
	// Create creates branch from baseBranch (local or remote) and checks it out in path.
	Create(ctx context.Context, repo string, baseBranch string, branch string, path string) error
	// CommitAll commits every pending change. It returns false when there was nothing to commit.
	CommitAll(ctx context.Context, path string, message string) (bool, error)
	// Diff returns the diff of branch against baseBranch.
	Diff(ctx context.Context, repo string, baseBranch string, branch string) (models.TaskDiff, error)
	// DiffLive returns the diff of the worktree against the divergence point with baseBranch: commits of
	// the branch plus uncommitted changes, including untracked new files.
	DiffLive(ctx context.Context, worktree string, baseBranch string) (models.TaskDiff, error)
	// DiffCommitted returns the diff of the commits of the worktree branch against the divergence point with
	// baseBranch: uncommitted and untracked changes are left out.
	DiffCommitted(ctx context.Context, worktree string, baseBranch string) (models.TaskDiff, error)
	// DiffFingerprint is a cheap fingerprint of Diff: it changes if and only if the diff changes.
	DiffFingerprint(ctx context.Context, repo string, baseBranch string, branch string) (string, error)
	// PinRange fixes the commit range of branch against baseBranch and creates pinRef at the tip so the
	// range survives the deletion of the branch. It must run before the merge.
	PinRange(ctx context.Context, repo string, baseBranch string, branch string, pinRef string) (models.DiffRange, error)
	// DiffRange returns the diff of a fixed range, even when the branch no longer exists.
	DiffRange(ctx context.Context, repo string, diffRange models.DiffRange) (models.TaskDiff, error)
	// Merge integrates branch into baseBranch with --no-ff. On conflict everything is left as it was.
	// A remote base (origin/develop) integrates into the local namesake (develop), created following the
	// remote when missing. It never pushes.
	Merge(ctx context.Context, repo string, baseBranch string, branch string, message string) (models.MergeOutcome, error)
	// Exists reports whether the worktree folder exists on disk.
	Exists(ctx context.Context, path string) bool
	// Remove deletes the worktree (when it exists) and the branch.
	Remove(ctx context.Context, repo string, path string, branch string) error
	// SweepOrphans deletes the leftovers of the normal cleanup: the trash and the task folders that are
	// no longer worktrees and are not in keep. Live worktrees are respected. It returns how many were deleted.
	SweepOrphans(ctx context.Context, keep []string) (int, error)
	// WaitForCleanup waits for the background deletions already started.
	WaitForCleanup(ctx context.Context)
}
