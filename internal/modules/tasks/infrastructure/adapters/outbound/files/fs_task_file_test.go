package files

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// newWorktree creates a worktree with one file, src/a.rs, and returns the temporary parent and the root.
func newWorktree(t *testing.T) (string, string) {
	t.Helper()
	parent := t.TempDir()
	root := filepath.Join(parent, "wt")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "src", "a.rs"), []byte("fn a() {}\n"), 0o644))
	return parent, root
}

// Rust: fs_task_file_adapter::validation_rejects_unsafe_paths
func TestValidationRejectsUnsafePaths(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"", "/etc/passwd", "../x", "a/../../x", "a/..", ".git/config", "src/.git/hooks", ".git", "a\x00b"} {
		_, err := ValidateRelativePath(bad)
		assert.ErrorIs(t, err, werrors.InvalidFilePath(""), "%q", bad)
	}
}

// Rust: fs_task_file_adapter::validation_accepts_nested_relative_paths
func TestValidationAcceptsNestedRelativePaths(t *testing.T) {
	t.Parallel()
	cleaned, err := ValidateRelativePath("src/a.rs")
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("src", "a.rs"), cleaned)

	_, err = ValidateRelativePath(".github/ci.yml")
	assert.NoError(t, err)
}

// Rust: fs_task_file_adapter::round_trip_overwrites_existing_file
func TestRoundTripOverwritesExistingFile(t *testing.T) {
	t.Parallel()
	_, root := newWorktree(t)
	adapter := NewFsTaskFile()

	require.NoError(t, adapter.Write(context.Background(), root, "src/a.rs", "fn b() {}\n"))

	file, err := adapter.Read(context.Background(), root, "src/a.rs")
	require.NoError(t, err)
	assert.Equal(t, "src/a.rs", file.Path)
	assert.Equal(t, "fn b() {}\n", file.Content)
}

// Rust: fs_task_file_adapter::read_rejects_traversal_absolute_and_git
func TestReadRejectsTraversalAbsoluteAndGit(t *testing.T) {
	t.Parallel()
	_, root := newWorktree(t)
	adapter := NewFsTaskFile()
	for _, bad := range []string{"../outside", "/etc/passwd", ".git/config"} {
		_, err := adapter.Read(context.Background(), root, bad)
		assert.ErrorIs(t, err, werrors.InvalidFilePath(""), bad)
	}
}

// Rust: fs_task_file_adapter::rejects_symlink_escape
func TestRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()
	parent, root := newWorktree(t)
	secret := filepath.Join(parent, "secret.txt")
	require.NoError(t, os.WriteFile(secret, []byte("secret"), 0o600))
	require.NoError(t, os.Symlink(secret, filepath.Join(root, "link.txt")))
	require.NoError(t, os.Symlink(parent, filepath.Join(root, "linkdir")))
	adapter := NewFsTaskFile()

	_, readErr := adapter.Read(context.Background(), root, "link.txt")
	assert.ErrorIs(t, readErr, werrors.InvalidFilePath(""))
	writeErr := adapter.Write(context.Background(), root, "linkdir/secret.txt", "x")
	assert.ErrorIs(t, writeErr, werrors.InvalidFilePath(""))

	content, err := os.ReadFile(secret)
	require.NoError(t, err)
	assert.Equal(t, "secret", string(content))
}

// Rust: fs_task_file_adapter::rejects_too_large_files
func TestRejectsTooLargeFiles(t *testing.T) {
	t.Parallel()
	_, root := newWorktree(t)
	big := strings.Repeat("a", models.MaxTaskFileBytes+1)
	require.NoError(t, os.WriteFile(filepath.Join(root, "big.txt"), []byte(big), 0o644))
	adapter := NewFsTaskFile()

	_, readErr := adapter.Read(context.Background(), root, "big.txt")
	assert.ErrorIs(t, readErr, werrors.FileNotEditable(""))
	writeErr := adapter.Write(context.Background(), root, "src/a.rs", big)
	assert.ErrorIs(t, writeErr, werrors.FileNotEditable(""))
}

// Rust: fs_task_file_adapter::rejects_binary_files
func TestRejectsBinaryFiles(t *testing.T) {
	t.Parallel()
	_, root := newWorktree(t)
	require.NoError(t, os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0xff, 0xfe, 0x00, 0x01}, 0o644))

	_, err := NewFsTaskFile().Read(context.Background(), root, "bin.dat")

	assert.ErrorIs(t, err, werrors.FileNotEditable(""))
}

// Rust: fs_task_file_adapter::never_creates_files_or_writes_directories
func TestNeverCreatesFilesOrWritesDirectories(t *testing.T) {
	t.Parallel()
	_, root := newWorktree(t)
	adapter := NewFsTaskFile()

	assert.ErrorIs(t, adapter.Write(context.Background(), root, "src/new.rs", "x"), werrors.FileNotEditable(""))
	assert.NoFileExists(t, filepath.Join(root, "src", "new.rs"))
	assert.ErrorIs(t, adapter.Write(context.Background(), root, "src", "x"), werrors.FileNotEditable(""))
	_, err := adapter.Read(context.Background(), root, "src")
	assert.ErrorIs(t, err, werrors.FileNotEditable(""))
}
