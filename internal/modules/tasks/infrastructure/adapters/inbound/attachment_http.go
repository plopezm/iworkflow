package inbound

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// AttachmentRoute serves the content of one image of a task. The module does not mount it: the
// composition root does, next to the other routes.
const AttachmentRoute = "/api/tasks/{task_id}/attachments/{attachment_id}"

// immutableCacheControl applies because an attachment never changes: each upload gets a new id, and
// editing a task only adds or removes attachments.
const immutableCacheControl = "private, max-age=31536000, immutable"

// AttachmentHandler serves the stored images. Both identifiers must be UUIDs; anything else is a 404
// before the use case or the disk is touched.
type AttachmentHandler struct {
	getAttachment *application.GetTaskAttachment
}

// NewAttachmentHandler builds the handler over the use case.
func NewAttachmentHandler(getAttachment *application.GetTaskAttachment) *AttachmentHandler {
	return &AttachmentHandler{getAttachment: getAttachment}
}

// Serve writes the image with its stored MIME type, marked as not to be sniffed and cached for good.
func (h *AttachmentHandler) Serve(w http.ResponseWriter, r *http.Request) {
	taskID, attachmentID, ok := parseAttachmentIDs(chi.URLParam(r, "task_id"), chi.URLParam(r, "attachment_id"))
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	content, err := h.getAttachment.Execute(r.Context(), taskID, attachmentID)
	writeAttachment(w, content, err)
}

// parseAttachmentIDs reads both identifiers. The attachment id is written back in canonical form.
func parseAttachmentIDs(taskID string, attachmentID string) (models.TaskID, string, bool) {
	task, err := models.ParseTaskID(taskID)
	if err != nil {
		return models.TaskID{}, "", false
	}
	attachment, err := uuid.Parse(attachmentID)
	if err != nil {
		return models.TaskID{}, "", false
	}
	return task, attachment.String(), true
}

// writeAttachment turns the outcome of the use case into the response. A stored MIME type that is not an
// accepted image type is an internal error: the content is never served under a type the browser could run.
func writeAttachment(w http.ResponseWriter, content models.TaskAttachmentContent, err error) {
	switch {
	case errors.Is(err, werrors.NotFound("")):
		w.WriteHeader(http.StatusNotFound)
	case err != nil:
		slog.Error("could not serve an attachment", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
	case !models.IsAllowedImageMimeType(content.MimeType):
		slog.Error("a stored attachment has a content type that is not an accepted image type", "mime_type", content.MimeType)
		w.WriteHeader(http.StatusInternalServerError)
	default:
		header := w.Header()
		header.Set("Content-Type", content.MimeType)
		header.Set("Cache-Control", immutableCacheControl)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Content-Length", strconv.Itoa(len(content.Bytes)))
		w.WriteHeader(http.StatusOK)
		// A failed write means the client went away; there is nobody left to report it to.
		_, _ = w.Write(content.Bytes)
	}
}
