package tasks_test

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/storage"
)

// storageBytes is the content of every fixture image.
var storageBytes = []byte("\x89PNG fake image bytes")

// storageImage is an image of the given MIME type whose data is storageBytes, base64 encoded.
func storageImage(mimeType string) models.ImageAttachment {
	return models.ImageAttachment{Name: "foto", MimeType: mimeType, Data: base64.StdEncoding.EncodeToString(storageBytes)}
}

// newStorage builds the storage over a fresh "attachments" folder inside a temporary directory.
func newStorage(t *testing.T) (*storage.FsAttachmentStorage, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "attachments")
	adapter, err := storage.NewFsAttachmentStorage(dir)
	require.NoError(t, err)
	return adapter, dir
}

// folderEntries lists the names in dir; a missing folder has no entries.
func folderEntries(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}

// Rust: fs_attachment_storage_adapter_test::saves_and_reads_back_the_same_bytes
func TestStorageSavesAndReadsBackTheSameBytes(t *testing.T) {
	t.Parallel()
	adapter, dir := newStorage(t)
	taskID := newTaskID(t)
	attachmentID := uuid.NewString()

	path, err := adapter.Save(context.Background(), taskID, attachmentID, storageImage("image/png"))

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(taskID.String(), attachmentID+".png"), path)
	onDisk, err := os.ReadFile(filepath.Join(dir, path))
	require.NoError(t, err)
	assert.Equal(t, storageBytes, onDisk)
	read, err := adapter.Read(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, base64.StdEncoding.EncodeToString(storageBytes), read)
}

// Rust: fs_attachment_storage_adapter_test::maps_mime_types_to_extensions
func TestStorageMapsMimeTypesToExtensions(t *testing.T) {
	t.Parallel()
	adapter, _ := newStorage(t)
	taskID := newTaskID(t)
	for mimeType, extension := range map[string]string{
		"image/png":  "png",
		"image/jpeg": "jpg",
		"image/gif":  "gif",
		"image/webp": "webp",
	} {
		attachmentID := uuid.NewString()
		path, err := adapter.Save(context.Background(), taskID, attachmentID, storageImage(mimeType))
		require.NoError(t, err, mimeType)
		assert.Equal(t, filepath.Join(taskID.String(), attachmentID+"."+extension), path, mimeType)
	}
}

// Rust: fs_attachment_storage_adapter_test::rejects_unsupported_mime_and_invalid_base64
func TestStorageRejectsUnsupportedMimeAndInvalidBase64WithoutWriting(t *testing.T) {
	t.Parallel()
	adapter, dir := newStorage(t)
	taskID := newTaskID(t)

	_, err := adapter.Save(context.Background(), taskID, uuid.NewString(), storageImage("image/bmp"))
	assert.ErrorIs(t, err, werrors.Storage(""))

	broken := storageImage("image/png")
	broken.Data = "%%%not base64%%%"
	_, err = adapter.Save(context.Background(), taskID, uuid.NewString(), broken)
	assert.ErrorIs(t, err, werrors.Storage(""))

	assert.Empty(t, folderEntries(t, filepath.Join(dir, taskID.String())))
}

// Rust: fs_attachment_storage_adapter_test::write_is_atomic_and_leaves_no_temporary_files
func TestStorageWriteIsAtomicAndLeavesNoTemporaryFiles(t *testing.T) {
	t.Parallel()
	adapter, dir := newStorage(t)
	taskID := newTaskID(t)
	attachmentID := uuid.NewString()

	_, err := adapter.Save(context.Background(), taskID, attachmentID, storageImage("image/png"))
	require.NoError(t, err)
	_, err = adapter.Save(context.Background(), taskID, attachmentID, storageImage("image/png"))
	require.NoError(t, err, "overwriting an attachment is allowed")

	assert.Equal(t, []string{attachmentID + ".png"}, folderEntries(t, filepath.Join(dir, taskID.String())))
}

// Rust: fs_attachment_storage_adapter_test::rejects_path_traversal_in_ids_and_paths
func TestStorageRejectsPathTraversalInIdsAndPaths(t *testing.T) {
	t.Parallel()
	adapter, dir := newStorage(t)
	outside := filepath.Join(filepath.Dir(dir), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))

	for _, path := range []string{"../secret.txt", "/etc/passwd", "a/../../secret.txt", "", "."} {
		_, readErr := adapter.Read(context.Background(), path)
		assert.ErrorIs(t, readErr, werrors.Storage(""), "read %q", path)
		assert.ErrorIs(t, adapter.Delete(context.Background(), path), werrors.Storage(""), "delete %q", path)
	}
	_, err := adapter.Save(context.Background(), newTaskID(t), "../evil", storageImage("image/png"))
	assert.ErrorIs(t, err, werrors.Storage(""), "an attachment id that is not a generated UUID is rejected")

	assert.FileExists(t, outside)
}

// Rust: fs_attachment_storage_adapter_test::reading_a_missing_file_fails_with_a_clear_error
func TestStorageReadingAMissingFileFailsWithAClearError(t *testing.T) {
	t.Parallel()
	adapter, _ := newStorage(t)

	_, err := adapter.Read(context.Background(), "nada/ninguno.png")

	assert.ErrorIs(t, err, werrors.Storage(""))
	assert.Contains(t, err.Error(), "nada/ninguno.png")
}

// Rust: fs_attachment_storage_adapter_test::delete_removes_one_file_and_is_idempotent
func TestStorageDeleteRemovesOneFileAndIsIdempotent(t *testing.T) {
	t.Parallel()
	adapter, _ := newStorage(t)
	taskID := newTaskID(t)
	first, err := adapter.Save(context.Background(), taskID, uuid.NewString(), storageImage("image/png"))
	require.NoError(t, err)
	second, err := adapter.Save(context.Background(), taskID, uuid.NewString(), storageImage("image/png"))
	require.NoError(t, err)

	require.NoError(t, adapter.Delete(context.Background(), first))
	require.NoError(t, adapter.Delete(context.Background(), first), "a second delete is not an error")

	_, err = adapter.Read(context.Background(), first)
	assert.Error(t, err)
	_, err = adapter.Read(context.Background(), second)
	assert.NoError(t, err)
}

// Rust: fs_attachment_storage_adapter_test::delete_task_dir_removes_only_that_task_and_is_idempotent
func TestStorageDeleteTaskDirRemovesOnlyThatTaskAndIsIdempotent(t *testing.T) {
	t.Parallel()
	adapter, _ := newStorage(t)
	taskID := newTaskID(t)
	other := newTaskID(t)
	path, err := adapter.Save(context.Background(), taskID, uuid.NewString(), storageImage("image/png"))
	require.NoError(t, err)
	otherPath, err := adapter.Save(context.Background(), other, uuid.NewString(), storageImage("image/png"))
	require.NoError(t, err)

	require.NoError(t, adapter.DeleteTaskDir(context.Background(), taskID))
	require.NoError(t, adapter.DeleteTaskDir(context.Background(), taskID))
	require.NoError(t, adapter.DeleteTaskDir(context.Background(), newTaskID(t)), "a folder that never existed")

	_, err = adapter.Read(context.Background(), path)
	assert.Error(t, err)
	_, err = adapter.Read(context.Background(), otherPath)
	assert.NoError(t, err)
}

// Rust: fs_attachment_storage_adapter_test::read_bytes_returns_raw_bytes_and_not_found_for_missing_or_escaping_paths
func TestStorageReadBytesReturnsRawBytesAndNotFoundForMissingOrEscapingPaths(t *testing.T) {
	t.Parallel()
	adapter, _ := newStorage(t)
	taskID := newTaskID(t)
	path, err := adapter.Save(context.Background(), taskID, uuid.NewString(), storageImage("image/png"))
	require.NoError(t, err)

	bytes, err := adapter.ReadBytes(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, storageBytes, bytes)

	_, err = adapter.ReadBytes(context.Background(), filepath.Join(taskID.String(), "nope.png"))
	assert.ErrorIs(t, err, werrors.NotFound(""))

	_, err = adapter.ReadBytes(context.Background(), "../secret")
	assert.ErrorIs(t, err, werrors.Storage(""))
}

// Security: a symbolic link inside the folder that points outside must not be readable through the storage.
func TestStorageReadRefusesSymbolicLinkEscapes(t *testing.T) {
	t.Parallel()
	adapter, dir := newStorage(t)
	secret := filepath.Join(filepath.Dir(dir), "secret.png")
	require.NoError(t, os.WriteFile(secret, []byte("secret"), 0o600))
	taskID := newTaskID(t)
	require.NoError(t, os.MkdirAll(filepath.Join(dir, taskID.String()), 0o700))
	link := filepath.Join(taskID.String(), "link.png")
	require.NoError(t, os.Symlink(secret, filepath.Join(dir, link)))

	_, err := adapter.ReadBytes(context.Background(), link)

	assert.ErrorIs(t, err, werrors.Storage(""))
	assert.NotContains(t, err.Error(), "secret")
}

// Security: a task folder that is a symbolic link to a folder outside must not receive the image.
func TestStorageSaveRefusesWritesThroughASymbolicLinkedFolder(t *testing.T) {
	t.Parallel()
	adapter, dir := newStorage(t)
	outside := filepath.Join(filepath.Dir(dir), "outside")
	require.NoError(t, os.MkdirAll(outside, 0o700))
	taskID := newTaskID(t)
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, taskID.String())))

	_, err := adapter.Save(context.Background(), taskID, uuid.NewString(), storageImage("image/png"))

	assert.ErrorIs(t, err, werrors.Storage(""))
	assert.Empty(t, folderEntries(t, outside), "nothing is written outside the attachments folder")
}

// Security: deleting a task folder that is a symbolic link removes the link, never the files it points to.
func TestStorageDeleteTaskDirDoesNotFollowSymbolicLinks(t *testing.T) {
	t.Parallel()
	adapter, dir := newStorage(t)
	outside := filepath.Join(filepath.Dir(dir), "outside")
	require.NoError(t, os.MkdirAll(outside, 0o700))
	keep := filepath.Join(outside, "keep.png")
	require.NoError(t, os.WriteFile(keep, []byte("keep"), 0o600))
	taskID := newTaskID(t)
	require.NoError(t, os.Symlink(outside, filepath.Join(dir, taskID.String())))

	require.NoError(t, adapter.DeleteTaskDir(context.Background(), taskID))

	assert.FileExists(t, keep)
}

// Security: the decoded size limit is enforced by the storage too, not only by the request validation.
func TestStorageRejectsImagesLargerThanTheLimit(t *testing.T) {
	t.Parallel()
	adapter, _ := newStorage(t)
	oversized := storageImage("image/png")
	oversized.Name = "grande"
	oversized.Data = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", models.MaxImageBytes+1)))

	_, err := adapter.Save(context.Background(), newTaskID(t), uuid.NewString(), oversized)

	assert.ErrorIs(t, err, werrors.Storage(""))
	assert.Contains(t, err.Error(), "grande")
}
