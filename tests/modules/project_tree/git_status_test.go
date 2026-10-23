package projecttree_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/outbound/git"
)

// gitIn runs git in dir with a test identity and fails the test on error.
func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)
	output, err := exec.CommandContext(context.Background(), "git", full...).CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, output)
}

// gitOut runs git in dir and returns its trimmed stdout.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	full := append([]string{"-C", dir}, args...)
	output, err := exec.CommandContext(context.Background(), "git", full...).Output()
	require.NoError(t, err, "git %v", args)
	return strings.TrimSpace(string(output))
}

// newRepo builds a repository with two committed files: kept.txt and edited.txt.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// The adapter commits without its own identity, so the environment provides one for the test.
	t.Setenv("GIT_AUTHOR_NAME", "t")
	t.Setenv("GIT_AUTHOR_EMAIL", "t@t")
	t.Setenv("GIT_COMMITTER_NAME", "t")
	t.Setenv("GIT_COMMITTER_EMAIL", "t@t")
	gitIn(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("one\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nb\n"), 0o600))
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// sortedChanges returns "path:kind" pairs sorted by path, which is easy to compare.
func sortedChanges(changes []models.GitChange) []string {
	pairs := make([]string, 0, len(changes))
	for _, change := range changes {
		pairs = append(pairs, change.Path+":"+strconv.Itoa(int(change.Kind)))
	}
	sort.Strings(pairs)
	return pairs
}

// pair builds a "path:kind" entry for sortedChanges.
func pair(path string, kind models.GitChangeKind) string {
	return path + ":" + strconv.Itoa(int(kind))
}

// flagsOf returns the staged and unstaged flags of path.
func flagsOf(t *testing.T, changes []models.GitChange, path string) (bool, bool) {
	t.Helper()
	for _, change := range changes {
		if change.Path == path {
			return change.Staged, change.Unstaged
		}
	}
	require.Failf(t, "missing change", "no change for %s", path)
	return false, false
}

// cachedNames returns the staged paths, one per line.
func cachedNames(t *testing.T, dir string) string {
	t.Helper()
	return gitOut(t, dir, "diff", "--cached", "--name-only")
}

func TestReportsModifiedAddedAndUntrackedFiles(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("x\n"), 0o600))
	gitIn(t, dir, "add", "staged.txt")
	require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "new.txt"), []byte("y\n"), 0o600))

	changes, err := git.NewStatusAdapter().Changes(context.Background(), dir)

	require.NoError(t, err)
	require.Equal(t, []string{
		pair("edited.txt", models.GitChangeModified),
		pair("staged.txt", models.GitChangeAdded),
		pair("sub/new.txt", models.GitChangeUntracked),
	}, sortedChanges(changes))
}

func TestPathsAreRelativeToASubdirectoryRoot(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "app"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app", "a.txt"), []byte("x\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "outside.txt"), []byte("x\n"), 0o600))

	changes, err := git.NewStatusAdapter().Changes(context.Background(), filepath.Join(dir, "app"))

	require.NoError(t, err)
	require.Equal(t, []string{pair("a.txt", models.GitChangeUntracked)}, sortedChanges(changes))
}

func TestNotARepositoryHasNoChanges(t *testing.T) {
	dir := t.TempDir()

	changes, err := git.NewStatusAdapter().Changes(context.Background(), dir)

	require.NoError(t, err)
	require.Empty(t, changes)
}

func TestBaseContentIsTheHeadVersionOrNil(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("changed\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fresh.txt"), []byte("new\n"), 0o600))
	adapter := git.NewStatusAdapter()

	base, err := adapter.BaseContent(context.Background(), dir, "edited.txt")
	require.NoError(t, err)
	require.NotNil(t, base)
	require.Equal(t, "a\nb\n", *base)

	base, err = adapter.BaseContent(context.Background(), dir, "fresh.txt")
	require.NoError(t, err)
	require.Nil(t, base)

	_, err = adapter.BaseContent(context.Background(), dir, "../x")
	require.Equal(t, treeerrors.InvalidPath("../x"), err)
}

func TestStageAddsOnlyTheGivenPathsAndUnstageKeepsTheContent(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fresh.txt"), []byte("new\n"), 0o600))
	adapter := git.NewStatusAdapter()
	ctx := context.Background()

	require.NoError(t, adapter.Stage(ctx, dir, []string{"edited.txt"}))
	require.Equal(t, "edited.txt", cachedNames(t, dir))
	changes, err := adapter.Changes(ctx, dir)
	require.NoError(t, err)
	staged, unstaged := flagsOf(t, changes, "edited.txt")
	require.True(t, staged)
	require.False(t, unstaged)
	staged, unstaged = flagsOf(t, changes, "fresh.txt")
	require.False(t, staged)
	require.True(t, unstaged)

	require.NoError(t, adapter.Unstage(ctx, dir, []string{"edited.txt"}))
	require.Empty(t, cachedNames(t, dir))
	content, err := os.ReadFile(filepath.Join(dir, "edited.txt"))
	require.NoError(t, err)
	require.Equal(t, "a\nB\n", string(content))
}

func TestEmptyPathListStagesAndUnstagesEverything(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fresh.txt"), []byte("new\n"), 0o600))
	adapter := git.NewStatusAdapter()
	ctx := context.Background()

	require.NoError(t, adapter.Stage(ctx, dir, nil))
	require.Equal(t, "edited.txt\nfresh.txt", cachedNames(t, dir))

	require.NoError(t, adapter.Unstage(ctx, dir, nil))
	require.Empty(t, cachedNames(t, dir))
	require.FileExists(t, filepath.Join(dir, "fresh.txt"))
}

func TestPartiallyStagedFileIsBothStagedAndUnstaged(t *testing.T) {
	dir := newRepo(t)
	adapter := git.NewStatusAdapter()
	ctx := context.Background()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\n"), 0o600))
	require.NoError(t, adapter.Stage(ctx, dir, []string{"edited.txt"}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\nC\n"), 0o600))

	changes, err := adapter.Changes(ctx, dir)

	require.NoError(t, err)
	staged, unstaged := flagsOf(t, changes, "edited.txt")
	require.True(t, staged)
	require.True(t, unstaged)
}

func TestUnstageWorksBeforeTheFirstCommit(t *testing.T) {
	dir := t.TempDir()
	gitIn(t, dir, "init", "-q")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x\n"), 0o600))
	adapter := git.NewStatusAdapter()
	ctx := context.Background()
	require.NoError(t, adapter.Stage(ctx, dir, nil))

	require.NoError(t, adapter.Unstage(ctx, dir, nil))

	changes, err := adapter.Changes(ctx, dir)
	require.NoError(t, err)
	staged, unstaged := flagsOf(t, changes, "a.txt")
	require.False(t, staged)
	require.True(t, unstaged)
}

func TestSpecialAndEscapingPathsAreNeverTreatedAsPathspecMagic(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\n"), 0o600))
	adapter := git.NewStatusAdapter()
	ctx := context.Background()

	require.Error(t, adapter.Stage(ctx, dir, []string{"../x"}))
	require.Error(t, adapter.Stage(ctx, dir, []string{""}))
	// A glob is a literal file name here: it matches nothing, so the call may fail, but it must not
	// stage edited.txt.
	_ = adapter.Stage(ctx, dir, []string{"*.txt"})
	require.Empty(t, cachedNames(t, dir))
}

func TestCommitRecordsOnlyTheStagedChanges(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"), []byte("z\n"), 0o600))
	gitIn(t, dir, "add", "edited.txt")
	before, err := strconv.Atoi(gitOut(t, dir, "rev-list", "--count", "HEAD"))
	require.NoError(t, err)
	ctx := context.Background()

	require.NoError(t, git.NewStatusAdapter().Commit(ctx, dir, "feat: edit"))

	changes, err := git.NewStatusAdapter().Changes(ctx, dir)
	require.NoError(t, err)
	require.Equal(t, []string{pair("other.txt", models.GitChangeUntracked)}, sortedChanges(changes))
	require.Equal(t, "feat: edit", gitOut(t, dir, "log", "-1", "--format=%s"))
	after, err := strconv.Atoi(gitOut(t, dir, "rev-list", "--count", "HEAD"))
	require.NoError(t, err)
	require.Equal(t, before+1, after)
	require.Equal(t, "edited.txt", gitOut(t, dir, "show", "--name-only", "--format=", "HEAD"))
}

func TestCommitWithoutStagedChangesFails(t *testing.T) {
	dir := newRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "edited.txt"), []byte("a\nB\n"), 0o600))

	err := git.NewStatusAdapter().Commit(context.Background(), dir, "nothing")

	// The wording is git's (and locale dependent): only its presence is pinned.
	var failure treeerrors.Error
	require.ErrorAs(t, err, &failure)
	require.ErrorIs(t, err, treeerrors.CommitFailed(""))
	require.Greater(t, len(failure.Error()), len("no se pudo hacer commit: "))
	require.Equal(t, "1", gitOut(t, dir, "rev-list", "--count", "HEAD"))
}
