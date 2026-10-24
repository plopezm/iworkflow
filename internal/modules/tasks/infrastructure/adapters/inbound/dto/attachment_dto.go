package dto

import (
	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskAttachmentDTO is the metadata of a stored image as the board receives it. The bytes are not in the
// payload: the browser fetches them from URL, and caches them, since an attachment never changes.
type TaskAttachmentDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MimeType  string `json:"mime_type"`
	SizeBytes uint64 `json:"size_bytes"`
	URL       string `json:"url"`
}

// AttachmentURL is the public path of the content of one image of a task.
func AttachmentURL(taskID string, attachmentID string) string {
	return "/api/tasks/" + taskID + "/attachments/" + attachmentID
}

// TaskAttachmentFromDomain maps the stored metadata of an image of the task taskID.
func TaskAttachmentFromDomain(taskID string, attachment models.StoredAttachment) TaskAttachmentDTO {
	return TaskAttachmentDTO{
		ID:        attachment.ID,
		Name:      attachment.Name,
		MimeType:  attachment.MimeType,
		SizeBytes: attachment.SizeBytes,
		URL:       AttachmentURL(taskID, attachment.ID),
	}
}

// ImageDTOFromDomain maps an image received in a request back to its form. It is the reverse of the
// conversion done when a request is decoded, and it keeps the same JSON names.
func ImageDTOFromDomain(image models.ImageAttachment) ImageDTO {
	return ImageDTO{Name: image.Name, MimeType: image.MimeType, Data: image.Data}
}
