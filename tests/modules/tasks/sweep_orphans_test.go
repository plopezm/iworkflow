package tasks_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/git"
)

// leftover creates a folder without git metadata, the way a deleted task leaves its files behind.
func leftover(t *testing.T, path string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(path, "target", "debug"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(path, "target", "debug", "big.bin"), []byte("x"), 0o644))
	return path
}

// Rust: git_cli_worktree_adapter_test::sweep_orphans_removes_unowned_folders_without_git_metadata
func TestSweepRemovesUnownedFoldersWithoutGitMetadata(t *testing.T) {
	f := newWorktreeFixture(t)
	orphan := leftover(t, f.adapter.WorktreePath("demo", "gone-task"))
	owned := leftover(t, f.adapter.WorktreePath("demo", "backlog-task"))

	removed, err := f.adapter.SweepOrphans(bg(), []string{owned})

	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.NoDirExists(t, orphan)
	assert.DirExists(t, owned, "a folder owned by an open task is kept")
	assert.DirExists(t, f.worktree, "a live worktree is never swept")
}

// Rust: sweep_orphans_keeps_unowned_live_worktrees
func TestSweepKeepsUnownedLiveWorktrees(t *testing.T) {
	f := newWorktreeFixture(t)

	removed, err := f.adapter.SweepOrphans(bg(), nil)

	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.FileExists(t, filepath.Join(f.worktree, ".git"))
}

// Rust: sweep_orphans_purges_the_trash_and_leaves_hidden_dirs_alone
func TestSweepPurgesTheTrashAndLeavesHiddenDirsAlone(t *testing.T) {
	f := newWorktreeFixture(t)
	parked := filepath.Join(f.root, ".trash", "half-deleted", "target")
	require.NoError(t, os.MkdirAll(parked, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(parked, "x.bin"), []byte("x"), 0o644))
	mergeDir := filepath.Join(f.root, ".merge", "pending")
	require.NoError(t, os.MkdirAll(mergeDir, 0o755))

	_, err := f.adapter.SweepOrphans(bg(), nil)

	require.NoError(t, err)
	entries, err := os.ReadDir(filepath.Join(f.root, ".trash"))
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.DirExists(t, mergeDir)
}

// Rust: sweep_orphans_tolerates_a_missing_root
func TestSweepToleratesAMissingRoot(t *testing.T) {
	adapter, err := git.NewWorktreeAdapter(filepath.Join(t.TempDir(), "never-created"))
	require.NoError(t, err)

	removed, err := adapter.SweepOrphans(bg(), nil)

	require.NoError(t, err)
	assert.Zero(t, removed)
}

// Keep set: a live task whose folder has no .git yet (its worktree is being created) is protected through
// the path the task stores.
func TestSweepKeepsTheFolderOfAnOpenTaskEvenBeforeItHasGitMetadata(t *testing.T) {
	f := newWorktreeFixture(t)
	starting := leftover(t, f.adapter.WorktreePath("demo", "starting-task"))

	removed, err := f.adapter.SweepOrphans(bg(), []string{starting})

	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.DirExists(t, starting)
}

// Keep set: the keep list may name the folder through a symbolic link of the root. The protection must
// still hold, because a protected folder that is deleted is data loss.
func TestSweepKeepSetMatchesThroughASymbolicLinkOfTheRoot(t *testing.T) {
	f := newWorktreeFixture(t)
	owned := leftover(t, f.adapter.WorktreePath("demo", "owned-task"))
	alias := filepath.Join(t.TempDir(), "alias")
	require.NoError(t, os.Symlink(f.root, alias))

	removed, err := f.adapter.SweepOrphans(bg(), []string{filepath.Join(alias, "demo", "owned-task")})

	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.DirExists(t, owned)
}

// Keep set: a relative path in keep is compared as an absolute one.
func TestSweepKeepSetAcceptsRelativePaths(t *testing.T) {
	f := newWorktreeFixture(t)
	owned := leftover(t, f.adapter.WorktreePath("demo", "owned-task"))
	wd, err := os.Getwd()
	require.NoError(t, err)
	relative, err := filepath.Rel(wd, owned)
	require.NoError(t, err)

	removed, err := f.adapter.SweepOrphans(bg(), []string{relative})

	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.DirExists(t, owned)
}

// Dirty worktree: a live worktree with uncommitted work is kept whether or not a task owns it, because
// the folder still has its git metadata.
func TestSweepKeepsDirtyLiveWorktrees(t *testing.T) {
	f := newWorktreeFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(f.worktree, "trabajo.txt"), []byte("sin commitear\n"), 0o644))

	removed, err := f.adapter.SweepOrphans(bg(), nil)

	require.NoError(t, err)
	assert.Zero(t, removed)
	assert.FileExists(t, filepath.Join(f.worktree, "trabajo.txt"))
}

// Documented behavior (see the report): a folder that has lost its .git is treated as a leftover, even when
// it holds files, and it is deleted unless it is kept. Whoever changes this must decide what happens to
// the work inside it.
func TestSweepDeletesUnownedFolderWithoutGitEvenWhenItHoldsFiles(t *testing.T) {
	f := newWorktreeFixture(t)
	lost := f.adapter.WorktreePath("demo", "lost-git")
	require.NoError(t, os.MkdirAll(lost, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(lost, "notas.txt"), []byte("trabajo\n"), 0o644))

	removed, err := f.adapter.SweepOrphans(bg(), nil)

	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.NoDirExists(t, lost)
}

// Symbolic links: a link in a workspace folder that points outside the root is never followed or deleted.
func TestSweepNeverFollowsSymbolicLinksOutsideTheRoot(t *testing.T) {
	f := newWorktreeFixture(t)
	outside := realDir(t, t.TempDir())
	precious := leftover(t, filepath.Join(outside, "precious"))
	require.NoError(t, os.Symlink(precious, filepath.Join(f.root, "demo", "link-to-precious")))
	require.NoError(t, os.Symlink(outside, filepath.Join(f.root, "linked-workspace")))

	_, err := f.adapter.SweepOrphans(bg(), nil)

	require.NoError(t, err)
	assert.DirExists(t, precious)
	assert.DirExists(t, outside)
}

// Cancellation: a context that ends before the sweep starts deletes nothing.
func TestSweepStopsWhenTheContextIsCancelled(t *testing.T) {
	f := newWorktreeFixture(t)
	orphan := leftover(t, f.adapter.WorktreePath("demo", "gone-task"))
	ctx, cancel := context.WithCancel(bg())
	cancel()

	_, err := f.adapter.SweepOrphans(ctx, nil)

	assert.ErrorIs(t, err, context.Canceled)
	assert.DirExists(t, orphan)
}
