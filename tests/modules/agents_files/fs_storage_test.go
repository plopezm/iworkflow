package agents_files_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	agentserrors "iworkflow/internal/modules/agents_files/domain/errors"
	"iworkflow/internal/modules/agents_files/infrastructure/adapters/outbound/fs"
)

func newStorage(t *testing.T) (*fs.AgentsFileStorage, string) {
	t.Helper()
	dir := t.TempDir()
	return fs.NewAgentsFileStorage(filepath.Join(dir, "agents-files")), dir
}

func TestFsStorageStoresReadsAndRemovesAFile(t *testing.T) {
	ctx := context.Background()
	storage, _ := newStorage(t)
	id := mustNewID(t)

	path, err := storage.Store(ctx, id, []byte("# reglas\n"))

	require.NoError(t, err)
	require.True(t, filepath.IsAbs(path))
	require.Equal(t, id.String()+".md", filepath.Base(path))

	content, err := storage.Read(ctx, path)
	require.NoError(t, err)
	require.Equal(t, []byte("# reglas\n"), content)

	require.NoError(t, storage.Remove(ctx, path))
	_, statErr := os.Stat(path)
	require.True(t, os.IsNotExist(statErr))
	require.NoError(t, storage.Remove(ctx, path), "removing a missing file is not an error")
}

func TestFsStorageLeavesNoTemporaryFiles(t *testing.T) {
	ctx := context.Background()
	storage, dir := newStorage(t)

	_, err := storage.Store(ctx, mustNewID(t), []byte("x"))
	require.NoError(t, err)

	entries, err := os.ReadDir(filepath.Join(dir, "agents-files"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, filepath.Ext(entries[0].Name()) == ".md" && entries[0].Name()[0] != '.')
}

func TestFsStorageRejectsPathsOutsideTheBaseDir(t *testing.T) {
	ctx := context.Background()
	storage, dir := newStorage(t)
	_, err := storage.Store(ctx, mustNewID(t), []byte("x"))
	require.NoError(t, err)
	foreign := filepath.Join(dir, "foreign.md")
	require.NoError(t, os.WriteFile(foreign, []byte("secreto"), 0o644))
	traversal := filepath.Join(dir, "agents-files", "..", "foreign.md")

	for _, path := range []string{foreign, traversal, "/etc/hostname"} {
		_, readErr := storage.Read(ctx, path)
		removeErr := storage.Remove(ctx, path)
		require.ErrorIs(t, readErr, agentserrors.Storage(""), path)
		require.ErrorIs(t, removeErr, agentserrors.Storage(""), path)
	}
	require.FileExists(t, foreign)
}

func TestFsStorageReadOfAMissingFileIsAStorageError(t *testing.T) {
	storage, dir := newStorage(t)
	missing := filepath.Join(dir, "agents-files", mustNewID(t).String()+".md")

	_, err := storage.Read(context.Background(), missing)

	require.ErrorIs(t, err, agentserrors.Storage(""))
}

// Symlinked storage folders are accepted: the containment check also accepts the canonical path.
func TestFsStorageAcceptsTheCanonicalPathOfASymlinkedBaseDir(t *testing.T) {
	ctx := context.Background()
	target := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	require.NoError(t, os.Symlink(target, link))
	storage := fs.NewAgentsFileStorage(link)

	path, err := storage.Store(ctx, mustNewID(t), []byte("x"))
	require.NoError(t, err)

	content, err := storage.Read(ctx, path)
	require.NoError(t, err)
	require.Equal(t, []byte("x"), content)
}
