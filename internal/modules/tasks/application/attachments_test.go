package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

func storedAttachment(id string, path string) models.StoredAttachment {
	return models.StoredAttachment{ID: id, Name: id + ".png", MimeType: "image/png", Path: path, SizeBytes: 3}
}

// Rust: list_task_attachments::returns_metadata_in_order
func TestListTaskAttachmentsReturnsMetadataInOrder(t *testing.T) {
	t.Parallel()
	attachments := &fakeAttachments{listed: []models.StoredAttachment{storedAttachment("a", "a.png"), storedAttachment("b", "b.png")}}

	listed, err := application.NewListTaskAttachments(attachments).Execute(context.Background(), newTaskIDValue(t))

	require.NoError(t, err)
	assert.Equal(t, attachments.listed, listed)
}

// Rust: list_task_attachments::repository_failure_is_propagated
func TestListTaskAttachmentsPropagatesRepositoryFailure(t *testing.T) {
	t.Parallel()
	attachments := &failingListAttachments{err: werrors.Repository("boom")}

	_, err := application.NewListTaskAttachments(attachments).Execute(context.Background(), newTaskIDValue(t))

	assert.ErrorIs(t, err, werrors.Repository("boom"))
}

// failingListAttachments fails every list call.
type failingListAttachments struct {
	fakeAttachments
	err error
}

func (f *failingListAttachments) ListByTask(context.Context, models.TaskID) ([]models.StoredAttachment, error) {
	return nil, f.err
}

// Rust: get_task_attachment::returns_bytes_and_mime_of_the_matching_attachment
func TestGetTaskAttachmentReturnsBytesAndMimeOfTheMatchingAttachment(t *testing.T) {
	t.Parallel()
	attachments := &fakeAttachments{listed: []models.StoredAttachment{storedAttachment("a", "a.png"), storedAttachment("b", "b.png")}}
	storage := &fakeStorage{readBytesResult: []byte("PNG")}

	content, err := application.NewGetTaskAttachment(attachments, storage).Execute(context.Background(), newTaskIDValue(t), "b")

	require.NoError(t, err)
	assert.Equal(t, models.TaskAttachmentContent{MimeType: "image/png", Bytes: []byte("PNG")}, content)
	assert.Equal(t, []string{"b.png"}, storage.readPaths, "the path comes from the stored metadata")
}

// Rust: get_task_attachment::unknown_or_foreign_attachment_is_not_found_without_touching_disk
func TestGetTaskAttachmentUnknownOrForeignIsNotFoundWithoutTouchingDisk(t *testing.T) {
	t.Parallel()
	attachments := &fakeAttachments{listed: []models.StoredAttachment{storedAttachment("a", "a.png")}}
	storage := &fakeStorage{}

	_, err := application.NewGetTaskAttachment(attachments, storage).Execute(context.Background(), newTaskIDValue(t), "someone-elses")

	assert.ErrorIs(t, err, werrors.NotFound(""))
	assert.Empty(t, storage.readPaths)
}

// Rust: get_task_attachment::missing_file_and_repository_errors_are_propagated
func TestGetTaskAttachmentPropagatesMissingFileAndRepositoryErrors(t *testing.T) {
	t.Parallel()
	attachments := &fakeAttachments{listed: []models.StoredAttachment{storedAttachment("a", "a.png")}}
	storage := &fakeStorage{readBytesErr: werrors.NotFound("adjunto a.png")}

	_, missing := application.NewGetTaskAttachment(attachments, storage).Execute(context.Background(), newTaskIDValue(t), "a")
	_, broken := application.NewGetTaskAttachment(&failingListAttachments{err: errors.New("boom")}, storage).Execute(context.Background(), newTaskIDValue(t), "a")

	assert.ErrorIs(t, missing, werrors.NotFound(""))
	assert.EqualError(t, broken, "boom")
}

// Rust: attachment_cleanup::removes_rows_and_directory
func TestAttachmentCleanupRemovesRowsAndDirectory(t *testing.T) {
	t.Parallel()
	attachments := &fakeAttachments{}
	storage := &fakeStorage{}

	application.NewAttachmentCleanup(attachments, storage).CleanupTask(context.Background(), newTaskIDValue(t))

	assert.Equal(t, 1, attachments.deletedByTask)
	assert.Len(t, storage.deletedDirs, 1)
}

// Rust: attachment_cleanup::repository_failure_still_deletes_the_directory
func TestAttachmentCleanupRepositoryFailureStillDeletesTheDirectory(t *testing.T) {
	t.Parallel()
	attachments := &fakeAttachments{failDeleteByTask: werrors.Repository("boom")}
	storage := &fakeStorage{}

	application.NewAttachmentCleanup(attachments, storage).CleanupTask(context.Background(), newTaskIDValue(t))

	assert.Len(t, storage.deletedDirs, 1)
}
