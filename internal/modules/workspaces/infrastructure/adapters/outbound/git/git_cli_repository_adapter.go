// Package git inspects and initializes repositories through the git CLI.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package git

import (
	"context"
	"errors"
	"os"
	"strings"

	"iworkflow/internal/core/gitx"
	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/domain/models"
	"iworkflow/internal/modules/workspaces/domain/ports/outbound"
)

// initialCommitIdentity is used only when the repository has no git identity configured.
var initialCommitIdentity = []string{"-c", "user.name=iworkflow", "-c", "user.email=iworkflow@localhost"}

// RepositoryAdapter implements outbound.GitRepository with the git CLI.
type RepositoryAdapter struct{}

var _ outbound.GitRepository = (*RepositoryAdapter)(nil)

// NewRepositoryAdapter builds the adapter.
func NewRepositoryAdapter() *RepositoryAdapter {
	return &RepositoryAdapter{}
}

// Inspect resolves the repository root, whether HEAD has commits and the current branch. A missing
// folder is PathNotFound; a folder outside any repository is NotAGitRepository.
func (a *RepositoryAdapter) Inspect(ctx context.Context, path string) (models.GitRepositoryInfo, error) {
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return models.GitRepositoryInfo{}, werrors.PathNotFound(path)
	}
	root, err := gitx.Run(ctx, path, "rev-parse", "--show-toplevel")
	if err != nil {
		var failed *gitx.FailedError
		if errors.As(err, &failed) {
			return models.GitRepositoryInfo{}, werrors.NotAGitRepository(path)
		}
		return models.GitRepositoryInfo{}, werrors.Git(err.Error())
	}

	hasCommits, err := runSucceeds(ctx, root, "rev-parse", "--verify", "--quiet", "HEAD")
	if err != nil {
		return models.GitRepositoryInfo{}, err
	}
	branch, err := gitx.RunRaw(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return models.GitRepositoryInfo{}, werrors.Git(err.Error())
	}
	currentBranch := ""
	if branch.Success {
		currentBranch = strings.TrimSpace(branch.Stdout)
	}
	return models.GitRepositoryInfo{Root: root, CurrentBranch: currentBranch, HasCommits: hasCommits}, nil
}

// BranchExists reports whether a local branch exists.
func (a *RepositoryAdapter) BranchExists(ctx context.Context, root string, branch string) (bool, error) {
	return runSucceeds(ctx, root, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
}

// InitRepository creates the folder (and missing parents) and runs `git init` on the branch.
func (a *RepositoryAdapter) InitRepository(ctx context.Context, path string, initialBranch string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return werrors.IO(err.Error())
	}
	if _, err := gitx.Run(ctx, path, "init", "-q", "-b", initialBranch); err != nil {
		return werrors.Git(err.Error())
	}
	return nil
}

// CreateInitialCommit creates an empty commit that points HEAD at it, without touching the index.
// The commit gets a fallback identity when the repository has none configured.
func (a *RepositoryAdapter) CreateInitialCommit(ctx context.Context, root string) error {
	emptyTree, err := gitx.RunWithStdin(ctx, root, nil, "hash-object", "-t", "tree", "-w", "--stdin")
	if err != nil {
		return werrors.Git(err.Error())
	}

	identityArgs, err := identityArguments(ctx, root)
	if err != nil {
		return err
	}
	commitArgs := append(identityArgs, "commit-tree", emptyTree, "-m", "Initial commit")
	commitSha, err := gitx.Run(ctx, root, commitArgs...)
	if err != nil {
		return werrors.Git(err.Error())
	}
	if _, err := gitx.Run(ctx, root, "update-ref", "HEAD", commitSha); err != nil {
		return werrors.Git(err.Error())
	}
	return nil
}

// identityArguments returns the `-c` overrides needed to commit, empty when an identity is configured.
func identityArguments(ctx context.Context, dir string) ([]string, error) {
	name, err := gitx.RunRaw(ctx, dir, "config", "user.name")
	if err != nil {
		return nil, werrors.Git(err.Error())
	}
	email, err := gitx.RunRaw(ctx, dir, "config", "user.email")
	if err != nil {
		return nil, werrors.Git(err.Error())
	}
	hasIdentity := name.Success && strings.TrimSpace(name.Stdout) != "" &&
		email.Success && strings.TrimSpace(email.Stdout) != ""
	if hasIdentity {
		return nil, nil
	}
	return append([]string(nil), initialCommitIdentity...), nil
}

// runSucceeds runs git and reports whether it exited with status 0. Only a failure to start git is an error.
func runSucceeds(ctx context.Context, dir string, args ...string) (bool, error) {
	output, err := gitx.RunRaw(ctx, dir, args...)
	if err != nil {
		return false, werrors.Git(err.Error())
	}
	return output.Success, nil
}
