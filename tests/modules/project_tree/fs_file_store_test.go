package projecttree_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	treeerrors "iworkflow/internal/modules/project_tree/domain/errors"
	"iworkflow/internal/modules/project_tree/domain/models"
	"iworkflow/internal/modules/project_tree/infrastructure/adapters/outbound/fs"
)

// maxEditableBytes mirrors the adapter limit (2 MiB) so the size tests need no magic numbers.
const maxEditableBytes int = 2 * 1024 * 1024

func TestReadsNestedFileWithNameAndContent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.rs"), []byte("fn main() {}"), 0o600))

	file, err := fs.NewFileStore().Read(context.Background(), dir, "src/main.rs")

	require.NoError(t, err)
	require.Equal(t, models.ProjectFile{Name: "main.rs", Path: "src/main.rs", Content: "fn main() {}"}, file)
}

func TestReadRejectsTraversalDirectoriesAndMissingFiles(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "src"), 0o755))
	store := fs.NewFileStore()

	_, err := store.Read(context.Background(), dir, "../x")
	require.Equal(t, treeerrors.InvalidPath("../x"), err)

	_, err = store.Read(context.Background(), dir, "src")
	require.Equal(t, treeerrors.NotAFile("src"), err)

	_, err = store.Read(context.Background(), dir, "nope.txt")
	require.Equal(t, treeerrors.PathNotFound("nope.txt"), err)

	_, err = store.Read(context.Background(), dir, "")
	require.Equal(t, treeerrors.NotAFile(""), err)
}

func TestReadRejectsBinaryContent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.bin"), []byte{0xff, 0xfe, 0x00}, 0o600))

	_, err := fs.NewFileStore().Read(context.Background(), dir, "a.bin")

	require.Equal(t, treeerrors.NotText("a.bin"), err)
}

func TestReadAndWriteRefuseFilesTooLargeToEdit(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.txt"), bytes.Repeat([]byte("a"), maxEditableBytes+1), 0o600))
	store := fs.NewFileStore()

	_, err := store.Read(context.Background(), dir, "big.txt")
	require.Equal(t, treeerrors.FileTooLarge("big.txt"), err)

	err = store.Write(context.Background(), dir, "big.txt", "small")
	require.Equal(t, treeerrors.FileTooLarge("big.txt"), err)

	err = store.Write(context.Background(), dir, "big.txt", string(bytes.Repeat([]byte("b"), maxEditableBytes+1)))
	require.Equal(t, treeerrors.FileTooLarge("big.txt"), err)
}

func TestWriteOverwritesExistingFileOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("old"), 0o600))
	store := fs.NewFileStore()

	require.NoError(t, store.Write(context.Background(), dir, "a.txt", "new"))

	content, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	require.NoError(t, err)
	require.Equal(t, "new", string(content))

	err = store.Write(context.Background(), dir, "missing.txt", "x")
	require.Equal(t, treeerrors.PathNotFound("missing.txt"), err)
	require.NoFileExists(t, filepath.Join(dir, "missing.txt"))
}

func TestSymlinkEscapingRootIsRefusedForReadAndWrite(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("s"), 0o600))
	dir := t.TempDir()
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(dir, "link")))
	store := fs.NewFileStore()

	_, err := store.Read(context.Background(), dir, "link")
	require.Equal(t, treeerrors.PathOutsideRoot("link"), err)

	err = store.Write(context.Background(), dir, "link", "x")
	require.Equal(t, treeerrors.PathOutsideRoot("link"), err)

	content, err := os.ReadFile(filepath.Join(outside, "secret.txt"))
	require.NoError(t, err)
	require.Equal(t, "s", string(content), "the file outside the root must not change")
}

func TestFileInsideASymlinkedDirectoryOutsideRootIsRefused(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("s"), 0o600))
	dir := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, "escape")))
	store := fs.NewFileStore()

	_, err := store.Read(context.Background(), dir, "escape/secret.txt")
	require.Equal(t, treeerrors.PathOutsideRoot("escape/secret.txt"), err)

	err = store.Write(context.Background(), dir, "escape/secret.txt", "x")
	require.Equal(t, treeerrors.PathOutsideRoot("escape/secret.txt"), err)
}
