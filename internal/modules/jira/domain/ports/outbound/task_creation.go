package outbound

import (
	"context"

	"iworkflow/internal/modules/jira/domain/models"
)

// NewLinkedTask is what the tasks feature needs to create a task from a Jira issue.
//
// Port for the conversion Jira issue -> iworkflow task. The tasks module does not exist in Go yet:
// the composition root binds this port to tasks once it lands (the Rust equivalent is TaskCreationBridge
// in src/app_state.rs).
type NewLinkedTask struct {
	WorkspaceID string
	BaseBranch  *string
	Title       string
	Prompt      string
	Priority    models.PriorityHint
	StartNow    bool
}

// CreatedLinkedTask is the task that was created.
type CreatedLinkedTask struct {
	TaskID string
	// StartError is set when StartNow was requested but the task stayed in the backlog.
	StartError *string
}

// TaskCreation creates an iworkflow task for a Jira issue.
type TaskCreation interface {
	Create(ctx context.Context, task NewLinkedTask) (CreatedLinkedTask, error)
}
