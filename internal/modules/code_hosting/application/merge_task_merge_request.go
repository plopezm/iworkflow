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
	// Only a request that carries the whole local branch lets the task skip the integration: commits made
	// after the request head would be lost. When the branch moved, or the provider did not say which commit
	// the request holds, an open request is pushed and merged and a merged one is integrated locally.
	head, err := u.remote.BranchHead(ctx, resolved.Source.RepoPath, branch)
	if err != nil {
		slog.Warn("could not read the task branch", "task_id", taskID, "branch", branch, "error", err)
		if found.IsMerged() {
			// Unreadable branch: the local integration reports the real problem, if there is one.
			return inspectedRequest{resolved: resolved}, nil //nolint:nilerr // Fall back to the local integration.
		}
		return inspectedRequest{resolved: resolved, open: found}, nil //nolint:nilerr // The push reports the real problem, if there is one.
	}
	if found.HeadSHA != "" && found.HeadSHA == head {
		return inspectedRequest{resolved: resolved, atTip: true, open: openOnly(found)}, nil
	}
	if found.IsMerged() {
		return inspectedRequest{resolved: resolved}, nil
	}
	return inspectedRequest{resolved: resolved, open: found}, nil
}

// openOnly returns the request when it is open and nil when it is merged.
func openOnly(request *models.MergeRequest) *models.MergeRequest {
	if request.IsOpen() {
		return request
	}
	return nil
}

// Preview reports, without pushing or merging, whether the latest merge request of the branch is open and
// and whether it already carries the current tip of the branch, merged or not.
func (u *MergeTaskMergeRequest) Preview(ctx context.Context, taskID string) (open bool, atTip bool, err error) {
	inspected, err := u.inspect(ctx, taskID)
	if err != nil {
		return false, false, err
	}
	return inspected.open != nil, inspected.atTip, nil
}

// ExecuteIfOpenOrDetectMerged is ExecuteIfOpen that also reports, without pushing or merging, whether the
// latest merge request of the branch, open or merged, already carries the current tip of the branch. An open
// request in that state is left open: only the local worktree needs cleaning.
func (u *MergeTaskMergeRequest) ExecuteIfOpenOrDetectMerged(ctx context.Context, taskID string) (*models.MergeRequestStatus, bool, error) {
	inspected, err := u.inspect(ctx, taskID)
	if err != nil {
		return nil, false, err
	}
	if inspected.atTip {
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
