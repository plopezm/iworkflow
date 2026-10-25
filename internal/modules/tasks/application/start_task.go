//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// StartTask launches a backlog task: it claims the task, creates its worktree, starts the agent and sends it
// the initial prompt. When the agent cannot start, or the prompt cannot be sent, the task ends in failed and its
// worktree is kept. The claim moves the task out of backlog atomically, so two concurrent starts never both run it.
// When the worktree cannot be created, the claim is reverted and the creation error is returned.
type StartTask struct {
	lifecycle      *TaskLifecycle
	worktrees      outbound.Worktree
	runtime        outbound.AgentRuntime
	launchSettings outbound.LaunchSettings
	sink           inbound.AgentEventSink
	attachments    outbound.TaskAttachmentRepository
	storage        outbound.AttachmentStorage
	newUUID        UUIDGenerator
	clock          Clock
}

var _ inbound.StartTaskUseCase = (*StartTask)(nil)

// NewStartTask builds the use case on its ports. The sink receives the events of the started agent.
func NewStartTask(
	lifecycle *TaskLifecycle,
	worktrees outbound.Worktree,
	runtime outbound.AgentRuntime,
	launchSettings outbound.LaunchSettings,
	sink inbound.AgentEventSink,
	attachments outbound.TaskAttachmentRepository,
	storage outbound.AttachmentStorage,
	newUUID UUIDGenerator,
	clock Clock,
) *StartTask {
	return &StartTask{
		lifecycle:      lifecycle,
		worktrees:      worktrees,
		runtime:        runtime,
		launchSettings: launchSettings,
		sink:           sink,
		attachments:    attachments,
		storage:        storage,
		newUUID:        newUUID,
		clock:          clock,
	}
}

// Start implements StartTaskUseCase: the create use case and the graph trigger start tasks through it.
func (u *StartTask) Start(ctx context.Context, id models.TaskID) (models.Task, error) {
	return u.Execute(ctx, id)
}

// Execute starts the task and returns it. It fails, changing nothing, when the task is missing, is not in backlog,
// its workspace is gone, another start claimed it first, or the worktree cannot be created. A failure after the
// claim while the agent starts or the prompt is sent is not returned: the task ends in failed, and so does the result.
func (u *StartTask) Execute(ctx context.Context, id models.TaskID) (models.Task, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return models.Task{}, err
	}
	if !task.Status.CanStart() {
		return models.Task{}, invalidState("iniciar", task.Status)
	}
	workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
	if err != nil {
		return models.Task{}, err
	}

	unique, err := u.newUUID()
	if err != nil {
		return models.Task{}, err
	}
	branch := models.BranchName(task.Title, unique)
	worktreePath := u.worktrees.WorktreePath(
		models.WorkspaceDirName(workspace.Name, workspace.ID),
		models.WorktreeDirName(task.Title, unique),
	)
	// The branch chosen when the task was created wins; without one, the base branch of the workspace.
	baseBranch := task.BaseBranch
	if strings.TrimSpace(baseBranch) == "" {
		baseBranch = workspace.BaseBranch
	}

	claimed, err := u.lifecycle.Repository().ClaimForStart(ctx, id, branch, worktreePath, baseBranch)
	if err != nil {
		return models.Task{}, err
	}
	if !claimed {
		return models.Task{}, u.lifecycle.Conflict(ctx, id, "iniciar")
	}
	task.Branch, task.WorktreePath, task.BaseBranch = branch, worktreePath, baseBranch

	if err = u.worktrees.Create(ctx, workspace.Path, task.BaseBranch, task.Branch, task.WorktreePath); err != nil {
		return models.Task{}, u.abandonClaim(ctx, id, err)
	}

	if err = u.lifecycle.NotifyStatus(ctx, id, models.StatusStarting, nil); err != nil {
		return models.Task{}, err
	}
	task.Status = models.StatusStarting
	task.UpdatedAt = u.clock()

	if err = u.launch(ctx, task); err != nil {
		message := err.Error()
		if u.runtime.IsRunning(ctx, id) {
			if stopErr := u.runtime.Stop(ctx, id); stopErr != nil {
				slog.Warn("could not stop the agent after its launch failed", "task_id", id.String(), "stop_error", stopErr)
			}
		}
		if err = u.lifecycle.Transition(ctx, id, models.StatusFailed, &message); err != nil {
			return models.Task{}, err
		}
		task.Status = models.StatusFailed
		task.Error = &message
	}
	return task, nil
}

// abandonClaim gives back a task whose worktree could not be created, so it can be started again, and returns
// the creation error. When the revert fails too, the task is marked failed with the creation error instead.
func (u *StartTask) abandonClaim(ctx context.Context, id models.TaskID, createErr error) error {
	// The boolean of the revert is ignored on purpose: Rust (start_task.rs) also notifies backlog when it is false.
	if _, err := u.lifecycle.Repository().RevertStartClaim(ctx, id); err != nil {
		slog.Warn("could not revert the start claim; the task is marked failed", "task_id", id.String(), "revert_error", err, "create_error", createErr)
		message := createErr.Error()
		if transitionErr := u.lifecycle.Transition(ctx, id, models.StatusFailed, &message); transitionErr != nil {
			slog.Warn("could not mark the task failed after its worktree failed", "task_id", id.String(), "transition_error", transitionErr)
		}
		return createErr
	}
	if err := u.lifecycle.NotifyStatus(ctx, id, models.StatusBacklog, nil); err != nil {
		return err
	}
	return createErr
}

// launch prepares the launch options, starts the agent in the worktree and sends it the initial prompt. The images
// are read before the agent starts, so a missing file never leaves a running agent without its prompt.
func (u *StartTask) launch(ctx context.Context, task models.Task) error {
	prepared, err := u.launchSettings.Prepare(ctx, task.WorktreePath)
	if err != nil {
		return err
	}
	capabilities := u.runtime.RuntimeCapabilities(prepared.Runtime)
	options := prepared.WithTaskLimits(task.Limits).RestrictedTo(capabilities)
	images, err := u.loadImages(ctx, task.ID)
	if err != nil {
		return err
	}
	if err = u.runtime.Start(ctx, task.ID, task.WorktreePath, false, options, u.sink); err != nil {
		return err
	}
	return u.runtime.Send(ctx, task.ID, models.PromptCommand{Message: task.Prompt, Images: images})
}

// loadImages reads the images of the initial prompt from the storage. A file that cannot be read aborts the start.
func (u *StartTask) loadImages(ctx context.Context, taskID models.TaskID) ([]models.ImageAttachment, error) {
	stored, err := u.attachments.ListByTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	images := make([]models.ImageAttachment, 0, len(stored))
	for _, attachment := range stored {
		data, readErr := u.storage.Read(ctx, attachment.Path)
		if readErr != nil {
			return nil, werrors.Storage(fmt.Sprintf("no se pudo leer la imagen adjunta %s: %s", attachment.Name, readErr.Error()))
		}
		images = append(images, models.ImageAttachment{Name: attachment.Name, MimeType: attachment.MimeType, Data: data})
	}
	return images, nil
}
