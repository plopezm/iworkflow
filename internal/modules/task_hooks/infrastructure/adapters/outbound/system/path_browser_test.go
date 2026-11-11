package system_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/system"
)

func TestPathBrowserListsFoldersFirstAndSkipsHidden(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "zdir"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.sh"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".hidden"), nil, 0o600))

	listing, err := system.NewPathBrowser().List(context.Background(), dir)

	require.NoError(t, err)
	require.Len(t, listing.Entries, 2)
	assert.Equal(t, "zdir", listing.Entries[0].Name)
	assert.True(t, listing.Entries[0].IsDir)
	assert.Equal(t, "a.sh", listing.Entries[1].Name)
	assert.Equal(t, filepath.Dir(dir), listing.Parent)
}

func TestPathBrowserFailsForAMissingFolder(t *testing.T) {
	_, err := system.NewPathBrowser().List(context.Background(), filepath.Join(t.TempDir(), "nope"))
	require.Error(t, err)
}

// An empty path lists the home folder of the user, HOME or its equivalent on each system.
func TestPathBrowserStartsAtTheHomeFolderWhenThePathIsEmpty(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(home, "projects"), 0o750))
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	listing, err := system.NewPathBrowser().List(context.Background(), "")

	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(home), listing.Path)
	require.Len(t, listing.Entries, 1)
	assert.Equal(t, "projects", listing.Entries[0].Name)
}
