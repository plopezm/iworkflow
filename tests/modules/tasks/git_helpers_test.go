package tasks_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/git"
)

// The git tests create their own repositories under t.TempDir(). They also isolate git from the user's
// configuration: HOME and the global and system files are pointed away from the real ones. Because
// t.Setenv cannot be combined with t.Parallel, these tests run one after another.

// isolateGit points git at empty configuration for the rest of the test.
func isolateGit(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// realDir returns the folder with its symbolic links resolved. On macOS the temporary folder lives under
// a link, and the containment checks compare resolved paths.
func realDir(t *testing.T, dir string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	return resolved
}

// gitCmd runs git in dir and returns its trimmed output. It fails the test on a non-zero exit.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	require.NoError(t, err, "git %s", strings.Join(args, " "))
	return strings.TrimSpace(string(out))
}

// newGitRepository creates a repository on main with one commit, in a fresh temporary folder.
func newGitRepository(t *testing.T) string {
	t.Helper()
	isolateGit(t)
	dir := realDir(t, t.TempDir())
	gitCmd(t, dir, "init", "-q", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@example.com")
	gitCmd(t, dir, "config", "user.name", "Test")
	gitCmd(t, dir, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("hola\n"), 0o644))
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-q", "-m", "init")
	return dir
}

// newAdapter builds the adapter over a fresh worktrees root and returns both.
func newAdapter(t *testing.T) (*git.WorktreeAdapter, string) {
	t.Helper()
	root := realDir(t, t.TempDir())
	adapter, err := git.NewWorktreeAdapter(root)
	require.NoError(t, err)
	return adapter, root
}

// gitRunExit runs git in dir and returns its trimmed output with the exit error, so a test can assert
// that a command fails.
func gitRunExit(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}
