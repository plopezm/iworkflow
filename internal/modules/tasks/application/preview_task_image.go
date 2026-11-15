//nolint:wrapcheck // Domain errors are returned unwrapped: the handler maps them to their user-visible text.
package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// PreviewTaskImage reads an image that the agent mentioned in the chat so the page can show it in a lightbox. A
// relative path is resolved against the worktree of the task, like OpenTaskPath does.
type PreviewTaskImage struct {
	lifecycle *TaskLifecycle
	reader    outbound.ImageReader
	home      string
}

// NewPreviewTaskImage builds the use case. home is the home folder that a leading ~ stands for.
func NewPreviewTaskImage(lifecycle *TaskLifecycle, reader outbound.ImageReader, home string) *PreviewTaskImage {
	return &PreviewTaskImage{lifecycle: lifecycle, reader: reader, home: home}
}

// Execute resolves the path and reads the image.
func (u *PreviewTaskImage) Execute(ctx context.Context, id models.TaskID, raw string) (models.PreviewImage, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return models.PreviewImage{}, err
	}
	worktree := ""
	if task.Status.HasWorktree() {
		worktree = task.WorktreePath
	}
	path, contentType, err := models.ResolvePreviewImage(raw, worktree, u.home)
	if err != nil {
		return models.PreviewImage{}, err
	}
	data, err := u.reader.ReadImage(ctx, path, models.MaxPreviewImageBytes)
	if err != nil {
		return models.PreviewImage{}, fmt.Errorf("read image %s: %w", path, err)
	}
	return models.PreviewImage{ContentType: contentType, Data: data}, nil
}
