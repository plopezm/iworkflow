//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/models"
)

// GetTaskMergeRequest reads the merge request of a task and its latest pipeline.
type GetTaskMergeRequest struct {
	resolver *TaskProjectResolver
}

// NewGetTaskMergeRequest builds the use case.
func NewGetTaskMergeRequest(resolver *TaskProjectResolver) *GetTaskMergeRequest {
	return &GetTaskMergeRequest{resolver: resolver}
}

// Execute returns nil and no error when the task has no merge request yet.
func (u *GetTaskMergeRequest) Execute(ctx context.Context, taskID string) (*models.MergeRequestStatus, error) {
	resolved, err := u.resolver.Resolve(ctx, taskID)
	if err != nil {
		return nil, err
	}
	found, err := resolved.Client.FindLatestMergeRequest(ctx, resolved.Settings, resolved.Project, resolved.Source.Branch)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	status, err := u.resolver.StatusOf(ctx, resolved, *found)
	if err != nil {
		return nil, err
	}
	return &status, nil
}
