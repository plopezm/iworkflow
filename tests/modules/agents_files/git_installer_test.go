package agents_files_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/git"
)

// gitRepo creates a repository on branch main with one committed README.md, like the Rust fixture.
func gitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	root := t.TempDir()
	runGitCmd(t, root, "init", "-q", "-b", "main")
	runGitCmd(t, root, "config", "user.email", "test@example.com")
	runGitCmd(t, root, "config", "user.name", "Test")
	runGitCmd(t, root, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hola\n"), 0o644))
	runGitCmd(t, root, "add", ".")
	runGitCmd(t, root, "commit", "-q", "-m", "init")
	return root
}

// runGitCmd runs git in root and returns its trimmed stdout; any failure fails the test.
func runGitCmd(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(context.Background(), "git", append([]string{"-C", root}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Run(), "git %v: %s", args, stderr.String())
	return strings.TrimSpace(stdout.String())
}

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newInstaller() *git.AgentsFileInstaller {
	return git.NewAgentsFileInstaller(quietLogger())
}

func TestGitInstallerCommitsOnlyAgentsMdAndLeavesOtherChangesUntouched(t *testing.T) {
	root := gitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("modificado\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("sin seguimiento\n"), 0o644))

	outcome, err := newInstaller().Install(context.Background(), []byte("# reglas\n"), root, false)

	require.NoError(t, err)
	require.True(t, outcome.Committed)
	content, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	require.NoError(t, err)
	require.Equal(t, "# reglas\n", string(content))
	require.Equal(t, "docs: add AGENTS.md", runGitCmd(t, root, "log", "-1", "--format=%s"))
	require.Equal(t, "AGENTS.md", runGitCmd(t, root, "show", "--name-only", "--format=", "HEAD"))
	status := runGitCmd(t, root, "status", "--porcelain")
	require.Contains(t, status, "M README.md")
	require.Contains(t, status, "?? notes.txt")
	require.NotContains(t, status, "AGENTS.md")
}

func TestGitInstallerRejectsAnExistingFileWithoutOverwriteAndKeepsIt(t *testing.T) {
	root := gitRepo(t)
	target := filepath.Join(root, "AGENTS.md")
	require.NoError(t, os.WriteFile(target, []byte("original\n"), 0o644))

	_, err := newInstaller().Install(context.Background(), []byte("nuevo\n"), root, false)

	require.Equal(t, agentserrors.TargetExists(target), err)
	content, readErr := os.ReadFile(target)
	require.NoError(t, readErr)
	require.Equal(t, "original\n", string(content))
}

func TestGitInstallerOverwriteReplacesAndCommitsTheNewContent(t *testing.T) {
	ctx := context.Background()
	root := gitRepo(t)
	installer := newInstaller()
	_, err := installer.Install(ctx, []byte("original\n"), root, false)
	require.NoError(t, err)

	outcome, err := installer.Install(ctx, []byte("nuevo\n"), root, true)

	require.NoError(t, err)
	require.True(t, outcome.Committed)
	require.Equal(t, "nuevo", runGitCmd(t, root, "show", "HEAD:AGENTS.md"))
	require.Empty(t, runGitCmd(t, root, "status", "--porcelain"))
}

func TestGitInstallerOverwriteWithIdenticalContentIsNotCommittedAgain(t *testing.T) {
	ctx := context.Background()
	root := gitRepo(t)
	installer := newInstaller()
	_, err := installer.Install(ctx, []byte("igual\n"), root, false)
	require.NoError(t, err)
	before := runGitCmd(t, root, "rev-parse", "HEAD")

	outcome, err := installer.Install(ctx, []byte("igual\n"), root, true)

	require.NoError(t, err)
	require.False(t, outcome.Committed)
	require.Equal(t, before, runGitCmd(t, root, "rev-parse", "HEAD"))
}

func TestGitInstallerWritesWithoutCommittingOutsideARepository(t *testing.T) {
	dir := t.TempDir()

	outcome, err := newInstaller().Install(context.Background(), []byte("# reglas\n"), dir, false)

	require.NoError(t, err)
	require.False(t, outcome.Committed)
	content, readErr := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	require.NoError(t, readErr)
	require.Equal(t, "# reglas\n", string(content))
}

func TestGitInstallerReportsAMissingDirectoryAsInstallError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope")

	_, err := newInstaller().Install(context.Background(), []byte("x"), missing, false)

	require.Equal(t, agentserrors.Install("la carpeta no existe: "+missing), err)
}
