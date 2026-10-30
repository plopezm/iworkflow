//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"strings"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// Bounds of the page size of the finished tasks.
const (
	minDonePageLimit int64 = 1
	maxDonePageLimit int64 = 100
)

// GetTask returns a task by id; NotFound when it does not exist.
type GetTask struct {
	lifecycle *TaskLifecycle
}

// NewGetTask builds the use case.
func NewGetTask(lifecycle *TaskLifecycle) *GetTask {
	return &GetTask{lifecycle: lifecycle}
}

// Execute loads the task.
func (u *GetTask) Execute(ctx context.Context, id models.TaskID) (models.Task, error) {
	return u.lifecycle.Load(ctx, id)
}

// ListTasks lists the unfinished tasks, newest first, optionally of one workspace.
type ListTasks struct {
	repository outbound.TaskRepository
}

// NewListTasks builds the use case.
func NewListTasks(repository outbound.TaskRepository) *ListTasks {
	return &ListTasks{repository: repository}
}

// Execute lists the active tasks. A blank workspace filter means every workspace.
func (u *ListTasks) Execute(ctx context.Context, workspaceID *string) ([]models.Task, error) {
	return u.repository.ListActive(ctx, normalizeWorkspaceFilter(workspaceID))
}

// ListDoneTasks lists one page of finished tasks, newest first, optionally of one workspace.
type ListDoneTasks struct {
	repository outbound.TaskRepository
}

// NewListDoneTasks builds the use case.
func NewListDoneTasks(repository outbound.TaskRepository) *ListDoneTasks {
	return &ListDoneTasks{repository: repository}
}

// Execute lists a page. The limit is clamped to [1, 100] and the offset to be non-negative.
func (u *ListDoneTasks) Execute(ctx context.Context, workspaceID *string, limit int64, offset int64) ([]models.Task, error) {
	return u.repository.ListDonePage(ctx, normalizeWorkspaceFilter(workspaceID), clampLimit(limit), max(offset, 0))
}

func clampLimit(limit int64) int64 {
	return min(max(limit, minDonePageLimit), maxDonePageLimit)
}

// normalizeWorkspaceFilter trims the filter and treats a blank value as "no filter".
func normalizeWorkspaceFilter(workspaceID *string) *string {
	if workspaceID == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*workspaceID)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
