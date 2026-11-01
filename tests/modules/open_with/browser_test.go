package open_with_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/outbound/filesystem"
)

func TestBrowserListsFoldersFirstThenFilesAndTheParent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "zdir"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Beta"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "alpha"), nil, 0o600))
	require.NoError(t, os.Symlink(filepath.Join(dir, "zdir"), filepath.Join(dir, "link")))
	require.NoError(t, os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "broken")))

	listing, err := filesystem.NewBrowser().Browse(context.Background(), dir)

	require.NoError(t, err)
	require.Equal(t, dir, listing.Path)
	require.Equal(t, filepath.Dir(dir), listing.Parent)
	var got []string
	for _, entry := range listing.Entries {
		got = append(got, entry.Name)
	}
	require.Equal(t, []string{"link", "zdir", "alpha", "Beta"}, got)
	require.True(t, listing.Entries[0].IsDir, "a link to a folder is a folder")
}

func TestBrowserAcceptsAFilePathAndEmptyPathAndRejectsBadOnes(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "app")
	require.NoError(t, os.WriteFile(file, nil, 0o600))
	browser := filesystem.NewBrowser()

	fromFile, err := browser.Browse(context.Background(), file)
	require.NoError(t, err)
	require.Equal(t, dir, fromFile.Path)

	home, err := browser.Browse(context.Background(), "")
	require.NoError(t, err)
	require.NotEmpty(t, home.Path)

	for _, bad := range []string{"relative/dir", filepath.Join(dir, "missing")} {
		_, err = browser.Browse(context.Background(), bad)
		require.ErrorIs(t, err, openerrors.ErrBrowseFailed, bad)
	}
}

func TestBrowserRootHasNoParent(t *testing.T) {
	listing, err := filesystem.NewBrowser().Browse(context.Background(), string(filepath.Separator))

	require.NoError(t, err)
	require.Empty(t, listing.Parent)
}
