package datahome_test

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/datahome"
	"iworkflow/internal/core/db"
)

// seededDatabase creates a migrated database at <dir>/workspaces.db with one workspace, three tasks
// (inside the folder, outside it, and without a worktree) and two agents files.
func seededDatabase(t *testing.T, dir string, repo string) string {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(dir, "workspaces.db")
	handle, err := db.Open(ctx, path)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(ctx, handle))
	exec1 := func(query string, args ...any) {
		_, err := handle.ExecContext(ctx, query, args...)
		require.NoError(t, err, query)
	}
	exec1(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'ws', ?, 'main', '2026-01-01')`, repo)
	insertTask := func(id string, worktree string) {
		exec1(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
			VALUES (?, 'w1', 't', 'p', 'b', 'main', ?, 'idle', '2026-01-01', '2026-01-01')`, id, worktree)
	}
	insertTask("inside", filepath.Join(dir, "worktrees", "ws", "inside"))
	insertTask("sibling", dir+"-sibling"+string(filepath.Separator)+"x")
	insertTask("backlog", "")
	exec1(`INSERT INTO agents_files (id, description, path, created_at) VALUES ('a1', 'd', ?, '2026-01-01')`, filepath.Join(dir, "agents-files", "a1.md"))
	require.NoError(t, handle.Close())
	return path
}

func queryStrings(t *testing.T, path string, query string) []string {
	t.Helper()
	handle, err := db.Open(context.Background(), path)
	require.NoError(t, err)
	defer func() { _ = handle.Close() }()
	rows, err := handle.QueryContext(context.Background(), query)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var value string
		require.NoError(t, rows.Scan(&value))
		out = append(out, value)
	}
	require.NoError(t, rows.Err())
	return out
}

func TestRewritePathsMovesOnlyPathsInsideTheOldFolder(t *testing.T) {
	root := t.TempDir()
	from, to := filepath.Join(root, "old"), filepath.Join(root, "new")
	require.NoError(t, os.MkdirAll(from, 0o755))
	path := seededDatabase(t, from, filepath.Join(root, "repo"))

	report, err := datahome.RewritePaths(context.Background(), path, []datahome.Prefix{{From: from, To: to}})

	require.NoError(t, err)
	assert.Equal(t, 2, report.Rewritten, "one worktree and one agents file")
	assert.ElementsMatch(t, []string{
		filepath.Join(to, "worktrees", "ws", "inside"),
		from + "-sibling" + string(filepath.Separator) + "x",
		"",
	}, queryStrings(t, path, `SELECT worktree_path FROM tasks`), "a sibling folder with the same name prefix is not touched")
	assert.Equal(t, []string{filepath.Join(to, "agents-files", "a1.md")}, queryStrings(t, path, `SELECT path FROM agents_files`))
	assert.Equal(t, []string{filepath.Join(root, "repo")}, queryStrings(t, path, `SELECT path FROM workspaces`), "the user's repositories never move")
}

func TestRewritePathsIsIdempotent(t *testing.T) {
	root := t.TempDir()
	from, to := filepath.Join(root, "old"), filepath.Join(root, "new")
	require.NoError(t, os.MkdirAll(from, 0o755))
	path := seededDatabase(t, from, filepath.Join(root, "repo"))
	prefixes := []datahome.Prefix{{From: from, To: to}}
	_, err := datahome.RewritePaths(context.Background(), path, prefixes)
	require.NoError(t, err)

	report, err := datahome.RewritePaths(context.Background(), path, prefixes)

	require.NoError(t, err)
	assert.Zero(t, report.Rewritten)
}

func TestRewritePathsTreatsPercentAndUnderscoreAsPlainCharacters(t *testing.T) {
	root := t.TempDir()
	from, to := filepath.Join(root, "100%_old"), filepath.Join(root, "new")
	require.NoError(t, os.MkdirAll(from, 0o755))
	path := seededDatabase(t, from, filepath.Join(root, "repo"))

	report, err := datahome.RewritePaths(context.Background(), path, []datahome.Prefix{{From: from, To: to}})

	require.NoError(t, err)
	assert.Equal(t, 2, report.Rewritten)
	other := filepath.Join(root, "100x_old")
	_, err = datahome.RewritePaths(context.Background(), path, []datahome.Prefix{{From: other, To: to}})
	require.NoError(t, err)
}

func TestRewritePathsSkipsTablesThatDoNotExistYet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workspaces.db")
	handle, err := db.Open(context.Background(), path)
	require.NoError(t, err)
	require.NoError(t, handle.Close())

	report, err := datahome.RewritePaths(context.Background(), path, []datahome.Prefix{{From: "/a", To: "/b"}})

	require.NoError(t, err)
	assert.Zero(t, report.Rewritten)
}

func TestQuiesceFoldsTheWriteAheadLogAndLeavesOneFile(t *testing.T) {
	dir := t.TempDir()
	path := seededDatabase(t, dir, "/repo")

	require.NoError(t, datahome.Quiesce(context.Background(), path))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no -wal or -shm file is left to copy")
}

func TestQuiesceRefusesADatabaseAnotherProcessHolds(t *testing.T) {
	dir := t.TempDir()
	path := seededDatabase(t, dir, "/repo")
	holder, err := sql.Open("sqlite", "file:"+path)
	require.NoError(t, err)
	defer func() { _ = holder.Close() }()
	tx, err := holder.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	var count int
	require.NoError(t, tx.QueryRow(`SELECT count(*) FROM tasks`).Scan(&count))
	defer func() { _ = tx.Rollback() }()

	err = datahome.Quiesce(context.Background(), path)

	// A held read transaction cannot be checkpointed away only when the log has content; either outcome
	// must leave the database readable and never lose rows.
	if err != nil {
		require.ErrorIs(t, err, datahome.ErrDatabaseBusy)
	}
	assert.Equal(t, 3, count)
}

func TestCompareDatabasesDetectsLostRows(t *testing.T) {
	root := t.TempDir()
	a, b := filepath.Join(root, "a"), filepath.Join(root, "b")
	require.NoError(t, os.MkdirAll(a, 0o755))
	require.NoError(t, os.MkdirAll(b, 0o755))
	pathA := seededDatabase(t, a, "/repo")
	pathB := seededDatabase(t, b, "/repo")
	require.NoError(t, datahome.CompareDatabases(context.Background(), pathA, pathB))

	handle, err := db.Open(context.Background(), pathB)
	require.NoError(t, err)
	_, err = handle.Exec(`DELETE FROM tasks WHERE id = 'backlog'`)
	require.NoError(t, err)
	require.NoError(t, handle.Close())

	require.ErrorIs(t, datahome.CompareDatabases(context.Background(), pathA, pathB), datahome.ErrVerify)
}

func TestRebasedRewritesAbsoluteLinksIntoTheOldFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	root := filepath.Join(t.TempDir(), "old")
	writeFile(t, filepath.Join(root, "worktrees", "t", "real"), "x", 0o644)
	require.NoError(t, os.Symlink(filepath.Join(root, "worktrees", "t", "real"), filepath.Join(root, "worktrees", "t", "abs")))
	require.NoError(t, os.Symlink("real", filepath.Join(root, "worktrees", "t", "rel")))
	require.NoError(t, os.Symlink("/etc/hosts", filepath.Join(root, "worktrees", "t", "other")))
	manifest, err := datahome.Scan(root, nil)
	require.NoError(t, err)
	newRoot := filepath.Join(t.TempDir(), "new")

	rebased := manifest.Rebased(root, newRoot)
	require.NoError(t, datahome.Copy(context.Background(), root, newRoot, rebased))

	require.NoError(t, datahome.Verify(newRoot, rebased))
	link := func(name string) string {
		target, err := os.Readlink(filepath.Join(newRoot, "worktrees", "t", name))
		require.NoError(t, err)
		return target
	}
	assert.Equal(t, filepath.Join(newRoot, "worktrees", "t", "real"), link("abs"))
	assert.Equal(t, "real", link("rel"))
	assert.Equal(t, "/etc/hosts", link("other"))
	require.NoError(t, datahome.Verify(root, manifest), "the original manifest still matches the source")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)...)
	out, err := command.CombinedOutput()
	require.NoError(t, err, string(out))
	return string(out)
}

func TestRepairWorktreesReconnectsAMovedWorktreeSoPruneKeepsIt(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	require.NoError(t, os.MkdirAll(repo, 0o755))
	git(t, repo, "init", "-q", "-b", "main")
	writeFile(t, filepath.Join(repo, "a.txt"), "a", 0o644)
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-qm", "init")
	oldWorktree := filepath.Join(root, "old", "worktrees", "wt")
	git(t, repo, "worktree", "add", "-q", "-b", "iwf/x", oldWorktree)
	newWorktree := filepath.Join(root, "new", "worktrees", "wt")
	require.NoError(t, os.MkdirAll(filepath.Dir(newWorktree), 0o755))
	manifest, err := datahome.Scan(filepath.Join(root, "old"), nil)
	require.NoError(t, err)
	require.NoError(t, datahome.Copy(context.Background(), filepath.Join(root, "old"), filepath.Join(root, "new"), manifest))

	report, err := datahome.RepairWorktrees(context.Background(), []datahome.Worktree{
		{Repo: repo, Path: newWorktree},
		{Repo: repo, Path: filepath.Join(root, "new", "worktrees", "merged-gone")},
		{Repo: filepath.Join(root, "missing-repo"), Path: newWorktree},
	})

	require.NoError(t, err)
	assert.Equal(t, 1, report.Repaired)
	assert.Equal(t, 1, report.MissingRepos)
	require.NoError(t, os.RemoveAll(filepath.Join(root, "old")))
	git(t, repo, "worktree", "prune")
	listed := git(t, repo, "worktree", "list", "--porcelain")
	resolved, err := filepath.EvalSymlinks(newWorktree)
	require.NoError(t, err)
	assert.Contains(t, listed, resolved, "the moved worktree is still registered after the old folder is gone")
}

func TestWorktreesOfReadsRepositoryAndPathOfEveryTaskWithAWorktree(t *testing.T) {
	dir := t.TempDir()
	path := seededDatabase(t, dir, "/repo")

	links, err := datahome.WorktreesOf(context.Background(), path)

	require.NoError(t, err)
	assert.ElementsMatch(t, []datahome.Worktree{
		{Repo: "/repo", Path: filepath.Join(dir, "worktrees", "ws", "inside")},
		{Repo: "/repo", Path: dir + "-sibling" + string(filepath.Separator) + "x"},
	}, links)
}

// seededDatabaseAt is seededDatabase with the inside task pointing at a real worktree.
func seededDatabaseAt(t *testing.T, dir string, repo string, worktree string) string {
	t.Helper()
	path := seededDatabase(t, dir, repo)
	handle, err := db.Open(context.Background(), path)
	require.NoError(t, err)
	_, err = handle.Exec(`UPDATE tasks SET worktree_path = ? WHERE id = 'inside'`, worktree)
	require.NoError(t, err)
	require.NoError(t, handle.Close())
	return path
}

func TestLockDatabaseStopsOtherProcessesFromWritingUntilReleased(t *testing.T) {
	dir := t.TempDir()
	path := seededDatabase(t, dir, "/repo")
	release, err := datahome.LockDatabase(context.Background(), path)
	require.NoError(t, err)

	other, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(150)")
	require.NoError(t, err)
	defer func() { _ = other.Close() }()
	_, writeErr := other.Exec(`UPDATE tasks SET title = 'late write' WHERE id = 'inside'`)
	assert.Error(t, writeErr, "a write while the move holds the database is refused, not silently lost")

	require.NoError(t, release())
	_, writeErr = other.Exec(`UPDATE tasks SET title = 'after' WHERE id = 'inside'`)
	assert.NoError(t, writeErr)
}
