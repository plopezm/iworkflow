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

// MergePreviewResult is what the merge action of a task will do.
type MergePreviewResult struct {
	// CleanupOnly is true when a merge request exists on the provider: no merge runs and only the worktree is removed.
	CleanupOnly bool
	// BranchDiffers is true when, besides, the task branch has commits the merge request does not carry: they are lost.
	BranchDiffers bool
}

// Execute reports whether merging the task only cleans up because a merge request, merged or still open, exists, and
// whether the branch holds commits that request does not carry. A task that cannot be merged, or whose provider cannot
// be read, reports neither: the action then keeps its merge label.
func (u *PreviewMerge) Execute(ctx context.Context, id models.TaskID) (MergePreviewResult, error) {
	if u.remote == nil {
		return MergePreviewResult{}, nil
	}
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return MergePreviewResult{}, err
	}
	if !task.Status.CanMerge() {
		return MergePreviewResult{}, nil
	}
	outcome, err := u.remote.PreviewMergeOpenRequest(ctx, id.String())
	if err != nil {
		return MergePreviewResult{}, fmt.Errorf("preview the merge of the task: %w", err)
	}
	return MergePreviewResult{
		CleanupOnly:   outcome == outbound.RemoteAlreadyMerged || outcome == outbound.RemoteBranchDiffers,
		BranchDiffers: outcome == outbound.RemoteBranchDiffers,
	}, nil
}
