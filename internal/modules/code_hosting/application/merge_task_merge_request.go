//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"log/slog"

	"iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// MergeTaskMergeRequest merges the open merge request of a task on the provider. Before that it pushes
// the pending commits of the branch, so the merge request contains every change.
type MergeTaskMergeRequest struct {
	resolver *TaskProjectResolver
	remote   outbound.GitRemote
}

// NewMergeTaskMergeRequest builds the use case.
func NewMergeTaskMergeRequest(resolver *TaskProjectResolver, remote outbound.GitRemote) *MergeTaskMergeRequest {
	return &MergeTaskMergeRequest{resolver: resolver, remote: remote}
}

// Execute merges the open merge request, or fails with MergeRequestNotFound when there is none.
func (u *MergeTaskMergeRequest) Execute(ctx context.Context, taskID string) (models.MergeRequestStatus, error) {
	status, err := u.ExecuteIfOpen(ctx, taskID)
	if err != nil {
		return models.MergeRequestStatus{}, err
	}
	if status != nil {
		return *status, nil
	}
	resolved, err := u.resolver.Resolve(ctx, taskID)
	if err != nil {
		return models.MergeRequestStatus{}, err
	}
	return models.MergeRequestStatus{}, errors.MergeRequestNotFound(resolved.Source.Branch)
}

// ExecuteIfOpen is Execute, but it returns nil and no error, without pushing or merging, when the
// branch has no open merge request.
func (u *MergeTaskMergeRequest) ExecuteIfOpen(ctx context.Context, taskID string) (*models.MergeRequestStatus, error) {
	inspected, err := u.inspect(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return u.mergeInspected(ctx, inspected)
}

// inspectedRequest is what the provider says about the latest merge request of a task branch.
type inspectedRequest struct {
	resolved ResolvedTask
	// open is the latest request when it is still open, nil otherwise.
	open *models.MergeRequest
	// atTip reports that the latest request, open or merged, carries the current tip of the branch: every
	// commit of the task is already on the provider, so the task flow has nothing left to push or merge.
	atTip bool
	// differs reports that a request, open or merged, exists but does not carry the current tip of the branch: the
	// branch has commits the provider does not know.
	differs bool
}

// inspect reads, without pushing or merging, the latest merge request of the branch of the task.
func (u *MergeTaskMergeRequest) inspect(ctx context.Context, taskID string) (inspectedRequest, error) {
	resolved, err := u.resolver.Resolve(ctx, taskID)
	if err != nil {
		return inspectedRequest{}, err
	}
	branch := resolved.Source.Branch
	found, err := resolved.Client.FindLatestMergeRequest(ctx, resolved.Settings, resolved.Project, branch)
	if err != nil {
		return inspectedRequest{}, err
	}
	if found == nil || (!found.IsMerged() && !found.IsOpen()) {
		return inspectedRequest{resolved: resolved}, nil
	}
	// A request that exists, open or merged, never lets the task push or merge: only a request that carries the whole
	// local branch is at the tip, and any other case, including a branch or a request head that cannot be read, is
	// reported as differing so the task only cleans up and warns.
	head, err := u.remote.BranchHead(ctx, resolved.Source.RepoPath, branch)
	if err != nil {
		slog.Warn("could not read the task branch", "task_id", taskID, "branch", branch, "error", err)
	}
	if err == nil && found.HeadSHA != "" && found.HeadSHA == head {
		return inspectedRequest{resolved: resolved, atTip: true, open: openOnly(found)}, nil
	}
	return inspectedRequest{resolved: resolved, open: openOnly(found), differs: true}, nil
}

// openOnly returns the request when it is open and nil when it is merged.
func openOnly(request *models.MergeRequest) *models.MergeRequest {
	if request.IsOpen() {
		return request
	}
	return nil
}

// Preview reports, without pushing or merging, whether the latest merge request of the branch is open, whether it
// carries the current tip of the branch, and whether it exists but the branch has commits it does not carry.
func (u *MergeTaskMergeRequest) Preview(ctx context.Context, taskID string) (open bool, atTip bool, differs bool, err error) {
	inspected, err := u.inspect(ctx, taskID)
	if err != nil {
		return false, false, false, err
	}
	return inspected.open != nil, inspected.atTip, inspected.differs, nil
}

// ExecuteIfOpenOrDetectMerged is ExecuteIfOpen that also reports, without pushing or merging, whether the
// latest merge request of the branch, open or merged, exists: whether it carries the current tip or not, nothing is pushed
// or merged, an open request is left open and only the local worktree needs cleaning.
func (u *MergeTaskMergeRequest) ExecuteIfOpenOrDetectMerged(ctx context.Context, taskID string) (*models.MergeRequestStatus, bool, error) {
	inspected, err := u.inspect(ctx, taskID)
	if err != nil {
		return nil, false, err
	}
	if inspected.atTip || inspected.differs {
		return nil, true, nil
	}
	status, err := u.mergeInspected(ctx, inspected)
	return status, false, err
}

// mergeInspected pushes the pending commits and merges the open request found by inspect. It returns nil
// and no error, without pushing or merging, when there is no open request.
func (u *MergeTaskMergeRequest) mergeInspected(ctx context.Context, inspected inspectedRequest) (*models.MergeRequestStatus, error) {
	if inspected.open == nil {
		return nil, nil
	}
	resolved, branch, found := inspected.resolved, inspected.resolved.Source.Branch, inspected.open
	if err := u.remote.PublishBranch(ctx, resolved.Source.RepoPath, branch, resolved.Settings, resolved.Project); err != nil {
		return nil, err
	}
	merged, err := resolved.Client.MergeMergeRequest(ctx, resolved.Settings, resolved.Project, found.IID)
	if err != nil {
		return nil, err
	}
	// The merge already happened: a failure reading the pipeline must not look like a failed merge
	// (the user would retry it). The merged request is returned without its pipeline instead.
	status, err := u.resolver.StatusOf(ctx, resolved, merged)
	if err != nil {
		return &models.MergeRequestStatus{
			Provider:     resolved.Settings.Provider(),
			ProjectPath:  resolved.Project,
			MergeRequest: merged,
		}, nil
	}
	return &status, nil
}
