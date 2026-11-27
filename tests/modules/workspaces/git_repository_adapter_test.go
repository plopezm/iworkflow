package workspaces_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/git"
)

func TestGitInspectResolvesRootAndBranchFromSubfolder(t *testing.T) {
	root := newGitRepo(t)
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))

	info, err := git.NewRepositoryAdapter().Inspect(context.Background(), sub)

	require.NoError(t, err)
	assert.Equal(t, evalSymlinks(t, root), info.Root)
	assert.Equal(t, "main", info.CurrentBranch)
	assert.True(t, info.HasCommits)
}

func TestGitInspectReportsSymbolicBranchWithoutCommits(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")

	info, err := git.NewRepositoryAdapter().Inspect(context.Background(), dir)

	require.NoError(t, err)
	assert.Equal(t, "main", info.CurrentBranch)
	assert.False(t, info.HasCommits)
}

func TestGitInspectReportsDetachedHeadAsEmptyBranch(t *testing.T) {
	root := newGitRepo(t)
	sha := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	runGit(t, root, "update-ref", "--no-deref", "HEAD", sha)

	info, err := git.NewRepositoryAdapter().Inspect(context.Background(), root)

	require.NoError(t, err)
	assert.Empty(t, info.CurrentBranch)
	assert.True(t, info.HasCommits)
}

func TestGitInspectRejectsPlainFolderAndMissingPath(t *testing.T) {
	dir := t.TempDir()
	adapter := git.NewRepositoryAdapter()

	_, plainErr := adapter.Inspect(context.Background(), dir)
	_, missingErr := adapter.Inspect(context.Background(), filepath.Join(dir, "nope"))

	require.ErrorIs(t, plainErr, werrors.NotAGitRepository(""))
	require.ErrorIs(t, missingErr, werrors.PathNotFound(""))
}

func TestGitBranchExistsChecksLocalBranches(t *testing.T) {
	root := newGitRepo(t)
	runGit(t, root, "branch", "develop")
	adapter := git.NewRepositoryAdapter()

	exists, err := adapter.BranchExists(context.Background(), root, "develop")
	require.NoError(t, err)
	missing, err := adapter.BranchExists(context.Background(), root, "nope")
	require.NoError(t, err)

	assert.True(t, exists)
	assert.False(t, missing)
}

func TestGitInitRepositoryCreatesNestedMissingDirOnRequestedBranch(t *testing.T) {
	target := filepath.Join(t.TempDir(), "nested", "repo")
	adapter := git.NewRepositoryAdapter()

	require.NoError(t, adapter.InitRepository(context.Background(), target, "develop"))

	info, err := adapter.Inspect(context.Background(), target)
	require.NoError(t, err)
	assert.Equal(t, "develop", info.CurrentBranch)
	assert.False(t, info.HasCommits)
}

func TestGitCreateInitialCommitLeavesStagedFileUncommitted(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("pending\n"), 0o600))
	runGit(t, dir, "add", "staged.txt")
	adapter := git.NewRepositoryAdapter()

	require.NoError(t, adapter.CreateInitialCommit(context.Background(), dir))

	info, err := adapter.Inspect(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, info.HasCommits)
	assert.Equal(t, "main", info.CurrentBranch)
	show := runGit(t, dir, "show", "--stat", "--format=", "HEAD")
	assert.Empty(t, strings.TrimSpace(show), "the initial commit must not include files")
	assert.Contains(t, runGit(t, dir, "status", "--porcelain"), "staged.txt")
}

func TestGitCreateInitialCommitWorksWithoutConfiguredIdentity(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	// Point the global and system configuration at empty files so no identity is found.
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "empty-global"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	adapter := git.NewRepositoryAdapter()

	require.NoError(t, adapter.CreateInitialCommit(context.Background(), dir))

	info, err := adapter.Inspect(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, info.HasCommits)
}

// evalSymlinks resolves temporary-folder symlinks (macOS /var -> /private/var) so paths compare equal.
func evalSymlinks(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return resolved
}

func TestGitInspectReportsGitFailuresThatAreNotNotARepositoryAsGitErrors(t *testing.T) {
	dir := t.TempDir()
	// A malformed global config makes every git command fail with a message that is not
	// "not a git repository". Such a failure must not be reported as a missing repository, because
	// the register flow would then offer to initialize a folder that may already be one.
	broken := filepath.Join(t.TempDir(), "broken-gitconfig")
	require.NoError(t, os.WriteFile(broken, []byte("[core\n"), 0o600))
	t.Setenv("GIT_CONFIG_GLOBAL", broken)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")

	_, err := git.NewRepositoryAdapter().Inspect(context.Background(), dir)

	require.ErrorIs(t, err, werrors.Git(""))
	assert.NotErrorIs(t, err, werrors.NotAGitRepository(""))
}

func TestGitRootCommitsListsTheFirstCommitOfTheRepository(t *testing.T) {
	root := newGitRepo(t)
	first := strings.TrimSpace(runGit(t, root, "rev-list", "--max-parents=0", "HEAD"))

	roots, err := git.NewRepositoryAdapter().RootCommits(context.Background(), root)

	require.NoError(t, err)
	assert.Equal(t, []string{first}, roots)
}

func TestGitRootCommitsIsEmptyForAnUnbornRepository(t *testing.T) {
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")

	roots, err := git.NewRepositoryAdapter().RootCommits(context.Background(), dir)

	require.NoError(t, err)
	assert.Empty(t, roots)
}

func TestGitRootCommitsDifferBetweenUnrelatedRepositories(t *testing.T) {
	first := newGitRepo(t)
	// A second repository with a different first commit: two commits made in the same second with the same
	// content would be identical, which would make the two repositories look like one project.
	second := t.TempDir()
	runGit(t, second, "init", "-q", "-b", "main")
	runGit(t, second, "config", "user.email", "test@example.com")
	runGit(t, second, "config", "user.name", "Test")
	runGit(t, second, "commit", "-q", "--allow-empty", "-m", "unrelated project")
	adapter := git.NewRepositoryAdapter()

	firstRoots, err := adapter.RootCommits(context.Background(), first)
	require.NoError(t, err)
	secondRoots, err := adapter.RootCommits(context.Background(), second)
	require.NoError(t, err)

	assert.NotEqual(t, firstRoots, secondRoots)
}

func TestGitRootCommitsReportsAMissingFolderAndAPlainFolder(t *testing.T) {
	dir := t.TempDir()
	adapter := git.NewRepositoryAdapter()

	_, missingErr := adapter.RootCommits(context.Background(), filepath.Join(dir, "nope"))
	_, plainErr := adapter.RootCommits(context.Background(), dir)

	require.ErrorIs(t, missingErr, werrors.PathNotFound(""))
	require.ErrorIs(t, plainErr, werrors.NotAGitRepository(""))
}

func TestGitRootCommitsSurviveAMovedFolder(t *testing.T) {
	root := newGitRepo(t)
	adapter := git.NewRepositoryAdapter()
	before, err := adapter.RootCommits(context.Background(), root)
	require.NoError(t, err)
	moved := filepath.Join(t.TempDir(), "moved-repo")
	require.NoError(t, os.Rename(root, moved))

	after, err := adapter.RootCommits(context.Background(), moved)

	require.NoError(t, err)
	assert.Equal(t, before, after)
}

func TestGitHasLinkedWorktreesCountsOnlyWorktreesBesidesTheMainOne(t *testing.T) {
	root := newGitRepo(t)
	adapter := git.NewRepositoryAdapter()

	without, err := adapter.HasLinkedWorktrees(context.Background(), root)
	require.NoError(t, err)
	runGit(t, root, "worktree", "add", "-q", "-b", "task", filepath.Join(t.TempDir(), "task-wt"))
	with, err := adapter.HasLinkedWorktrees(context.Background(), root)
	require.NoError(t, err)

	assert.False(t, without)
	assert.True(t, with)
}

// A linked worktree lives outside the moved repository. Once the repository moves, git no longer finds the
// worktree; RepairWorktrees, run from the new root, reconnects it. This is the S5 safety net.
func TestGitRepairWorktreesReconnectsALinkedWorktreeAfterTheRepositoryMoved(t *testing.T) {
	ctx := context.Background()
	root := newGitRepo(t)
	worktree := filepath.Join(t.TempDir(), "task-wt")
	runGit(t, root, "worktree", "add", "-q", "-b", "task", worktree)
	moved := filepath.Join(t.TempDir(), "moved-repo")
	require.NoError(t, os.Rename(root, moved))
	adapter := git.NewRepositoryAdapter()
	require.Error(t, runGitError(worktree, "rev-parse", "--show-toplevel"), "the worktree is broken before the repair")

	require.NoError(t, adapter.RepairWorktrees(ctx, moved))

	info, err := adapter.Inspect(ctx, worktree)
	require.NoError(t, err)
	assert.Equal(t, evalSymlinks(t, worktree), info.Root)
	list := runGit(t, moved, "worktree", "list", "--porcelain")
	assert.Contains(t, list, evalSymlinks(t, worktree))
	assert.NotContains(t, list, "prunable")
}

// runGitError runs git in dir and returns its error, nil when it succeeds. Unlike runGit it does not stop the test.
func runGitError(dir string, args ...string) error {
	return exec.Command("git", append([]string{"-C", dir}, args...)...).Run()
}
