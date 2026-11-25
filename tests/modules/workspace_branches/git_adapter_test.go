// Package branchestest holds the integration tests of the workspace_branches git adapter. Every test works on a
// real repository and a bare remote created in t.TempDir(), which the testing framework removes afterwards.
package branchestest

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/workspace_branches/domain/errors"
	"iworkflow/internal/modules/workspace_branches/domain/models"
	"iworkflow/internal/modules/workspace_branches/infrastructure/adapters/outbound/git"
)

// gitIn runs git in dir with a fixed identity and fails the test on error. It is the test's own setup helper.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_TERMINAL_PROMPT=0",
	)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(), "git %s: %s", strings.Join(args, " "), stderr.String())
	return strings.TrimSpace(stdout.String())
}

// fixture is a repository with origin pointing at a bare remote. It has the branches main, feature/old and
// feature/keep (checked out in a linked worktree), all pushed, and origin/HEAD set.
type fixture struct {
	repo   string
	remote string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	base := t.TempDir()
	repo := filepath.Join(base, "repo")
	remote := filepath.Join(base, "remote.git")
	gitIn(t, base, "init", "-q", "-b", "main", repo)
	gitIn(t, repo, "commit", "-q", "--allow-empty", "-m", "init")
	gitIn(t, base, "init", "-q", "--bare", remote)
	gitIn(t, repo, "remote", "add", "origin", remote)
	gitIn(t, repo, "branch", "feature/old")
	gitIn(t, repo, "branch", "feature/keep")
	gitIn(t, repo, "push", "-q", "origin", "main", "feature/old", "feature/keep")
	gitIn(t, repo, "remote", "set-head", "origin", "main")
	gitIn(t, repo, "worktree", "add", "-q", filepath.Join(base, "wt"), "feature/keep")
	return fixture{repo: repo, remote: remote}
}

func branchNames(t *testing.T, dir string, pattern string) []string {
	t.Helper()
	out := gitIn(t, dir, "for-each-ref", "--format=%(refname)", pattern)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func find(t *testing.T, branches []models.Branch, fullName string) models.Branch {
	t.Helper()
	for _, branch := range branches {
		if branch.FullName() == fullName {
			return branch
		}
	}
	t.Fatalf("branch %q not listed", fullName)
	return models.Branch{}
}

func TestListBranchesReportsLocalRemoteCurrentAndWorktreeFlags(t *testing.T) {
	fx := newFixture(t)
	branches, err := git.NewAdapter().ListBranches(context.Background(), fx.repo)
	require.NoError(t, err)

	main := find(t, branches, "main")
	assert.False(t, main.Remote)
	assert.True(t, main.Current)
	assert.True(t, main.CheckedOut)

	assert.True(t, find(t, branches, "feature/keep").CheckedOut, "checked out in the linked worktree")
	assert.False(t, find(t, branches, "feature/old").CheckedOut)

	remote := find(t, branches, "origin/feature/old")
	assert.True(t, remote.Remote)
	assert.Equal(t, "origin", remote.RemoteName)
	assert.Equal(t, "feature/old", remote.Name)

	assert.Equal(t, "HEAD", find(t, branches, "origin/HEAD").Name, "the default ref is listed so it can be shown as protected")
}

func TestDeleteLocalRemovesTheBranch(t *testing.T) {
	fx := newFixture(t)
	require.NoError(t, git.NewAdapter().DeleteLocal(context.Background(), fx.repo, "feature/old"))

	assert.NotContains(t, branchNames(t, fx.repo, "refs/heads/"), "refs/heads/feature/old")
	assert.Contains(t, branchNames(t, fx.repo, "refs/heads/"), "refs/heads/main", "other branches stay")
}

func TestDeleteRemoteRemovesTheBranchFromTheRemote(t *testing.T) {
	fx := newFixture(t)
	require.NoError(t, git.NewAdapter().DeleteRemote(context.Background(), fx.repo, "origin", "feature/old"))

	assert.NotContains(t, branchNames(t, fx.remote, "refs/heads/"), "refs/heads/feature/old")
	assert.Contains(t, branchNames(t, fx.remote, "refs/heads/"), "refs/heads/main")
}

func TestDeleteRemoteOfAMissingBranchFailsAndChangesNothing(t *testing.T) {
	fx := newFixture(t)
	before := branchNames(t, fx.remote, "refs/heads/")

	err := git.NewAdapter().DeleteRemote(context.Background(), fx.repo, "origin", "never-pushed")

	require.Error(t, err)
	assert.ErrorIs(t, err, werrors.ErrBranchNotFound, "a branch the remote does not list is an explicit outcome")
	assert.Equal(t, before, branchNames(t, fx.remote, "refs/heads/"))
}

func TestOptionLikeNamesNeverReachGit(t *testing.T) {
	fx := newFixture(t)
	adapter := git.NewAdapter()
	localBefore := branchNames(t, fx.repo, "refs/heads/")
	remoteBefore := branchNames(t, fx.remote, "refs/heads/")

	require.Error(t, adapter.DeleteLocal(context.Background(), fx.repo, "-D"))
	require.Error(t, adapter.DeleteLocal(context.Background(), fx.repo, "--force"))
	require.Error(t, adapter.DeleteLocal(context.Background(), fx.repo, ""))
	require.Error(t, adapter.DeleteRemote(context.Background(), fx.repo, "origin", "--delete"))
	require.Error(t, adapter.DeleteRemote(context.Background(), fx.repo, "--upload-pack=touch-x", "feature/old"))

	assert.Equal(t, localBefore, branchNames(t, fx.repo, "refs/heads/"))
	assert.Equal(t, remoteBefore, branchNames(t, fx.remote, "refs/heads/"))
}

func TestDeleteRemoteOfABranchNamedLikeATagDeletesOnlyTheBranch(t *testing.T) {
	fx := newFixture(t)
	// The remote gets a branch tags/x and a tag x: the bare name tags/x would match both refs/heads/tags/x and refs/tags/x.
	gitIn(t, fx.repo, "push", "-q", "origin", "refs/heads/feature/old:refs/heads/tags/x", "refs/heads/main:refs/tags/x")
	adapter := git.NewAdapter()

	branches, err := adapter.ListBranches(context.Background(), fx.repo)
	require.NoError(t, err)
	assert.True(t, find(t, branches, "origin/tags/x").Remote)

	require.NoError(t, adapter.DeleteRemote(context.Background(), fx.repo, "origin", "tags/x"))

	assert.NotContains(t, branchNames(t, fx.remote, "refs/heads/"), "refs/heads/tags/x")
	assert.Equal(t, []string{"refs/tags/x"}, branchNames(t, fx.remote, "refs/tags/"), "the tag of the same short name stays")
	assert.Contains(t, branchNames(t, fx.remote, "refs/heads/"), "refs/heads/feature/old", "other branches stay")
}

func TestListBranchesAttributesARemoteToItsLongestMatchingName(t *testing.T) {
	fx := newFixture(t)
	nested := filepath.Join(filepath.Dir(fx.remote), "nested.git")
	gitIn(t, filepath.Dir(fx.remote), "init", "-q", "--bare", nested)
	// Remotes "a" and "a/b" are both configured; refs/remotes/a/b/c belongs to remote a/b. git remote add refuses
	// a name that is a path prefix of another remote, so remote a is written as config keys instead.
	gitIn(t, fx.repo, "remote", "add", "a/b", nested)
	gitIn(t, fx.repo, "config", "remote.a.url", fx.remote)
	gitIn(t, fx.repo, "config", "remote.a.fetch", "+refs/heads/main:refs/remotes/a/main")
	gitIn(t, fx.repo, "push", "-q", "a/b", "refs/heads/main:refs/heads/c")
	adapter := git.NewAdapter()

	branches, err := adapter.ListBranches(context.Background(), fx.repo)
	require.NoError(t, err)
	branch := find(t, branches, "a/b/c")
	assert.Equal(t, "a/b", branch.RemoteName)
	assert.Equal(t, "c", branch.Name)

	require.NoError(t, adapter.DeleteRemote(context.Background(), fx.repo, "a/b", "c"))
	assert.NotContains(t, branchNames(t, nested, "refs/heads/"), "refs/heads/c")
}

func TestCancelledContextDeletesNothing(t *testing.T) {
	fx := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.Error(t, git.NewAdapter().DeleteLocal(ctx, fx.repo, "feature/old"))
	assert.Contains(t, branchNames(t, fx.repo, "refs/heads/"), "refs/heads/feature/old")
}

// installPushShim puts a `git` shim first on PATH. It answers `git push` with exit 0 and does nothing, which is what
// a push of a ref that vanished in between can look like. Every other command runs the real git.
func installPushShim(t *testing.T) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	require.NoError(t, err)
	dir := t.TempDir()
	script := "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = push ]; then exit 0; fi\ndone\nexec \"$IWF_TEST_REAL_GIT\" \"$@\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("IWF_TEST_REAL_GIT", realGit)
}

func TestDeleteRemoteReportsAPushThatLeavesTheRefInPlace(t *testing.T) {
	fx := newFixture(t)
	installPushShim(t)

	err := git.NewAdapter().DeleteRemote(context.Background(), fx.repo, "origin", "feature/old")

	require.Error(t, err, "a push that exits 0 but leaves the ref listed is not a deletion")
	assert.Contains(t, branchNames(t, fx.remote, "refs/heads/"), "refs/heads/feature/old")
}

func TestListBranchesStopsAtAnExpiredDeadline(t *testing.T) {
	fx := newFixture(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	_, err := git.NewAdapter().ListBranches(ctx, fx.repo)

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestListBranchesStopsWhenCancelled(t *testing.T) {
	fx := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := git.NewAdapter().ListBranches(ctx, fx.repo)

	assert.ErrorIs(t, err, context.Canceled)
}

func TestDeleteRemoteStopsAtAnExpiredDeadlineAndDeletesNothing(t *testing.T) {
	fx := newFixture(t)
	before := branchNames(t, fx.remote, "refs/heads/")
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	err := git.NewAdapter().DeleteRemote(ctx, fx.repo, "origin", "feature/old")

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, before, branchNames(t, fx.remote, "refs/heads/"))
}

func TestListRemotesIsEmptyWhenNoRemoteIsConfigured(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo")
	gitIn(t, filepath.Dir(repo), "init", "-q", "-b", "main", repo)

	remotes, err := git.NewAdapter().ListRemotes(context.Background(), repo)

	require.NoError(t, err)
	assert.Empty(t, remotes)
}

func TestListBranchesOfADetachedHeadHasNoCurrentBranch(t *testing.T) {
	fx := newFixture(t)
	gitIn(t, fx.repo, "checkout", "-q", "--detach")

	branches, err := git.NewAdapter().ListBranches(context.Background(), fx.repo)

	require.NoError(t, err)
	for _, branch := range branches {
		assert.False(t, branch.Current, branch.FullName())
	}
	main := find(t, branches, "main")
	assert.False(t, main.Current)
	assert.False(t, main.CheckedOut, "main is no longer checked out anywhere")
}

func TestListBranchesOfANonRepositoryFails(t *testing.T) {
	dir := t.TempDir()
	adapter := git.NewAdapter()

	_, err := adapter.ListBranches(context.Background(), dir)
	require.Error(t, err)

	_, err = adapter.ListRemotes(context.Background(), dir)
	assert.Error(t, err)
}
