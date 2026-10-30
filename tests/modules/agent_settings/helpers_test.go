package agent_settings_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

// runGit runs git in dir and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

// gitRepo creates a repository with one commit in a temporary folder and returns its root.
func gitRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("hola\n"), 0o644))
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-q", "-m", "init")
	return root
}

// profileOf parses a valid profile name for a test.
func profileOf(t *testing.T, raw string) *models.ProfileName {
	t.Helper()
	name, err := models.ParseProfileName(raw)
	require.NoError(t, err)
	return &name
}
