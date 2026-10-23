package fs_test

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

func listNames(t *testing.T, contents models.DirectoryContents) []string {
	t.Helper()
	names := make([]string, 0, len(contents.Entries))
	for _, entry := range contents.Entries {
		names = append(names, entry.Name)
	}
	return names
}

func TestDirectoryReaderSortsDirectoriesFirstThenFilesCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "beta"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "Alpha"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "zeta.txt"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Apple.txt"), nil, 0o600))

	contents, err := fs.NewDirectoryReader().List(context.Background(), dir, "")

	require.NoError(t, err)
	require.Equal(t, []string{"Alpha", "beta", "Apple.txt", "zeta.txt"}, listNames(t, contents))
	require.Empty(t, contents.Path)
	require.Equal(t, models.TreeEntryDirectory, contents.Entries[0].Kind)
	require.Equal(t, models.TreeEntryFile, contents.Entries[2].Kind)
}

func TestDirectoryReaderHidesGitButShowsOtherDotfiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".gitignore"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".env"), nil, 0o600))

	contents, err := fs.NewDirectoryReader().List(context.Background(), dir, "")

	require.NoError(t, err)
	require.Equal(t, []string{".env", ".gitignore"}, listNames(t, contents))
}

func TestDirectoryReaderListsNestedRelativePaths(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src", "ui"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.rs"), nil, 0o600))

	contents, err := fs.NewDirectoryReader().List(context.Background(), dir, "src")

	require.NoError(t, err)
	require.Equal(t, "src", contents.Path)
	require.Equal(t, []string{"ui", "main.rs"}, listNames(t, contents))
	require.Equal(t, "src/ui", contents.Entries[0].Path)
	require.Equal(t, "src/main.rs", contents.Entries[1].Path)
}

func TestDirectoryReaderRejectsTraversalAndAbsolutePaths(t *testing.T) {
	dir := t.TempDir()
	reader := fs.NewDirectoryReader()

	_, err := reader.List(context.Background(), dir, "../x")
	require.Equal(t, treeerrors.InvalidPath("../x"), err)

	_, err = reader.List(context.Background(), dir, "/etc")
	require.Equal(t, treeerrors.InvalidPath("/etc"), err)
}

func TestDirectoryReaderSkipsSymlinksThatEscapeTheRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))
	reader := fs.NewDirectoryReader()

	_, err := reader.List(context.Background(), root, "link")
	require.Equal(t, treeerrors.PathOutsideRoot("link"), err)

	contents, err := reader.List(context.Background(), root, "")
	require.NoError(t, err)
	require.Empty(t, contents.Entries)
}

func TestDirectoryReaderSkipsBrokenSymlinks(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Symlink(filepath.Join(root, "missing"), filepath.Join(root, "broken")))

	contents, err := fs.NewDirectoryReader().List(context.Background(), root, "")

	require.NoError(t, err)
	require.Empty(t, contents.Entries)
}

func TestDirectoryReaderReportsFilesAsNotADirectory(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0o600))

	_, err := fs.NewDirectoryReader().List(context.Background(), dir, "a.txt")

	require.Equal(t, treeerrors.NotADirectory("a.txt"), err)
}

func TestDirectoryReaderReportsMissingPaths(t *testing.T) {
	dir := t.TempDir()

	_, err := fs.NewDirectoryReader().List(context.Background(), dir, "nope")

	require.Equal(t, treeerrors.PathNotFound("nope"), err)
}

func TestDirectoryReaderKeepsSiblingsWithTheSamePrefixOutside(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "app")
	sibling := filepath.Join(base, "app2")
	require.NoError(t, os.Mkdir(root, 0o755))
	require.NoError(t, os.Mkdir(sibling, 0o755))
	require.NoError(t, os.Symlink(sibling, filepath.Join(root, "link")))

	_, err := fs.NewDirectoryReader().List(context.Background(), root, "link")

	require.Equal(t, treeerrors.PathOutsideRoot("link"), err)
}
