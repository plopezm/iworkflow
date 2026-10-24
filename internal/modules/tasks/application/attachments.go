//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// ListTaskAttachments returns the metadata of the images of a task, ordered by position. It does not
// touch the disk: the content is served separately by GetTaskAttachment.
type ListTaskAttachments struct {
	attachments outbound.TaskAttachmentRepository
}

// NewListTaskAttachments builds the use case.
func NewListTaskAttachments(attachments outbound.TaskAttachmentRepository) *ListTaskAttachments {
	return &ListTaskAttachments{attachments: attachments}
}

// Execute lists the metadata of the task images.
func (u *ListTaskAttachments) Execute(ctx context.Context, id models.TaskID) ([]models.StoredAttachment, error) {
	return u.attachments.ListByTask(ctx, id)
}

// GetTaskAttachment returns the content of one image of a task. The file path comes from the stored
// metadata of that task, never from the identifier received: an image of another task counts as missing.
type GetTaskAttachment struct {
	attachments outbound.TaskAttachmentRepository
	storage     outbound.AttachmentStorage
}

// NewGetTaskAttachment builds the use case.
func NewGetTaskAttachment(attachments outbound.TaskAttachmentRepository, storage outbound.AttachmentStorage) *GetTaskAttachment {
	return &GetTaskAttachment{attachments: attachments, storage: storage}
}

// Execute returns the bytes and the MIME type of the image, or NotFound.
func (u *GetTaskAttachment) Execute(ctx context.Context, id models.TaskID, attachmentID string) (models.TaskAttachmentContent, error) {
	stored, err := u.attachments.ListByTask(ctx, id)
	if err != nil {
		return models.TaskAttachmentContent{}, err
	}
	for _, attachment := range stored {
		if attachment.ID != attachmentID {
			continue
		}
		bytes, err := u.storage.ReadBytes(ctx, attachment.Path)
		if err != nil {
			return models.TaskAttachmentContent{}, err
		}
		return models.TaskAttachmentContent{MimeType: attachment.MimeType, Bytes: bytes}, nil
	}
	return models.TaskAttachmentContent{}, werrors.NotFound("adjunto " + attachmentID)
}

// AttachmentCleanup removes the image metadata and the image folder of a task, best effort: failures
// are logged and never returned to the use case that asks for the cleanup.
type AttachmentCleanup struct {
	attachments outbound.TaskAttachmentRepository
	storage     outbound.AttachmentStorage
}

// NewAttachmentCleanup builds the service.
func NewAttachmentCleanup(attachments outbound.TaskAttachmentRepository, storage outbound.AttachmentStorage) *AttachmentCleanup {
	return &AttachmentCleanup{attachments: attachments, storage: storage}
}

// CleanupTask deletes the metadata rows, then the folder. A failure of the first step does not skip the second.
func (c *AttachmentCleanup) CleanupTask(ctx context.Context, id models.TaskID) {
	if err := c.attachments.DeleteByTask(ctx, id); err != nil {
		slog.Warn("could not delete the task attachments from the database", "task_id", id.String(), "error", err)
	}
	if err := c.storage.DeleteTaskDir(ctx, id); err != nil {
		slog.Warn("could not delete the task attachment files", "task_id", id.String(), "error", err)
	}
}
