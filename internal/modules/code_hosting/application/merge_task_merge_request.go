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
	status, _, err := u.ExecuteIfOpenOrDetectMerged(ctx, taskID)
	return status, err
}

// ExecuteIfOpenOrDetectMerged is ExecuteIfOpen that also reports, without pushing or merging, whether the
// latest merge request of the branch was already merged on the provider with the current tip of the branch.
func (u *MergeTaskMergeRequest) ExecuteIfOpenOrDetectMerged(ctx context.Context, taskID string) (*models.MergeRequestStatus, bool, error) {
	resolved, err := u.resolver.Resolve(ctx, taskID)
	if err != nil {
		return nil, false, err
	}
	branch := resolved.Source.Branch
	found, err := resolved.Client.FindLatestMergeRequest(ctx, resolved.Settings, resolved.Project, branch)
	if err != nil {
		return nil, false, err
	}
	if found != nil && found.IsMerged() {
		// Only a request that merged the whole local branch lets the task skip the local integration:
		// commits made after the merge would be lost. When the branch moved, or the provider did not say
		// which commit it merged, the task is integrated locally instead. A branch that cannot be read
		// is treated the same way: the local integration reports the real problem, if there is one.
		head, err := u.remote.BranchHead(ctx, resolved.Source.RepoPath, branch)
		if err != nil {
			slog.Warn("could not read the task branch; integrating locally", "task_id", taskID, "branch", branch, "error", err)
			return nil, false, nil //nolint:nilerr // Unreadable branch: fall back to the local integration.
		}
		return nil, found.HeadSHA != "" && found.HeadSHA == head, nil
	}
	if found == nil || !found.IsOpen() {
		return nil, false, nil
	}
	if err := u.remote.PublishBranch(ctx, resolved.Source.RepoPath, branch, resolved.Settings, resolved.Project); err != nil {
		return nil, false, err
	}
	merged, err := resolved.Client.MergeMergeRequest(ctx, resolved.Settings, resolved.Project, found.IID)
	if err != nil {
		return nil, false, err
	}
	// The merge already happened: a failure reading the pipeline must not look like a failed merge
	// (the user would retry it). The merged request is returned without its pipeline instead.
	status, err := u.resolver.StatusOf(ctx, resolved, merged)
	if err != nil {
		return &models.MergeRequestStatus{
			Provider:     resolved.Settings.Provider(),
			ProjectPath:  resolved.Project,
			MergeRequest: merged,
		}, false, nil
	}
	return &status, false, nil
}
