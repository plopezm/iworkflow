package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// PreviewMerge tells what the merge action of a task will do, so its button can say it.
type PreviewMerge struct {
	lifecycle *TaskLifecycle
	remote    outbound.RemoteMerge
}

// NewPreviewMerge builds the use case. The remote merge is optional: nil means the task always integrates locally.
func NewPreviewMerge(lifecycle *TaskLifecycle, remote outbound.RemoteMerge) *PreviewMerge {
	return &PreviewMerge{lifecycle: lifecycle, remote: remote}
}

// Execute reports whether merging the task only cleans up: its merge request, merged or still open, already carries
// the whole branch, so no merge runs and only the worktree is removed. A task that cannot be merged, or whose provider cannot be read,
// reports false: the action then keeps its merge label.
func (u *PreviewMerge) Execute(ctx context.Context, id models.TaskID) (bool, error) {
	if u.remote == nil {
		return false, nil
	}
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return false, err
	}
	if !task.Status.CanMerge() {
		return false, nil
	}
	outcome, err := u.remote.PreviewMergeOpenRequest(ctx, id.String())
	if err != nil {
		return false, fmt.Errorf("preview the merge of the task: %w", err)
	}
	return outcome == outbound.RemoteAlreadyMerged, nil
}
