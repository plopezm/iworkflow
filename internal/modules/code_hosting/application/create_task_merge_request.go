//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"strings"

	"iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// CreateTaskMergeRequestInput is the request to open a merge request for a task. A blank Title or
// Description uses the one of the task.
type CreateTaskMergeRequestInput struct {
	TaskID      string
	Title       string
	Description string
}

// CreateTaskMergeRequest opens a merge request from the task branch to its base, or reuses the open
// one. When the branch is not published yet, it is pushed first.
type CreateTaskMergeRequest struct {
	resolver *TaskProjectResolver
	remote   outbound.GitRemote
}

// NewCreateTaskMergeRequest builds the use case.
func NewCreateTaskMergeRequest(resolver *TaskProjectResolver, remote outbound.GitRemote) *CreateTaskMergeRequest {
	return &CreateTaskMergeRequest{resolver: resolver, remote: remote}
}

// Execute returns the status of the open merge request, creating it when there is none.
func (u *CreateTaskMergeRequest) Execute(ctx context.Context, input CreateTaskMergeRequestInput) (models.MergeRequestStatus, error) {
	resolved, err := u.resolver.Resolve(ctx, input.TaskID)
	if err != nil {
		return models.MergeRequestStatus{}, err
	}
	source := resolved.Source
	existing, err := resolved.Client.FindLatestMergeRequest(ctx, resolved.Settings, resolved.Project, source.Branch)
	if err != nil {
		return models.MergeRequestStatus{}, err
	}
	var mergeRequest models.MergeRequest
	if existing != nil && existing.IsOpen() {
		mergeRequest = *existing
	} else {
		mergeRequest, err = u.openMergeRequest(ctx, resolved, input)
		if err != nil {
			return models.MergeRequestStatus{}, err
		}
	}
	return u.resolver.StatusOf(ctx, resolved, mergeRequest)
}

// openMergeRequest publishes the branch when needed and creates the merge request.
func (u *CreateTaskMergeRequest) openMergeRequest(ctx context.Context, resolved ResolvedTask, input CreateTaskMergeRequestInput) (models.MergeRequest, error) {
	source := resolved.Source
	// The provider expects the branch name on the server, not the local remote reference
	// (`origin/develop`) the task was created with.
	target := remoteBranchName(source.BaseBranch)
	if target == source.Branch {
		return models.MergeRequest{}, errors.SameSourceAndTarget(target)
	}
	basePublished, err := resolved.Client.BranchExists(ctx, resolved.Settings, resolved.Project, target)
	if err != nil {
		return models.MergeRequest{}, err
	}
	if !basePublished {
		return models.MergeRequest{}, errors.BaseBranchNotPublished(target)
	}
	emptyCommitAdded, err := u.remote.EnsureCommitsAhead(ctx, source.RepoPath, source.Branch, source.BaseBranch)
	if err != nil {
		return models.MergeRequest{}, err
	}
	published, err := resolved.Client.BranchExists(ctx, resolved.Settings, resolved.Project, source.Branch)
	if err != nil {
		return models.MergeRequest{}, err
	}
	if !published || emptyCommitAdded {
		if err := u.remote.PublishBranch(ctx, source.RepoPath, source.Branch, resolved.Settings, resolved.Project); err != nil {
			return models.MergeRequest{}, err
		}
	}
	return resolved.Client.CreateMergeRequest(ctx, resolved.Settings, resolved.Project, models.NewMergeRequest{
		SourceBranch: source.Branch,
		TargetBranch: target,
		Title:        firstNonBlank(input.Title, source.Title),
		Description:  firstNonBlank(input.Description, source.Description),
	})
}

// originPrefix is the prefix of the remote reference a task can start from (`origin/develop`).
const originPrefix = "origin/"

// remoteBranchName turns `origin/develop` into `develop`; a name without the prefix is kept.
func remoteBranchName(branch string) string {
	short := strings.TrimPrefix(branch, "refs/remotes/")
	return strings.TrimPrefix(short, originPrefix)
}

// firstNonBlank returns the trimmed value, or the fallback when the value is blank.
func firstNonBlank(value string, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
