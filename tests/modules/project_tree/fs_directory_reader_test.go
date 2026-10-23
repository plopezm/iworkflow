package projecttree_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/outbound/fs"
)

func TestListsRootWithDirectoriesFirstAndHidesGit(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0o600))

	contents, err := fs.NewDirectoryReader().List(context.Background(), dir, "")

	require.NoError(t, err)
	names := make([]string, 0, len(contents.Entries))
	for _, entry := range contents.Entries {
		names = append(names, entry.Name)
	}
	require.Equal(t, []string{"src", ".gitignore", "README.md"}, names)
	require.Equal(t, models.TreeEntryDirectory, contents.Entries[0].Kind)
}

func TestListsNestedDirectoryWithRelativePaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a", "b", "c.txt"), nil, 0o600))

	contents, err := fs.NewDirectoryReader().List(context.Background(), dir, "a/b")

	require.NoError(t, err)
	require.Equal(t, "a/b", contents.Path)
	require.Len(t, contents.Entries, 1)
	require.Equal(t, "a/b/c.txt", contents.Entries[0].Path)
}

func TestRejectsTraversalAndAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	reader := fs.NewDirectoryReader()

	_, err := reader.List(context.Background(), dir, "..")
	require.Equal(t, treeerrors.InvalidPath(".."), err)

	_, err = reader.List(context.Background(), dir, "/tmp")
	require.Equal(t, treeerrors.InvalidPath("/tmp"), err)
}

func TestRejectsSymlinkEscapingTheRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), nil, 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "escape")))
	reader := fs.NewDirectoryReader()

	_, err := reader.List(context.Background(), root, "escape")
	require.Equal(t, treeerrors.PathOutsideRoot("escape"), err)

	contents, err := reader.List(context.Background(), root, "")
	require.NoError(t, err)
	require.Empty(t, contents.Entries)
}

func TestReportsMissingAndNonDirectoryPaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.txt"), nil, 0o600))
	reader := fs.NewDirectoryReader()

	_, err := reader.List(context.Background(), dir, "missing")
	require.Equal(t, treeerrors.PathNotFound("missing"), err)

	_, err = reader.List(context.Background(), dir, "f.txt")
	require.Equal(t, treeerrors.NotADirectory("f.txt"), err)
}
