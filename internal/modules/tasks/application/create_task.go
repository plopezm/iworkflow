//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// WorktreeLayout is the part of the worktree port creation needs: where a task's worktree folder goes.
// Consumer-side: the full git port (slice E) satisfies it.
type WorktreeLayout interface {
	WorktreePath(workspaceName string, dirName string) string
}

// CreateTask creates a task in backlog (no worktree, no agent). When StartNow is set it delegates to the
// start use case. If the start fails, the task still exists in backlog and the error comes back in StartError.
type CreateTask struct {
	lifecycle   *TaskLifecycle
	worktrees   WorktreeLayout
	startTask   inbound.StartTaskUseCase
	attachments outbound.TaskAttachmentRepository
	storage     outbound.AttachmentStorage
	newUUID     UUIDGenerator
	clock       Clock
}

// NewCreateTask builds the use case.
func NewCreateTask(
	lifecycle *TaskLifecycle,
	worktrees WorktreeLayout,
	startTask inbound.StartTaskUseCase,
	attachments outbound.TaskAttachmentRepository,
	storage outbound.AttachmentStorage,
	newUUID UUIDGenerator,
	clock Clock,
) *CreateTask {
	return &CreateTask{
		lifecycle:   lifecycle,
		worktrees:   worktrees,
		startTask:   startTask,
		attachments: attachments,
		storage:     storage,
		newUUID:     newUUID,
		clock:       clock,
	}
}

// Execute validates the input, stores the task and its images, and optionally starts it.
func (u *CreateTask) Execute(ctx context.Context, input models.NewTask) (models.CreatedTask, error) {
	validated, err := input.Validated()
	if err != nil {
		return models.CreatedTask{}, err
	}
	workspace, err := u.lifecycle.Workspace(ctx, validated.WorkspaceID)
	if err != nil {
		return models.CreatedTask{}, err
	}
	taskUUID, err := u.newUUID()
	if err != nil {
		return models.CreatedTask{}, err
	}
	unique, err := u.newUUID()
	if err != nil {
		return models.CreatedTask{}, err
	}
	id := models.TaskIDFromUUID(taskUUID)
	baseBranch := workspace.BaseBranch
	if validated.BaseBranch != nil {
		baseBranch = *validated.BaseBranch
	}
	now := u.clock()
	task := models.Task{
		ID:          id,
		WorkspaceID: workspace.ID,
		Title:       validated.Title,
		Prompt:      validated.Prompt,
		Branch:      models.BranchName(validated.Title, unique),
		BaseBranch:  baseBranch,
		WorktreePath: u.worktrees.WorktreePath(
			models.WorkspaceDirName(workspace.Name, workspace.ID),
			models.WorktreeDirName(validated.Title, unique),
		),
		Status:    models.StatusBacklog,
		Priority:  validated.Priority,
		Limits:    validated.Limits,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := u.lifecycle.Repository().Insert(ctx, task); err != nil {
		return models.CreatedTask{}, err
	}
	if len(validated.Images) > 0 {
		if err := u.storeImages(ctx, id, validated.Images); err != nil {
			if _, cleanupErr := u.lifecycle.Repository().DeleteIfBacklog(ctx, id); cleanupErr != nil {
				slog.Warn("could not delete the task after its images failed to save", "task_id", id.String(), "cleanup_error", cleanupErr, "save_error", err)
			}
			return models.CreatedTask{}, err
		}
	}
	if err := u.lifecycle.Transition(ctx, id, models.StatusBacklog, nil); err != nil {
		return models.CreatedTask{}, err
	}
	if !validated.StartNow {
		return models.CreatedTask{Task: task}, nil
	}
	started, startErr := u.startTask.Start(ctx, id)
	if startErr != nil {
		return models.CreatedTask{Task: task, StartError: startErr}, nil
	}
	return models.CreatedTask{Task: started}, nil
}

// storeImages writes the files first and then their metadata. When anything fails, the files written
// by this call are removed with the task folder.
func (u *CreateTask) storeImages(ctx context.Context, id models.TaskID, images []models.ImageAttachment) error {
	err := u.saveAndRegister(ctx, id, images)
	if err != nil {
		if cleanupErr := u.storage.DeleteTaskDir(ctx, id); cleanupErr != nil {
			slog.Warn("could not delete the attachments written before a failure", "task_id", id.String(), "cleanup_error", cleanupErr)
		}
	}
	return err
}

func (u *CreateTask) saveAndRegister(ctx context.Context, id models.TaskID, images []models.ImageAttachment) error {
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
