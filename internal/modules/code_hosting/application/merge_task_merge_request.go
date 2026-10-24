//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

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
	resolved, err := u.resolver.Resolve(ctx, taskID)
	if err != nil {
		return nil, err
	}
	branch := resolved.Source.Branch
	found, err := resolved.Client.FindLatestMergeRequest(ctx, resolved.Settings, resolved.Project, branch)
	if err != nil {
		return nil, err
	}
	if found == nil || !found.IsOpen() {
		return nil, nil
	}
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
