package workspaces_test

import (
	"context"
	"os"
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
