package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskAttachmentRepository persists the metadata of the images of a task. Declared for slice D.
type TaskAttachmentRepository interface {
	// InsertMany stores the metadata in the received order, in one transaction (all or nothing).
	InsertMany(ctx context.Context, taskID models.TaskID, attachments []models.StoredAttachment) error
	// ListByTask returns the metadata ordered by position.
	ListByTask(ctx context.Context, taskID models.TaskID) ([]models.StoredAttachment, error)
	// Remove deletes only the rows whose id is in ids (unknown ids are ignored), in one transaction, and
	// returns the deleted rows so the caller can delete their files.
	Remove(ctx context.Context, taskID models.TaskID, ids []string) ([]models.StoredAttachment, error)
	// DeleteByTask deletes every image metadata row of the task.
	DeleteByTask(ctx context.Context, taskID models.TaskID) error
}

// AttachmentStorage stores the image files of the tasks. Paths are always relative to the storage root.
// Declared for slice D.
type AttachmentStorage interface {
	// Save stores the image (base64 in image.Data) for the task and returns its relative path.
	Save(ctx context.Context, taskID models.TaskID, attachmentID string, image models.ImageAttachment) (string, error)
	// Read returns the file encoded in base64.
	Read(ctx context.Context, path string) (string, error)
	// ReadBytes returns the file as it is; NotFound when it no longer exists.
	ReadBytes(ctx context.Context, path string) ([]byte, error)
	// Delete removes a file; it does not fail when the file is already gone.
	Delete(ctx context.Context, path string) error
	// DeleteTaskDir removes the attachment folder of the task; it does not fail when there is none.
	DeleteTaskDir(ctx context.Context, taskID models.TaskID) error
}
