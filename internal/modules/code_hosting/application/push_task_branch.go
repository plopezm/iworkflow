//nolint:wrapcheck // Port and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
)

// PushTaskBranch pushes the commits of the task branch (the branch of its merge request) to origin.
type PushTaskBranch struct {
	resolver *TaskProjectResolver
	remote   outbound.GitRemote
}

// NewPushTaskBranch builds the use case.
func NewPushTaskBranch(resolver *TaskProjectResolver, remote outbound.GitRemote) *PushTaskBranch {
	return &PushTaskBranch{resolver: resolver, remote: remote}
}

// Execute publishes the task branch to the project of its provider.
func (u *PushTaskBranch) Execute(ctx context.Context, taskID string) error {
	resolved, err := u.resolver.Resolve(ctx, taskID)
	if err != nil {
		return err
	}
	return u.remote.PublishBranch(ctx, resolved.Source.RepoPath, resolved.Source.Branch, resolved.Settings, resolved.Project)
}
