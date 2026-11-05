package datahome_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/datahome"
)

type moveFixture struct {
	root     string
	from, to string
	repo     string
	worktree string
	files    datahome.Files
	mover    *datahome.Mover
}

// newMoveFixture builds a data folder with a database, one real git worktree registered in a repository
// outside it, an attachment and a pending-free configuration folder.
func newMoveFixture(t *testing.T) *moveFixture {
	t.Helper()
	root := t.TempDir()
	f := &moveFixture{root: root, from: filepath.Join(root, "old"), to: filepath.Join(root, "new", "iwf"), repo: filepath.Join(root, "repo")}
	require.NoError(t, os.MkdirAll(f.repo, 0o755))
	git(t, f.repo, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(f.repo, "a.txt"), "a", 0o644)
	git(t, f.repo, "add", ".")
	git(t, f.repo, "commit", "-qm", "init")
	f.worktree = filepath.Join(f.from, "worktrees", "ws", "inside")
	require.NoError(t, os.MkdirAll(filepath.Dir(f.worktree), 0o755))
	git(t, f.repo, "worktree", "add", "-q", "-b", "iwf/x", f.worktree)
	writeFile(t, filepath.Join(f.worktree, "work.txt"), "uncommitted work", 0o644)
	writeFile(t, filepath.Join(f.from, "attachments", "t", "a.png"), "png", 0o644)
	writeFile(t, filepath.Join(f.from, "worktrees", ".trash", "junk", "x"), "x", 0o644)
	seededDatabaseAt(t, f.from, f.repo, f.worktree)
	f.files = datahome.FilesFor(filepath.Join(root, "cfg", "iworkflow", "data-home"))
	f.mover = &datahome.Mover{Files: f.files, Now: func() time.Time { return time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC) }, Log: func(string, ...any) {}}
	return f
}

func (f *moveFixture) pointer(t *testing.T) string {
	t.Helper()
	dir, err := datahome.Read(f.files.Pointer)
	require.NoError(t, err)
	return dir
}

func (f *moveFixture) request(t *testing.T) {
	t.Helper()
	require.NoError(t, f.mover.Request(f.from, f.to))
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func TestApplyWithoutAPendingMoveDoesNothing(t *testing.T) {
	f := newMoveFixture(t)

	result, err := f.mover.Apply(context.Background(), f.from)

	require.NoError(t, err)
	assert.Nil(t, result)
	assert.Empty(t, f.pointer(t))
}

func TestMovingCopiesSwitchesThePointerAndCleansTheOldFolder(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)

	result, err := f.mover.Apply(context.Background(), f.from)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.OK, result.Error)
	assert.Equal(t, f.to, f.pointer(t), "the pointer names the new folder")
	assert.False(t, exists(f.from), "the old folder is gone")
	assert.False(t, exists(f.to+".iwf-moving"), "no staging folder is left")
	assert.False(t, exists(filepath.Join(f.to, ".iwf-move-complete")), "the completion marker is removed")
	assert.False(t, exists(filepath.Join(f.to, "worktrees", ".trash")), "pending trash is not carried over")
	content, err := os.ReadFile(filepath.Join(f.to, "worktrees", "ws", "inside", "work.txt"))
	require.NoError(t, err)
	assert.Equal(t, "uncommitted work", string(content), "uncommitted work inside a worktree survives")
	assert.True(t, exists(filepath.Join(f.to, "attachments", "t", "a.png")))
	assert.Equal(t, []string{filepath.Join(f.to, "worktrees", "ws", "inside")}, queryStrings(t, filepath.Join(f.to, "workspaces.db"), `SELECT worktree_path FROM tasks WHERE id = 'inside'`))
	pending, err := f.mover.Pending()
	require.NoError(t, err)
	assert.Nil(t, pending, "the pending marker is consumed")
	saved, err := f.mover.LastResult()
	require.NoError(t, err)
	assert.Equal(t, result, saved, "the outcome is kept for the settings screen")
}

func TestAfterMovingGitStillKnowsTheWorktreeAndPruneKeepsIt(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)
	_, err := f.mover.Apply(context.Background(), f.from)
	require.NoError(t, err)

	git(t, f.repo, "worktree", "prune")
	listed := git(t, f.repo, "worktree", "list", "--porcelain")

	resolved, err := filepath.EvalSymlinks(filepath.Join(f.to, "worktrees", "ws", "inside"))
	require.NoError(t, err)
	assert.Contains(t, listed, resolved)
	assert.Equal(t, "uncommitted work", func() string {
		out := git(t, filepath.Join(f.to, "worktrees", "ws", "inside"), "status", "--porcelain")
		return map[bool]string{true: "uncommitted work", false: out}[len(out) > 0]
	}(), "git works inside the moved worktree")
}

func TestAFailedMoveLeavesTheSourceUntouchedAndDropsThePending(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)
	writeFile(t, filepath.Join(f.to, "someone-else.txt"), "x", 0o644) // appears after the request

	result, err := f.mover.Apply(context.Background(), f.from)

	require.NoError(t, err, "a failed move is reported in the result, not as a start-up failure")
	require.NotNil(t, result)
	assert.False(t, result.OK)
	assert.Contains(t, result.Error, "empty")
	assert.Empty(t, f.pointer(t), "the pointer did not move")
	assert.True(t, exists(filepath.Join(f.from, "workspaces.db")))
	assert.True(t, exists(filepath.Join(f.worktree, "work.txt")))
	assert.True(t, exists(filepath.Join(f.to, "someone-else.txt")), "a folder that is not ours is never cleaned")
	pending, err := f.mover.Pending()
	require.NoError(t, err)
	assert.Nil(t, pending, "a failed move is not retried at every start")
}

func TestACancelledMoveKeepsThePendingAndLeavesNoPartialCopy(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := f.mover.Apply(ctx, f.from)

	require.ErrorIs(t, err, context.Canceled)
	pending, perr := f.mover.Pending()
	require.NoError(t, perr)
	assert.NotNil(t, pending, "the next start tries again")
	assert.False(t, exists(f.to+".iwf-moving"))
	assert.False(t, exists(f.to))
	assert.True(t, exists(filepath.Join(f.from, "workspaces.db")))
	assert.Empty(t, f.pointer(t))
}

func TestAStaleStagingFolderFromACrashIsReplaced(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)
	writeFile(t, filepath.Join(f.to+".iwf-moving", "half-copied"), "x", 0o644)

	result, err := f.mover.Apply(context.Background(), f.from)

	require.NoError(t, err)
	assert.True(t, result.OK, result.Error)
	assert.False(t, exists(filepath.Join(f.to, "half-copied")))
}

func TestResumeAfterACrashBetweenTheRenameAndThePointer(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)
	// Reproduce the crash point: the staging copy was complete and renamed, nothing else happened.
	crashed := &datahome.Mover{Files: f.files, Now: f.mover.Now, Log: f.mover.Log, StopAfter: datahome.StepRenamed}
	_, err := crashed.Apply(context.Background(), f.from)
	require.ErrorIs(t, err, datahome.ErrStoppedForTest)
	require.Empty(t, f.pointer(t))
	require.True(t, exists(filepath.Join(f.to, ".iwf-move-complete")))
	require.True(t, exists(filepath.Join(f.from, "workspaces.db")))

	result, err := f.mover.Apply(context.Background(), f.from)

	require.NoError(t, err)
	assert.True(t, result.OK, result.Error)
	assert.Equal(t, f.to, f.pointer(t))
	assert.False(t, exists(f.from))
	assert.False(t, exists(filepath.Join(f.to, ".iwf-move-complete")))
}

func TestResumeAfterACrashBetweenThePointerAndTheCleanup(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)
	crashed := &datahome.Mover{Files: f.files, Now: f.mover.Now, Log: f.mover.Log, StopAfter: datahome.StepSwitched}
	_, err := crashed.Apply(context.Background(), f.from)
	require.ErrorIs(t, err, datahome.ErrStoppedForTest)
	require.Equal(t, f.to, f.pointer(t))
	writeFile(t, filepath.Join(f.from, "only-in-old.txt"), "never copied", 0o644)

	result, err := f.mover.Apply(context.Background(), f.to) // the process now resolves the new folder

	require.NoError(t, err)
	assert.True(t, result.OK, result.Error)
	assert.True(t, exists(filepath.Join(f.from, "only-in-old.txt")), "a file the copy never held is not deleted")
	assert.False(t, exists(filepath.Join(f.from, "workspaces.db")))
	assert.NotEmpty(t, result.Warnings, "the leftover is reported")
	assert.True(t, exists(filepath.Join(f.to, "workspaces.db")))
}

func TestApplyRefusesWhenTheDataFolderChangedSinceTheRequest(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)

	result, err := f.mover.Apply(context.Background(), filepath.Join(f.root, "elsewhere"))

	require.NoError(t, err)
	assert.False(t, result.OK)
	assert.True(t, exists(filepath.Join(f.from, "workspaces.db")))
	assert.False(t, exists(f.to))
	assert.Empty(t, f.pointer(t))
}

func TestRequestValidatesAndNeverOverwritesAPendingMove(t *testing.T) {
	f := newMoveFixture(t)

	require.ErrorIs(t, f.mover.Request(f.from, "relative"), datahome.ErrNotAbsolute)
	pending, err := f.mover.Pending()
	require.NoError(t, err)
	assert.Nil(t, pending, "a rejected request writes nothing")

	f.request(t)
	require.ErrorIs(t, f.mover.Request(f.from, filepath.Join(f.root, "other")), datahome.ErrPending)
	pending, err = f.mover.Pending()
	require.NoError(t, err)
	assert.Equal(t, f.to, pending.To)
}

func TestCancelRemovesThePendingMove(t *testing.T) {
	f := newMoveFixture(t)
	f.request(t)

	require.NoError(t, f.mover.Cancel())

	pending, err := f.mover.Pending()
	require.NoError(t, err)
	assert.Nil(t, pending)
	require.NoError(t, f.mover.Cancel(), "cancelling nothing is fine")
}
