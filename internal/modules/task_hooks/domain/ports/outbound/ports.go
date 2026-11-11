// Package outbound holds the ports the task_hooks use cases need from the infrastructure.
package outbound

import (
	"context"
	"time"

	"iworkflow/internal/modules/task_hooks/domain/models"
)

// HookRepository stores the hooks of the library. Get and Delete return errors.ErrHookNotFound for an unknown id.
// Deleting a hook deletes its assignments; its run records stay.
type HookRepository interface {
	List(ctx context.Context) ([]models.Hook, error)
	Get(ctx context.Context, id string) (models.Hook, error)
	// Save inserts a new hook or updates an existing one.
	Save(ctx context.Context, hook models.Hook) error
	Delete(ctx context.Context, id string) error
}

// AssignmentRepository stores which hook runs for which workspace, event and phase. Get and Delete return
// errors.ErrAssignmentNotFound for an unknown id. Lists come in position order.
type AssignmentRepository interface {
	// ListByWorkspace returns every assignment of a workspace with its hook, ordered by event, phase and position.
	ListByWorkspace(ctx context.Context, workspaceID string) ([]models.AssignedHook, error)
	// ListFor returns the assignments of a workspace attached to one event and phase, with their hooks.
	ListFor(ctx context.Context, workspaceID string, event models.Event, phase models.Phase) ([]models.AssignedHook, error)
	// ListByHook returns the assignments of one hook.
	ListByHook(ctx context.Context, hookID string) ([]models.Assignment, error)
	Get(ctx context.Context, id string) (models.Assignment, error)
	// Save inserts a new assignment at the end of its workspace, event and phase, or updates an existing one keeping
	// its position. Assigning the same hook twice to one workspace, event and phase is errors.ErrInvalidHook.
	Save(ctx context.Context, assignment models.Assignment) error
	Delete(ctx context.Context, id string) error
	// Reorder sets the position of the assignments of one workspace, event and phase to the order of ids.
	Reorder(ctx context.Context, workspaceID string, event models.Event, phase models.Phase, ids []string) error
}

// SkipRepository stores which assignments are disabled for one task.
type SkipRepository interface {
	SetSkipped(ctx context.Context, taskID string, assignmentID string, skipped bool) error
	SkippedAssignmentIDs(ctx context.Context, taskID string) ([]string, error)
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

// RunFilter narrows the run history. Empty fields do not filter; FailedOnly keeps the runs that did not end ok
// or skipped. Limit bounds the result (newest first).
type RunFilter struct {
	HookID      string
	WorkspaceID string
	FailedOnly  bool
	Limit       int
}

// RunRepository stores the run records of the tasks. Save inserts or updates by id; ListByTask returns the
// records of one task in start order; List returns the history, newest first; LastFailures returns the latest
// failed record of each hook that has one, by hook id. FailStaleRunning closes every record still running, as
// errored at finishedAt, and returns how many it closed; it is for the start of the process, when no run of this
// process can be alive.
type RunRepository interface {
	Save(ctx context.Context, run models.HookRun) error
	ListByTask(ctx context.Context, taskID string) ([]models.HookRun, error)
	List(ctx context.Context, filter RunFilter) ([]models.HookRun, error)
	LastFailures(ctx context.Context) (map[string]models.HookRun, error)
	FailStaleRunning(ctx context.Context, finishedAt time.Time) (int64, error)
}

// Clock gives the current time.
type Clock interface {
	Now() time.Time
}

// WorkspaceLookup gives the working directory of a workspace. The test of a hook runs in it, so the module does not
// read the workspaces module itself; the composition root binds this port to it.
type WorkspaceLookup interface {
	Path(ctx context.Context, workspaceID string) (string, error)
}

// PathBrowser lists a folder of the machine, so the editor of a hook can offer a file or a folder to pick. An empty
// dir is the home folder of the user.
type PathBrowser interface {
	List(ctx context.Context, dir string) (models.Listing, error)
}
