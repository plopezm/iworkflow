package tasks_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/git"
)

const (
	taskBranch = "pi/feature-1234"
	pinRef     = "refs/iworkflow/tasks/test"
)

// worktreeFixture is a repository with one task worktree on taskBranch, under a managed root.
type worktreeFixture struct {
	repo     string
	root     string
	worktree string
	adapter  *git.WorktreeAdapter
}

func newWorktreeFixture(t *testing.T) worktreeFixture {
	t.Helper()
	repo := newGitRepository(t)
	adapter, root := newAdapter(t)
	worktree := adapter.WorktreePath("demo", "1234-feature")
	require.NoError(t, adapter.Create(bg(), repo, "main", taskBranch, worktree))
	return worktreeFixture{repo: repo, root: root, worktree: worktree, adapter: adapter}
}

// commitFile writes a file in the worktree and commits it.
func (f worktreeFixture) commitFile(t *testing.T, name string, content string, message string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(f.worktree, name), []byte(content), 0o644))
	committed, err := f.adapter.CommitAll(bg(), f.worktree, message)
	require.NoError(t, err)
	require.True(t, committed)
}

// makeConflict changes README.md on the task branch and on main, so that a merge conflicts.
func (f worktreeFixture) makeConflict(t *testing.T) {
	t.Helper()
	f.commitFile(t, "README.md", "desde la tarea\n", "pi: cambia")
	require.NoError(t, os.WriteFile(filepath.Join(f.repo, "README.md"), []byte("desde main\n"), 0o644))
	gitCmd(t, f.repo, "commit", "-qam", "main cambia")
}

// Rust: git_cli_worktree_adapter_test::creates_worktree_on_new_branch
func TestCreateWorktreeOnNewBranch(t *testing.T) {
	f := newWorktreeFixture(t)
	assert.True(t, f.adapter.Exists(bg(), f.worktree))
	assert.Equal(t, taskBranch, gitCmd(t, f.worktree, "branch", "--show-current"))
}

// Rust: commit_all_reports_whether_there_were_changes
func TestCommitAllReportsWhetherThereWereChanges(t *testing.T) {
	f := newWorktreeFixture(t)
	nothing, err := f.adapter.CommitAll(bg(), f.worktree, "vacío")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(f.worktree, "nuevo.txt"), []byte("x\n"), 0o644))
	something, err := f.adapter.CommitAll(bg(), f.worktree, "pi: nuevo")
	require.NoError(t, err)
	assert.False(t, nothing)
	assert.True(t, something)
	assert.Equal(t, "pi: nuevo", gitCmd(t, f.worktree, "log", "-1", "--format=%s"))
}

// Rust: diff_lists_changes_and_commits
func TestDiffListsChangesAndCommits(t *testing.T) {
	f := newWorktreeFixture(t)
	f.commitFile(t, "nuevo.txt", "hola\n", "pi: nuevo")

	diff, err := f.adapter.Diff(bg(), f.repo, "main", taskBranch)

	require.NoError(t, err)
	assert.Contains(t, diff.Stat, "nuevo.txt")
	assert.Contains(t, diff.Patch, "+hola")
	assert.Len(t, diff.Commits, 1)
	assert.False(t, diff.Truncated)
}

// Rust: diff_live_includes_commits_uncommitted_and_untracked_changes
func TestDiffLiveIncludesCommitsUncommittedAndUntrackedChanges(t *testing.T) {
	f := newWorktreeFixture(t)
	f.commitFile(t, "committed.txt", "uno\n", "pi: committed")
	require.NoError(t, os.WriteFile(filepath.Join(f.worktree, "README.md"), []byte("editado sin commit\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(f.worktree, "untracked.txt"), []byte("nuevo\n"), 0o644))

	diff, err := f.adapter.DiffLive(bg(), f.worktree, "main")

	require.NoError(t, err)
	assert.Contains(t, diff.Patch, "+uno")
	assert.Contains(t, diff.Patch, "+editado sin commit")
	assert.Contains(t, diff.Patch, "+nuevo")
	assert.Contains(t, diff.Patch, "b/untracked.txt")
	assert.Contains(t, diff.Stat, "untracked.txt")
	assert.Contains(t, diff.Stat, "README.md")
	assert.Len(t, diff.Commits, 1)
	// The agent's index is left alone: the new file is still untracked.
	assert.Contains(t, gitCmd(t, f.worktree, "status", "--porcelain"), "?? untracked.txt")
}

// Rust: diff_live_is_empty_for_a_clean_worktree
func TestDiffLiveIsEmptyForACleanWorktree(t *testing.T) {
	f := newWorktreeFixture(t)
	diff, err := f.adapter.DiffLive(bg(), f.worktree, "main")
	require.NoError(t, err)
	assert.Empty(t, diff.Patch)
	assert.Empty(t, diff.Commits)
}

// Rust: diff_fingerprint_changes_only_with_commits
func TestDiffFingerprintChangesOnlyWithCommits(t *testing.T) {
	f := newWorktreeFixture(t)
	initial, err := f.adapter.DiffFingerprint(bg(), f.repo, "main", taskBranch)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(f.worktree, "nuevo.txt"), []byte("hola\n"), 0o644))
	uncommitted, err := f.adapter.DiffFingerprint(bg(), f.repo, "main", taskBranch)
	require.NoError(t, err)
	_, err = f.adapter.CommitAll(bg(), f.worktree, "pi: nuevo")
	require.NoError(t, err)
	committed, err := f.adapter.DiffFingerprint(bg(), f.repo, "main", taskBranch)
	require.NoError(t, err)
	_, missing := f.adapter.DiffFingerprint(bg(), f.repo, "main", "no-existe")

	assert.Equal(t, initial, uncommitted, "uncommitted changes are not part of the diff")
	assert.NotEqual(t, initial, committed)
	assert.Error(t, missing)
}

// Rust: merges_into_checked_out_base_and_removes_worktree
func TestMergeIntoCheckedOutBaseAndRemoveWorktree(t *testing.T) {
	f := newWorktreeFixture(t)
	f.commitFile(t, "nuevo.txt", "hola\n", "pi: nuevo")

	outcome, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")
	require.NoError(t, err)
	require.NoError(t, f.adapter.Remove(bg(), f.repo, f.worktree, taskBranch))

	assert.False(t, outcome.Conflict)
	assert.FileExists(t, filepath.Join(f.repo, "nuevo.txt"))
	assert.Equal(t, "Merge tarea", gitCmd(t, f.repo, "log", "-1", "--format=%s"))
	assert.False(t, f.adapter.Exists(bg(), f.worktree))
	assert.Empty(t, gitCmd(t, f.repo, "branch", "--list", taskBranch))
}

// Rust: remove_deletes_unregistered_folder
func TestRemoveDeletesUnregisteredFolder(t *testing.T) {
	f := newWorktreeFixture(t)
	stray := filepath.Join(f.root, "stray")
	require.NoError(t, os.MkdirAll(stray, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(stray, "x.txt"), []byte("x"), 0o644))

	require.NoError(t, f.adapter.Remove(bg(), f.repo, stray, "pi/inexistente"))

	assert.NoDirExists(t, stray)
}

// Rust: remove_propagates_failure_for_locked_worktree
func TestRemovePropagatesFailureForLockedWorktree(t *testing.T) {
	f := newWorktreeFixture(t)
	gitCmd(t, f.repo, "worktree", "lock", f.worktree)

	err := f.adapter.Remove(bg(), f.repo, f.worktree, taskBranch)

	assert.Error(t, err)
	assert.DirExists(t, f.worktree)
}

// Rust: merges_through_temporary_worktree_when_base_not_checked_out
func TestMergeThroughTemporaryWorktreeWhenBaseIsNotCheckedOut(t *testing.T) {
	f := newWorktreeFixture(t)
	gitCmd(t, f.repo, "checkout", "-q", "-b", "otra")
	f.commitFile(t, "nuevo.txt", "hola\n", "pi: nuevo")

	outcome, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")

	require.NoError(t, err)
	assert.False(t, outcome.Conflict)
	assert.Equal(t, "Merge tarea", gitCmd(t, f.repo, "log", "-1", "--format=%s", "main"))
	assert.Equal(t, "otra", gitCmd(t, f.repo, "branch", "--show-current"))
	assert.NotContains(t, gitCmd(t, f.repo, "worktree", "list"), ".merge")
}

// Rust: conflict_is_reported_and_base_left_clean
func TestConflictIsReportedAndBaseLeftClean(t *testing.T) {
	f := newWorktreeFixture(t)
	f.makeConflict(t)

	outcome, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")

	require.NoError(t, err)
	assert.True(t, outcome.Conflict)
	assert.Equal(t, []string{"README.md"}, outcome.Files)
	assert.Empty(t, gitCmd(t, f.repo, "status", "--porcelain"))
	assert.DirExists(t, f.worktree)
}

// Rust: dirty_base_blocks_merge
func TestDirtyBaseBlocksMerge(t *testing.T) {
	f := newWorktreeFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(f.repo, "README.md"), []byte("sin commitear\n"), 0o644))

	_, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")

	assert.ErrorIs(t, err, werrors.MergeBlocked(""))
}

// Rust: remove_tolerates_missing_worktree_folder
func TestRemoveToleratesMissingWorktreeFolder(t *testing.T) {
	f := newWorktreeFixture(t)
	require.NoError(t, os.RemoveAll(f.worktree))

	require.NoError(t, f.adapter.Remove(bg(), f.repo, f.worktree, taskBranch))

	assert.Empty(t, gitCmd(t, f.repo, "branch", "--list", taskBranch))
}

// Rust: concurrent_merges_into_the_same_base_both_succeed. The merge slot serializes them.
func TestConcurrentMergesIntoTheSameBaseBothSucceed(t *testing.T) {
	f := newWorktreeFixture(t)
	secondBranch := "pi/feature-5678"
	second := f.adapter.WorktreePath("demo", "5678-feature")
	require.NoError(t, f.adapter.Create(bg(), f.repo, "main", secondBranch, second))
	require.NoError(t, os.WriteFile(filepath.Join(f.worktree, "a.txt"), []byte("a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(second, "b.txt"), []byte("b\n"), 0o644))
	_, err := f.adapter.CommitAll(bg(), f.worktree, "a")
	require.NoError(t, err)
	_, err = f.adapter.CommitAll(bg(), second, "b")
	require.NoError(t, err)

	var wg sync.WaitGroup
	var outcomeA, outcomeB models.MergeOutcome
	var errA, errB error
	wg.Add(2)
	go func() {
		defer wg.Done()
		outcomeA, errA = f.adapter.Merge(bg(), f.repo, "main", taskBranch, "merge a")
	}()
	go func() {
		defer wg.Done()
		outcomeB, errB = f.adapter.Merge(bg(), f.repo, "main", secondBranch, "merge b")
	}()
	wg.Wait()

	require.NoError(t, errA)
	require.NoError(t, errB)
	assert.False(t, outcomeA.Conflict)
	assert.False(t, outcomeB.Conflict)
	assert.FileExists(t, filepath.Join(f.repo, "a.txt"))
	assert.FileExists(t, filepath.Join(f.repo, "b.txt"))
}

// Rust: conflict_in_checked_out_base_leaves_no_merge_pending
func TestConflictInCheckedOutBaseLeavesNoMergePending(t *testing.T) {
	f := newWorktreeFixture(t)
	f.makeConflict(t)

	outcome, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")

	require.NoError(t, err)
	assert.True(t, outcome.Conflict)
	_, pendingErr := gitRunExit(t, f.repo, "rev-parse", "-q", "--verify", "MERGE_HEAD")
	assert.Error(t, pendingErr, "MERGE_HEAD must not exist after the abort")
	assert.Empty(t, gitCmd(t, f.repo, "status", "--porcelain"))
}

// Rust: conflict_through_temporary_worktree_leaves_nothing_behind
func TestConflictThroughTemporaryWorktreeLeavesNothingBehind(t *testing.T) {
	f := newWorktreeFixture(t)
	f.makeConflict(t)
	gitCmd(t, f.repo, "checkout", "-q", "-b", "otra")
	before := gitCmd(t, f.repo, "rev-parse", "main")

	outcome, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")

	require.NoError(t, err)
	assert.True(t, outcome.Conflict)
	assert.Equal(t, []string{"README.md"}, outcome.Files)
	assert.Equal(t, before, gitCmd(t, f.repo, "rev-parse", "main"))
	assert.NotContains(t, gitCmd(t, f.repo, "worktree", "list"), ".merge")
	entries, err := os.ReadDir(filepath.Join(f.root, ".merge"))
	if !os.IsNotExist(err) {
		require.NoError(t, err)
	}
	assert.Empty(t, entries)
}

// Rust: stale_merge_worktree_does_not_block_later_merges
func TestStaleMergeWorktreeDoesNotBlockLaterMerges(t *testing.T) {
	f := newWorktreeFixture(t)
	gitCmd(t, f.repo, "checkout", "-q", "-b", "otra")
	f.commitFile(t, "nuevo.txt", "hola\n", "pi: nuevo")
	stale := filepath.Join(f.root, ".merge", models.Slugify(taskBranch))
	require.NoError(t, os.MkdirAll(filepath.Dir(stale), 0o755))
	gitCmd(t, f.repo, "worktree", "add", "--detach", stale, "main")

	outcome, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")

	require.NoError(t, err)
	assert.False(t, outcome.Conflict)
	assert.Equal(t, "Merge tarea", gitCmd(t, f.repo, "log", "-1", "--format=%s", "main"))
}

// Rust: exists_reflects_worktree_folder_presence
func TestExistsReflectsWorktreeFolderPresence(t *testing.T) {
	f := newWorktreeFixture(t)
	assert.True(t, f.adapter.Exists(bg(), f.worktree))
	require.NoError(t, os.RemoveAll(f.worktree))
	assert.False(t, f.adapter.Exists(bg(), f.worktree))
}

// Rust: pin_range_returns_merge_base_and_tip_and_creates_ref
func TestPinRangeReturnsMergeBaseAndTipAndCreatesRef(t *testing.T) {
	f := newWorktreeFixture(t)
	mainSHA := gitCmd(t, f.repo, "rev-parse", "main")
	f.commitFile(t, "nuevo.txt", "hola\n", "pi: nuevo")

	rng, err := f.adapter.PinRange(bg(), f.repo, "main", taskBranch, pinRef)

	require.NoError(t, err)
	assert.Equal(t, mainSHA, rng.BaseSHA)
	assert.Equal(t, gitCmd(t, f.worktree, "rev-parse", "HEAD"), rng.HeadSHA)
	assert.Equal(t, rng.HeadSHA, gitCmd(t, f.repo, "rev-parse", pinRef))
}

// Rust: diff_range_matches_diff_after_merge_and_cleanup
func TestDiffRangeMatchesDiffAfterMergeAndCleanup(t *testing.T) {
	f := newWorktreeFixture(t)
	f.commitFile(t, "nuevo.txt", "hola\n", "pi: nuevo")
	f.commitFile(t, "otro.txt", "adios\n", "pi: otro")
	before, err := f.adapter.Diff(bg(), f.repo, "main", taskBranch)
	require.NoError(t, err)
	rng, err := f.adapter.PinRange(bg(), f.repo, "main", taskBranch, pinRef)
	require.NoError(t, err)

	outcome, err := f.adapter.Merge(bg(), f.repo, "main", taskBranch, "Merge tarea")
	require.NoError(t, err)
	require.NoError(t, f.adapter.Remove(bg(), f.repo, f.worktree, taskBranch))
	after, err := f.adapter.DiffRange(bg(), f.repo, rng)
	require.NoError(t, err)

	assert.False(t, outcome.Conflict)
	assert.Empty(t, gitCmd(t, f.repo, "branch", "--list", taskBranch))
	assert.Equal(t, before.Patch, after.Patch)
	assert.Equal(t, before.Stat, after.Stat)
	assert.Equal(t, before.Commits, after.Commits)
	assert.Len(t, after.Commits, 2)
	assert.Equal(t, rng.HeadSHA, gitCmd(t, f.repo, "rev-parse", pinRef))
}

// Rust: diff_range_truncates_long_patches_like_diff
func TestDiffRangeTruncatesLongPatchesLikeDiff(t *testing.T) {
	f := newWorktreeFixture(t)
	big := strings.Repeat("linea de relleno\n", 80_000)
	f.commitFile(t, "grande.txt", big, "pi: grande")
	rng, err := f.adapter.PinRange(bg(), f.repo, "main", taskBranch, pinRef)
	require.NoError(t, err)

	plain, err := f.adapter.Diff(bg(), f.repo, "main", taskBranch)
	require.NoError(t, err)
	ranged, err := f.adapter.DiffRange(bg(), f.repo, rng)
	require.NoError(t, err)

	assert.True(t, ranged.Truncated)
	assert.Len(t, ranged.Patch, len(plain.Patch))
	assert.Equal(t, plain.Patch, ranged.Patch)
}

// Rust: remove_deletes_folder_that_is_no_longer_a_worktree
func TestRemoveDeletesFolderThatIsNoLongerAWorktree(t *testing.T) {
	f := newWorktreeFixture(t)
	require.NoError(t, os.Remove(filepath.Join(f.worktree, ".git")))

	require.NoError(t, f.adapter.Remove(bg(), f.repo, f.worktree, taskBranch))

	assert.NoDirExists(t, f.worktree)
	assert.Empty(t, gitCmd(t, f.repo, "branch", "--list", taskBranch))
	assert.NotContains(t, gitCmd(t, f.repo, "worktree", "list", "--porcelain"), f.worktree)
}

// Containment: a folder outside the managed root is never removed.
func TestRemoveRefusesToDeleteFoldersOutsideTheManagedRoot(t *testing.T) {
	f := newWorktreeFixture(t)
	outside := realDir(t, t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(outside, "x.txt"), []byte("x"), 0o644))

	err := f.adapter.Remove(bg(), f.repo, outside, "pi/inexistente")

	assert.ErrorIs(t, err, werrors.UnsafeWorktreePath(""))
	assert.FileExists(t, filepath.Join(outside, "x.txt"))
}

// Containment: the managed root itself is never a removal candidate.
func TestRemoveRefusesToDeleteTheManagedRootItself(t *testing.T) {
	f := newWorktreeFixture(t)

	err := f.adapter.Remove(bg(), f.repo, f.root, "pi/inexistente")

	assert.ErrorIs(t, err, werrors.UnsafeWorktreePath(""))
	assert.DirExists(t, f.worktree)
}

// Containment: a symbolic link inside the root that leads outside is not followed for deletion.
func TestRemoveRefusesSymlinksThatEscapeTheManagedRoot(t *testing.T) {
	f := newWorktreeFixture(t)
	outside := realDir(t, t.TempDir())
	require.NoError(t, os.WriteFile(filepath.Join(outside, "x.txt"), []byte("x"), 0o644))
	link := filepath.Join(f.root, "escape")
	require.NoError(t, os.Symlink(outside, link))

	err := f.adapter.Remove(bg(), f.repo, link, "pi/inexistente")

	assert.ErrorIs(t, err, werrors.UnsafeWorktreePath(""))
	assert.FileExists(t, filepath.Join(outside, "x.txt"))
}

// Containment: a worktree path that does not come from the root is refused before anything is created.
func TestCreateRefusesPathsOutsideTheRoot(t *testing.T) {
	f := newWorktreeFixture(t)
	escaping := filepath.Join(f.root, "..", "escaped-"+t.Name())

	err := f.adapter.Create(bg(), f.repo, "main", "pi/escape", escaping)

	assert.ErrorIs(t, err, werrors.UnsafeWorktreePath(""))
	assert.NoDirExists(t, filepath.Clean(escaping))
}

// Containment: removal never deletes a branch that is not a task branch, so a corrupt row cannot
// delete the user's branches.
func TestRemoveRefusesToDeleteNonTaskBranches(t *testing.T) {
	f := newWorktreeFixture(t)

	err := f.adapter.Remove(bg(), f.repo, f.worktree, "main")

	assert.Error(t, err)
	assert.NotEmpty(t, gitCmd(t, f.repo, "branch", "--list", "main"))
}

// Argument safety: a base or branch that git would read as an option is refused.
func TestCreateRefusesBaseReferencesThatLookLikeOptions(t *testing.T) {
	f := newWorktreeFixture(t)
	target := f.adapter.WorktreePath("demo", "9999-option")

	err := f.adapter.Create(bg(), f.repo, "--upload-pack=touch /tmp/owned", "pi/opcion", target)

	assert.Error(t, err)
	assert.NoDirExists(t, target)
}

// Cancellation: a context that is already done stops git before it changes anything.
func TestCancelledContextStopsGitBeforeItChangesAnything(t *testing.T) {
	f := newWorktreeFixture(t)
	ctx, cancel := context.WithCancel(bg())
	cancel()

	_, err := f.adapter.Diff(ctx, f.repo, "main", taskBranch)

	assert.ErrorIs(t, err, context.Canceled)
}

// Rust: lists_local_and_remote_branches_without_task_or_head_refs
func TestListsLocalAndRemoteBranchesWithoutTaskOrHeadRefs(t *testing.T) {
	remote := newRemoteFixture(t)
	gitCmd(t, remote.repo, "branch", "pi/otra-tarea-1")
	gitCmd(t, remote.repo, "remote", "set-head", "origin", "main")

	options, err := remote.adapter.ListBranches(bg(), remote.repo)

	require.NoError(t, err)
	var names []string
	var current []string
	for _, option := range options {
		names = append(names, option.Name+"|"+map[bool]string{true: "remote", false: "local"}[option.Remote])
		if option.Current {
			current = append(current, option.Name)
		}
	}
	assert.Equal(t, []string{"main"}, current)
	assert.Equal(t, []string{"main|local", "origin/develop|remote", "origin/main|remote"}, names)
}

// Rust: creates_worktree_from_a_remote_branch
func TestCreatesWorktreeFromARemoteBranch(t *testing.T) {
	remote := newRemoteFixture(t)
	worktree := remote.adapter.WorktreePath("demo", "1234-remote")

	require.NoError(t, remote.adapter.Create(bg(), remote.repo, "origin/develop", taskBranch, worktree))

	assert.FileExists(t, filepath.Join(worktree, "develop.txt"))
	assert.Equal(t, taskBranch, gitCmd(t, worktree, "branch", "--show-current"))
}

// Rust: task_branch_does_not_track_the_remote_base_branch. A push from the task branch must never
// reach the base branch.
func TestTaskBranchDoesNotTrackTheRemoteBaseBranch(t *testing.T) {
	remote := newRemoteFixture(t)
	worktree := remote.adapter.WorktreePath("demo", "1234-remote")
	require.NoError(t, remote.adapter.Create(bg(), remote.repo, "origin/develop", taskBranch, worktree))

	for _, key := range []string{"remote", "merge"} {
		_, err := gitRunExit(t, remote.repo, "config", "--get", "branch."+taskBranch+"."+key)
		assert.Error(t, err, "branch.%s.%s must be unset", taskBranch, key)
	}
}

// Rust: merges_a_task_based_on_a_remote_branch_into_a_local_tracking_branch
func TestMergesATaskBasedOnARemoteBranchIntoALocalTrackingBranch(t *testing.T) {
	remote := newRemoteFixture(t)
	worktree := remote.adapter.WorktreePath("demo", "1234-remote")
	require.NoError(t, remote.adapter.Create(bg(), remote.repo, "origin/develop", taskBranch, worktree))
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "nuevo.txt"), []byte("hola\n"), 0o644))
	_, err := remote.adapter.CommitAll(bg(), worktree, "pi: nuevo")
	require.NoError(t, err)
	remoteTip := gitCmd(t, remote.repo, "rev-parse", "origin/develop")

	outcome, err := remote.adapter.Merge(bg(), remote.repo, "origin/develop", taskBranch, "Merge tarea")

	require.NoError(t, err)
	assert.False(t, outcome.Conflict)
	assert.Equal(t, "hola", gitCmd(t, remote.repo, "show", "develop:nuevo.txt"))
	assert.Equal(t, remoteTip, gitCmd(t, remote.repo, "rev-parse", "origin/develop"), "the remote branch is never touched")
	assert.Equal(t, "origin", gitCmd(t, remote.repo, "config", "branch.develop.remote"))
	assert.Equal(t, "main", gitCmd(t, remote.repo, "branch", "--show-current"))
}

// remoteFixture is a clone of a bare remote with main and develop. The clone has only main checked out.
type remoteFixture struct {
	repo    string
	adapter *git.WorktreeAdapter
}

func newRemoteFixture(t *testing.T) remoteFixture {
	t.Helper()
	repo := newGitRepository(t)
	remote := realDir(t, t.TempDir())
	gitCmd(t, remote, "init", "-q", "--bare")
	gitCmd(t, repo, "remote", "add", "origin", remote)
	gitCmd(t, repo, "checkout", "-q", "-b", "develop")
	require.NoError(t, os.WriteFile(filepath.Join(repo, "develop.txt"), []byte("dev\n"), 0o644))
	gitCmd(t, repo, "add", ".")
	gitCmd(t, repo, "commit", "-q", "-m", "develop")
	gitCmd(t, repo, "push", "-q", "origin", "main", "develop")
	gitCmd(t, repo, "checkout", "-q", "main")
	gitCmd(t, repo, "branch", "-q", "-D", "develop")
	adapter, _ := newAdapter(t)
	return remoteFixture{repo: repo, adapter: adapter}
}

// Rust: remove_parks_worktree_and_purges_it_in_background. The folder and its registration are gone when
// Remove returns; the heavy part (the build folder) is purged in the background, and WaitForCleanup waits
// for that purge.
func TestRemoveParksTheWorktreeAndPurgesItInTheBackground(t *testing.T) {
	f := newWorktreeFixture(t)
	heavy := filepath.Join(f.worktree, "target", "debug")
	require.NoError(t, os.MkdirAll(heavy, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(heavy, "big.bin"), []byte("x"), 0o644))

	require.NoError(t, f.adapter.Remove(bg(), f.repo, f.worktree, taskBranch))

	assert.NoDirExists(t, f.worktree)
	assert.NotContains(t, gitCmd(t, f.repo, "worktree", "list", "--porcelain"), "1234-feature")
	assert.Empty(t, gitCmd(t, f.repo, "branch", "--list", taskBranch))
	f.adapter.WaitForCleanup(bg())
	entries, err := os.ReadDir(filepath.Join(f.root, ".trash"))
	require.NoError(t, err)
	assert.Empty(t, entries)
}
