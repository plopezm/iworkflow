package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// WorkspaceLookup is the read-only access to workspaces the tasks module needs. It is consumer-side:
// the composition root implements it with the workspaces module, so tasks never imports workspaces.
type WorkspaceLookup interface {
	// Find returns nil and no error when the workspace does not exist.
	Find(ctx context.Context, workspaceID string) (*models.WorkspaceRef, error)
	// List returns every workspace.
	List(ctx context.Context) ([]models.WorkspaceRef, error)
}

// LaunchSettings resolves the launch options (orchestrator model) for a launch in cwd, from the global
// settings of the agent_settings module. Consumer-side: implemented in the composition root.
type LaunchSettings interface {
	Prepare(ctx context.Context, cwd string) (models.LaunchOptions, error)
}

// TaskEventPublisher broadcasts task events to the live observers. Publishing does not fail.
type TaskEventPublisher interface {
	Publish(event models.TaskEvent)
}

// RemoteMergeOutcome is the result of integrating a task through its remote merge request.
type RemoteMergeOutcome uint8

// Remote merge outcomes.
const (
	// RemoteMerged means the open remote merge request was merged by the provider.
	RemoteMerged RemoteMergeOutcome = iota + 1
	// RemoteNoOpenRequest means the task has no open merge request (or no provider): it is integrated locally.
	RemoteNoOpenRequest
)

// RemoteMerge integrates a task through the repository provider (GitHub or GitLab) instead of locally.
// Consumer-side, slice E. Implemented in the composition root with the code_hosting module.
type RemoteMerge interface {
	// MergeOpenRequest pushes the pending changes of the branch to its open merge request and merges it remotely.
	MergeOpenRequest(ctx context.Context, taskID string) (RemoteMergeOutcome, error)
}
