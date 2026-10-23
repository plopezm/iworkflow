package workspaces_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/workspaces/domain/errors"
	"iworkflow/internal/modules/workspaces/infrastructure/adapters/outbound/fs"
)

func TestDirectoryListingShowsOnlyFoldersHidesDotEntriesAndSortsCaseInsensitively(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "Beta"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "alpha"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".hidden"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600))

	listing, err := fs.NewDirectoryBrowser(dir).List(context.Background(), dir)

	require.NoError(t, err)
	names := make([]string, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		names = append(names, entry.Name)
	}
	assert.Equal(t, []string{"alpha", "Beta"}, names)
}

func TestDirectoryListingMarksGitRepositories(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "repo", ".git"), 0o755))
	require.NoError(t, os.Mkdir(filepath.Join(dir, "plain"), 0o755))

	listing, err := fs.NewDirectoryBrowser(dir).List(context.Background(), dir)

	require.NoError(t, err)
	flags := map[string]bool{}
	for _, entry := range listing.Entries {
		flags[entry.Name] = entry.IsGitRepo
	}
	assert.True(t, flags["repo"])
	assert.False(t, flags["plain"])
}

func TestDirectoryListingFallsBackToNearestExistingAncestor(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing", "nested", "child")

	listing, err := fs.NewDirectoryBrowser(dir).List(context.Background(), missing)

	require.NoError(t, err)
	assert.Equal(t, dir, listing.Path)
}

func TestDirectoryListingReportsParentFolder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "child"), 0o755))

	listing, err := fs.NewDirectoryBrowser(dir).List(context.Background(), filepath.Join(dir, "child"))

	require.NoError(t, err)
	assert.Equal(t, dir, listing.Parent)
}

func TestDirectoryListingHasNoParentAtFilesystemRoot(t *testing.T) {
	root := string(filepath.Separator)
	if runtime.GOOS == "windows" {
		t.Skip("filesystem root differs on windows")
	}

	listing, err := fs.NewDirectoryBrowser(root).List(context.Background(), root)

	require.NoError(t, err)
	assert.Empty(t, listing.Parent)
}

func TestCreateDirectoryCreatesFolderInsideParent(t *testing.T) {
	dir := t.TempDir()

	created, err := fs.NewDirectoryBrowser(dir).CreateDirectory(context.Background(), dir, "nueva")

	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(resolved, "nueva"), created)
	assert.DirExists(t, created)
}

func TestCreateDirectoryFailsWhenItAlreadyExists(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "dup"), 0o755))

	_, err := fs.NewDirectoryBrowser(dir).CreateDirectory(context.Background(), dir, "dup")

	require.ErrorIs(t, err, werrors.IO(""))
}

func TestCreateDirectoryFailsWhenParentIsMissing(t *testing.T) {
	root := t.TempDir()

	_, err := fs.NewDirectoryBrowser(root).CreateDirectory(context.Background(), filepath.Join(root, "missing"), "x")

	require.ErrorIs(t, err, werrors.PathNotFound(""))
}

func TestCreateDirectoryRejectsParentOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()

	_, err := fs.NewDirectoryBrowser(root).CreateDirectory(context.Background(), outside, "x")

	require.ErrorIs(t, err, werrors.PathOutsideAllowedRoot(""))
	assert.NoDirExists(t, filepath.Join(outside, "x"))
}

func TestCreateDirectoryRejectsDotDotEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.Mkdir(root, 0o755))
	require.NoError(t, os.Mkdir(outside, 0o755))

	_, err := fs.NewDirectoryBrowser(root).CreateDirectory(context.Background(), filepath.Join(root, "..", "outside"), "x")

	require.ErrorIs(t, err, werrors.PathOutsideAllowedRoot(""))
	assert.NoDirExists(t, filepath.Join(outside, "x"))
}

func TestCreateDirectoryRejectsSymlinkPointingOutsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on windows")
	}
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.Mkdir(root, 0o755))
	require.NoError(t, os.Mkdir(outside, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	_, err := fs.NewDirectoryBrowser(root).CreateDirectory(context.Background(), filepath.Join(root, "link"), "x")

	require.ErrorIs(t, err, werrors.PathOutsideAllowedRoot(""))
	assert.NoDirExists(t, filepath.Join(outside, "x"))
}

func TestCreateDirectoryAllowsSymlinkPointingInsideRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privileges on windows")
	}
	root := t.TempDir()
	inside := filepath.Join(root, "inside")
	require.NoError(t, os.Mkdir(inside, 0o755))
	require.NoError(t, os.Symlink(inside, filepath.Join(root, "link")))

	created, err := fs.NewDirectoryBrowser(root).CreateDirectory(context.Background(), filepath.Join(root, "link"), "x")

	require.NoError(t, err)
	assert.DirExists(t, filepath.Join(inside, "x"))
	assert.DirExists(t, created)
}
