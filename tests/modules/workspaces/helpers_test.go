package workspaces_test

import (
	"bytes"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/workspaces/domain/models"
)

// insertTaskReferencing inserts a minimal task row that points at the workspace. It is used only to
// exercise the foreign key that protects a workspace from deletion while tasks still reference it.
func insertTaskReferencing(t *testing.T, handle *sql.DB, workspaceID models.WorkspaceID) {
	t.Helper()
	const insertTask = `INSERT INTO tasks
		(id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES (?, ?, 'task', 'prompt', 'branch', 'main', '/tmp/worktree', 'queued', ?, ?)`
	taskID, err := models.NewWorkspaceID()
	require.NoError(t, err)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err = handle.Exec(insertTask, taskID.String(), workspaceID.String(), now, now)
	require.NoError(t, err)
}

// newGitRepo creates a repository with one commit on branch main and returns its root. The identity is
// set locally so commits do not depend on the machine configuration.
func newGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	runGit(t, root, "config", "user.email", "test@example.com")
	runGit(t, root, "config", "user.name", "Test")
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"), []byte("demo\n"), 0o600))
	runGit(t, root, "add", "README.md")
	runGit(t, root, "commit", "-q", "-m", "init")
	return root
}

// runGit runs git in dir and returns its standard output; any failure stops the test.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	require.NoError(t, command.Run(), "git %v: %s", args, stderr.String())
	return stdout.String()
}
