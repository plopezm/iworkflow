// Package outbound holds the ports the task_hooks use cases need from the infrastructure.
package outbound

import (
	"context"
	"time"

	"iworkflow/internal/modules/task_hooks/domain/models"
)

// HookRepository stores the hooks. Get and Delete return errors.ErrHookNotFound for an unknown id. Lists
// return the hooks of a workspace in position order.
type HookRepository interface {
	List(ctx context.Context, workspaceID string) ([]models.Hook, error)
	ListFor(ctx context.Context, workspaceID string, event models.Event, phase models.Phase) ([]models.Hook, error)
	Get(ctx context.Context, id string) (models.Hook, error)
	// Save inserts a new hook at the end of its workspace list, or updates an existing one keeping its position.
	Save(ctx context.Context, hook models.Hook) error
	Delete(ctx context.Context, id string) error
	// Reorder sets the position of the workspace hooks to the order of ids.
	Reorder(ctx context.Context, workspaceID string, ids []string) error
}

// SkipRepository stores which hooks are disabled for one task.
type SkipRepository interface {
	SetSkipped(ctx context.Context, taskID string, hookID string, skipped bool) error
	SkippedHookIDs(ctx context.Context, taskID string) ([]string, error)
}

// IDGenerator creates the identifiers of new hooks.
type IDGenerator interface {
	NewID() string
}

// ScriptRunner runs a script hook in the worktree of a task and waits for it, bounded by the hook timeout.
// A script that exits non-zero or exceeds the timeout is a RunOutcome, not an error; the error is only for
// problems that stop the run from being attempted (bad template, missing directory).
type ScriptRunner interface {
	Run(ctx context.Context, hook models.Hook, run models.RunContext) (models.RunOutcome, error)
}

// AgentRunner runs an agent hook: one ephemeral agent session in the worktree of the task, bounded by the
// hook timeout. The verdict of the agent maps to RunOK or RunFailed; the same error rule as ScriptRunner applies.
type AgentRunner interface {
	Run(ctx context.Context, hook models.Hook, run models.RunContext) (models.RunOutcome, error)
}

// RunRepository stores the run records of the tasks. Save inserts or updates by id; ListByTask returns the
// records of one task in start order.
type RunRepository interface {
	Save(ctx context.Context, run models.HookRun) error
	ListByTask(ctx context.Context, taskID string) ([]models.HookRun, error)
}

// Clock gives the current time.
type Clock interface {
	Now() time.Time
}
