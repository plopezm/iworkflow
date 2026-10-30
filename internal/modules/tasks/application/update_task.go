//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// UpdateInput is the editable data of a backlog task, with the image changes of the same form.
type UpdateInput struct {
	Title      string
	Prompt     string
	Priority   models.TaskPriority
	Limits     models.ResourceLimits
	Agent      models.TaskAgentOptions
	BaseBranch *string
	Images     []models.ImageAttachment
	RemoveIDs  []string
	// RelatedProjectIDs replaces the stored related projects; an empty list clears them.
	RelatedProjectIDs []string
}

// UpdateTask edits the title, prompt, priority, limits and base branch of a task in backlog (same
// validation as on creation), removes the images asked for and adds the new ones.
type UpdateTask struct {
	lifecycle   *TaskLifecycle
	attachments outbound.TaskAttachmentRepository
	storage     outbound.AttachmentStorage
	newUUID     UUIDGenerator
}

// NewUpdateTask builds the use case.
func NewUpdateTask(lifecycle *TaskLifecycle, attachments outbound.TaskAttachmentRepository, storage outbound.AttachmentStorage, newUUID UUIDGenerator) *UpdateTask {
	return &UpdateTask{lifecycle: lifecycle, attachments: attachments, storage: storage, newUUID: newUUID}
}

// Execute applies the update and returns the task as it now is.
func (u *UpdateTask) Execute(ctx context.Context, id models.TaskID, input UpdateInput) (models.Task, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return models.Task{}, err
	}
	if !task.Status.CanEdit() {
		return models.Task{}, invalidState("editar", task.Status)
	}
	validated, err := models.NewTask{
		WorkspaceID:       task.WorkspaceID,
		Title:             input.Title,
		Prompt:            input.Prompt,
		Priority:          input.Priority,
		Limits:            input.Limits,
		Agent:             input.Agent,
		BaseBranch:        input.BaseBranch,
		RelatedProjectIDs: input.RelatedProjectIDs,
	}.Validated()
	if err != nil {
		return models.Task{}, err
	}
	// Read-only check before the write: a missing related workspace must not reach the transaction.
	if err := ensureWorkspacesExist(ctx, u.lifecycle, validated.RelatedProjectIDs); err != nil {
		return models.Task{}, err
	}
	// As on creation: without a chosen branch, the workspace base branch.
	baseBranch := ""
	if validated.BaseBranch != nil {
		baseBranch = *validated.BaseBranch
	} else {
		workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
		if err != nil {
			return models.Task{}, err
		}
		baseBranch = workspace.BaseBranch
	}
	images, err := models.ValidateAllImages(input.Images)
	if err != nil {
		return models.Task{}, err
	}
	var removableIDs []string
	if len(images) > 0 || len(input.RemoveIDs) > 0 {
		existing, err := u.attachments.ListByTask(ctx, id)
		if err != nil {
			return models.Task{}, err
		}
		for _, attachment := range existing {
			if slices.Contains(input.RemoveIDs, attachment.ID) {
				removableIDs = append(removableIDs, attachment.ID)
			}
		}
		remaining := len(existing) - len(removableIDs)
		if remaining+len(images) > models.MaxImagesPerMessage {
			return models.Task{}, werrors.InvalidAttachment(fmt.Sprintf("máximo %d imágenes por tarea", models.MaxImagesPerMessage))
		}
	}
	// The details and the related projects are one write: a failure leaves both as they were.
	updated, err := u.lifecycle.Repository().UpdateDetailsAndRelatedIfBacklog(ctx, id, validated.Title, validated.Prompt, validated.Priority, validated.Limits, baseBranch, validated.Agent, validated.RelatedProjectIDs)
	if err != nil {
		return models.Task{}, err
	}
	if !updated {
		return models.Task{}, u.lifecycle.Conflict(ctx, id, "editar")
	}
	if len(removableIDs) > 0 {
		if err := u.removeAttachments(ctx, id, removableIDs); err != nil {
			return models.Task{}, err
		}
	}
	if len(images) > 0 {
		if err := u.storeNewImages(ctx, id, images); err != nil {
			return models.Task{}, err
		}
	}
	if err := u.lifecycle.NotifyChanged(ctx, id); err != nil {
		return models.Task{}, err
	}
	task.Title = validated.Title
	task.Prompt = validated.Prompt
	task.Priority = validated.Priority
	task.Limits = validated.Limits
	task.Agent = validated.Agent
	task.BaseBranch = baseBranch
	return task, nil
}

// storeNewImages writes the new files first and then their metadata. Only the files written by this
// call are removed when something fails.
func (u *UpdateTask) storeNewImages(ctx context.Context, id models.TaskID, images []models.ImageAttachment) error {
	var savedPaths []string
	err := u.saveAndRegister(ctx, id, images, &savedPaths)
	if err != nil {
		for _, path := range savedPaths {
			if cleanupErr := u.storage.Delete(ctx, path); cleanupErr != nil {
				slog.Warn("could not delete an attachment written before a failure", "task_id", id.String(), "path", path, "cleanup_error", cleanupErr)
			}
		}
	}
	return err
}

func (u *UpdateTask) saveAndRegister(ctx context.Context, id models.TaskID, images []models.ImageAttachment, savedPaths *[]string) error {
	stored := make([]models.StoredAttachment, 0, len(images))
	for _, image := range images {
		attachmentUUID, err := u.newUUID()
		if err != nil {
			return err
		}
		attachmentID := attachmentUUID.String()
		path, err := u.storage.Save(ctx, id, attachmentID, image)
		if err != nil {
			return err
		}
		*savedPaths = append(*savedPaths, path)
		stored = append(stored, models.StoredAttachment{
			ID:        attachmentID,
			Name:      image.Name,
			MimeType:  image.MimeType,
			Path:      path,
			SizeBytes: uint64(image.DecodedLen()),
		})
	}
	return u.attachments.InsertMany(ctx, id, stored)
}

// removeAttachments deletes the metadata and, on a best-effort basis, the files of the removed images.
func (u *UpdateTask) removeAttachments(ctx context.Context, id models.TaskID, ids []string) error {
	removed, err := u.attachments.Remove(ctx, id, ids)
	if err != nil {
		return err
	}
	for _, attachment := range removed {
		if deleteErr := u.storage.Delete(ctx, attachment.Path); deleteErr != nil {
			slog.Warn("could not delete the file of a removed attachment", "task_id", id.String(), "path", attachment.Path, "error", deleteErr)
		}
	}
	return nil
}
