// Package outbound holds the interfaces the tasks use cases need from the outside world. The
// infrastructure implements them; the use cases only depend on these contracts.
package outbound

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
)

// TaskRepository persists tasks and their events. Every method is atomic: a retry of one call is safe.
type TaskRepository interface {
	// Insert stores a new task.
	Insert(ctx context.Context, task models.Task) error
	// Find returns nil and no error when the task does not exist.
	Find(ctx context.Context, id models.TaskID) (*models.Task, error)
	// List returns the tasks, newest first. A nil workspace means every workspace.
	List(ctx context.Context, workspaceID *string) ([]models.Task, error)
	// ListActive returns the unfinished tasks (neither merged nor discarded), newest first.
	ListActive(ctx context.Context, workspaceID *string) ([]models.Task, error)
	// ListDonePage returns a page of finished tasks (merged or discarded), newest first, ties by id.
	ListDonePage(ctx context.Context, workspaceID *string, limit int64, offset int64) ([]models.Task, error)
	// ListByStatus returns the tasks in one of the statuses.
	ListByStatus(ctx context.Context, statuses []models.TaskStatus) ([]models.Task, error)
	// UpdateStatus sets the status and its error text. Leaving RateLimited clears resume_at; a status
	// without a live agent clears the subagent counters. A missing task is NotFound.
	UpdateStatus(ctx context.Context, id models.TaskID, status models.TaskStatus, errorText *string) error
	// UpdateDetailsIfBacklog changes title, prompt, priority, limits, base branch and agent options only while the
	// task is in backlog. It returns false, changing nothing, when the task is missing or left backlog.
	UpdateDetailsIfBacklog(ctx context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string, agent models.TaskAgentOptions) (bool, error)
	// UpdateDetailsAndRelatedIfBacklog does what UpdateDetailsIfBacklog does and replaces the related workspaces with
	// workspaceIDs, in one transaction: either both change or neither does. It returns false, changing nothing, when the
	// task is missing or left backlog. A failing workspace id (for example one that does not exist) is an error.
	UpdateDetailsAndRelatedIfBacklog(ctx context.Context, id models.TaskID, title string, prompt string, priority models.TaskPriority, limits models.ResourceLimits, baseBranch string, agent models.TaskAgentOptions, workspaceIDs []string) (bool, error)
	// ClaimForStart atomically moves a backlog task to starting and fixes its plan (branch, worktree
	// path and base branch) in one conditional update. It returns false, changing nothing, when the
	// task is missing or left backlog, so two concurrent starts cannot claim the same task.
	ClaimForStart(ctx context.Context, id models.TaskID, branch string, worktreePath string, baseBranch string) (bool, error)
	// RevertStartClaim moves a starting task back to backlog. It returns false when the task is not starting.
	RevertStartClaim(ctx context.Context, id models.TaskID) (bool, error)
	// ResetToBacklog clears branch, worktree, base branch, error, dialogs and subagents and deletes the
	// events of the task, in one transaction. It applies only when the status allows a reset.
	ResetToBacklog(ctx context.Context, id models.TaskID) (bool, error)
	// AddPendingDialog adds a dialog. It is ignored when the task is missing, has no live agent, or
	// already lists the dialog (avoids duplicates and orphans caused by races).
	AddPendingDialog(ctx context.Context, id models.TaskID, dialog models.PendingDialog) error
	// RemovePendingDialog removes a dialog. It does not fail when either the task or the dialog is gone.
	RemovePendingDialog(ctx context.Context, id models.TaskID, dialogID string) error
	// ClearPendingDialogs removes every pending dialog of the task.
	ClearPendingDialogs(ctx context.Context, id models.TaskID) error
	// SetSubagentActivity sets the subagent counters while the task has a live agent. It does not fail
	// when the task is gone.
	SetSubagentActivity(ctx context.Context, id models.TaskID, activity models.SubagentActivity) error
	// RecordDiffRange stores the final commit range. It does not fail when the task is gone.
	RecordDiffRange(ctx context.Context, id models.TaskID, diffRange models.DiffRange) error
	// SetUsageLimit stores the resume schedule after a usage limit. The zero state clears it.
	SetUsageLimit(ctx context.Context, id models.TaskID, state models.UsageLimitState) error
	// DeleteIfBacklog deletes the task (and its events, by cascade) only while it is in backlog.
	DeleteIfBacklog(ctx context.Context, id models.TaskID) (bool, error)
	// DeleteInactiveByWorkspace deletes every task of a workspace in one transaction when all of them
	// are inactive, and returns their ids. When one is active it deletes nothing and returns false.
	DeleteInactiveByWorkspace(ctx context.Context, workspaceID string) ([]models.TaskID, bool, error)
	// AppendEvent persists an event and returns it with its sequence number.
	AppendEvent(ctx context.Context, id models.TaskID, kind models.TaskEventKind) (models.TaskEvent, error)
	// Events returns the persisted events with sequence greater than afterSeq, in order. A nil
	// afterSeq means from the beginning.
	Events(ctx context.Context, id models.TaskID, afterSeq *int64) ([]models.TaskEvent, error)
}
