package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expiredContext returns a context whose deadline has already passed, so its Err is context.DeadlineExceeded.
func expiredContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	t.Cleanup(cancel)
	require.Error(t, ctx.Err())
	return ctx
}

func TestOutcomeOfASuccessfulCommandIsSuccessEvenIfTheContextExpiredJustAfter(t *testing.T) {
	out, err := outcome(expiredContext(t), nil, "main\n", "", []string{"branch", "--show-current"})

	require.NoError(t, err)
	assert.Equal(t, "main", out)
}

func TestOutcomeOfAFailedCommandReportsTheContextWhenItEnded(t *testing.T) {
	_, err := outcome(expiredContext(t), &exec.ExitError{}, "", "boom", []string{"remote"})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestOutcomeOfAFailedCommandCarriesGitsStderr(t *testing.T) {
	_, err := outcome(context.Background(), &exec.ExitError{}, "", "fatal: not a git repository\n", []string{"remote"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fatal: not a git repository")
}

func TestListsRefSplitsOnTheTabSoUnusualNamesMatchExactly(t *testing.T) {
	// U+00A0 is legal in a git ref name and is a space for strings.Fields, which used to split the name.
	ref := "refs/heads/feat x"
	listed := "3f2a9c1\trefs/heads/feat x\n"

	assert.True(t, listsRef(listed, ref))
	assert.False(t, listsRef("3f2a9c1\trefs/heads/feat\n", ref), "a prefix of the ref is not the ref")
	assert.False(t, listsRef("3f2a9c1\trefs/heads/feat xy\n", ref), "a longer name is not the ref")
}

func TestListsRefIgnoresLinesWithoutATab(t *testing.T) {
	assert.False(t, listsRef("refs/heads/main\n", "refs/heads/main"))
}

// installSlowGit puts a `git` shim first on PATH. Every call sleeps for delay and then runs the real git, so the
// timing of each command is controlled without touching the adapter.
func installSlowGit(t *testing.T, delay string) {
	t.Helper()
	realGit, err := exec.LookPath("git")
	require.NoError(t, err)
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep \"$IWF_TEST_GIT_SLEEP\"\nexec \"$IWF_TEST_REAL_GIT\" \"$@\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("IWF_TEST_REAL_GIT", realGit)
	t.Setenv("IWF_TEST_GIT_SLEEP", delay)
}

func initRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	return repo
}

func TestListBranchesGivesEachGitCommandItsOwnBudget(t *testing.T) {
	repo := initRepo(t)
	// Four git commands at 0.4s each take 1.6s in total: longer than the 1s budget, but each one fits in it.
	installSlowGit(t, "0.4")

	branches, err := listBranches(context.Background(), repo, time.Second)

	require.NoError(t, err)
	require.Len(t, branches, 1)
	assert.Equal(t, "main", branches[0].Name)
}
