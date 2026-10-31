//nolint:wrapcheck // Port errors and domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/inbound"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// GetTaskDiff returns the changes of a task against its base. While the worktree exists the live diff is
// read, so uncommitted changes show up. A merged task is regenerated from the commit range recorded when
// it was integrated.
type GetTaskDiff struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
}

// NewGetTaskDiff builds the use case.
func NewGetTaskDiff(lifecycle *TaskLifecycle, worktrees outbound.Worktree) *GetTaskDiff {
	return &GetTaskDiff{lifecycle: lifecycle, worktrees: worktrees}
}

// Execute returns the diff of the task.
func (u *GetTaskDiff) Execute(ctx context.Context, id models.TaskID) (models.TaskDiff, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return models.TaskDiff{}, err
	}
	if task.Status == models.StatusMerged && task.DiffRange != nil {
		workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
		if err != nil {
			return models.TaskDiff{}, err
		}
		return u.worktrees.DiffRange(ctx, workspace.Path, *task.DiffRange)
	}
	if task.Status.IsClosed() {
		return models.TaskDiff{}, invalidState("ver diff", task.Status)
	}
	workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
	if err != nil {
		return models.TaskDiff{}, err
	}
	// With the agent working, the changes are not committed yet: the worktree itself is read.
	if u.worktrees.Exists(ctx, task.WorktreePath) {
		return u.worktrees.DiffLive(ctx, task.WorktreePath, task.BaseBranch)
	}
	return u.worktrees.Diff(ctx, workspace.Path, task.BaseBranch, task.Branch)
}

// GetTaskCommittedDiff returns the commits of a task against its base, without the uncommitted changes of its
// worktree. A merged task is regenerated from the commit range recorded when it was integrated.
type GetTaskCommittedDiff struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
}

// NewGetTaskCommittedDiff builds the use case.
func NewGetTaskCommittedDiff(lifecycle *TaskLifecycle, worktrees outbound.Worktree) *GetTaskCommittedDiff {
	return &GetTaskCommittedDiff{lifecycle: lifecycle, worktrees: worktrees}
}

// Execute returns the committed diff of the task.
func (u *GetTaskCommittedDiff) Execute(ctx context.Context, id models.TaskID) (models.TaskDiff, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return models.TaskDiff{}, err
	}
	if task.Status == models.StatusMerged && task.DiffRange != nil {
		workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
		if err != nil {
			return models.TaskDiff{}, err
		}
		return u.worktrees.DiffRange(ctx, workspace.Path, *task.DiffRange)
	}
	if task.Status.IsClosed() {
		return models.TaskDiff{}, invalidState("ver diff", task.Status)
	}
	if u.worktrees.Exists(ctx, task.WorktreePath) {
		return u.worktrees.DiffCommitted(ctx, task.WorktreePath, task.BaseBranch)
	}
	workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
	if err != nil {
		return models.TaskDiff{}, err
	}
	return u.worktrees.Diff(ctx, workspace.Path, task.BaseBranch, task.Branch)
}

// MergeTask integrates the branch of a task into its base after the manual review, and removes the
// worktree. A conflict leaves the task in Conflict with its worktree. When the task has an open merge
// request in the provider, that request is merged remotely (after pushing the pending changes) instead.
type MergeTask struct {
	lifecycle  *TaskLifecycle
	worktrees  outbound.Worktree
	successors inbound.SuccessorTrigger
	remote     outbound.RemoteMerge
}

// NewMergeTask builds the use case. The successor trigger and the remote merge are optional: nil means
// the task has no graph successors to start, or no provider to merge through.
func NewMergeTask(lifecycle *TaskLifecycle, worktrees outbound.Worktree, successors inbound.SuccessorTrigger, remote outbound.RemoteMerge) *MergeTask {
	return &MergeTask{lifecycle: lifecycle, worktrees: worktrees, successors: successors, remote: remote}
}

// Execute integrates the task and returns the outcome of the merge.
func (u *MergeTask) Execute(ctx context.Context, id models.TaskID) (models.MergeOutcome, error) {
	task, err := u.lifecycle.Load(ctx, id)
	if err != nil {
		return models.MergeOutcome{}, err
	}
	if !task.Status.CanMerge() {
		return models.MergeOutcome{}, invalidState("integrar", task.Status)
	}
	workspace, err := u.lifecycle.Workspace(ctx, task.WorkspaceID)
	if err != nil {
		return models.MergeOutcome{}, err
	}
	message := "Merge tarea pi: " + task.Title
	// Pinning the range is best effort: a failure does not stop the merge, it only leaves a note.
	pinRef := "refs/iworkflow/tasks/" + id.String()
	var notes []string
	rng, pinErr := u.worktrees.PinRange(ctx, workspace.Path, task.BaseBranch, task.Branch, pinRef)
	var pinned *models.DiffRange
	if pinErr != nil {
		notes = append(notes, fmt.Sprintf("no se pudo fijar el rango del diff: %v", pinErr))
	} else {
		pinned = &rng
	}
	remoteMerged := false
	if u.remote != nil {
		outcome, err := u.remote.MergeOpenRequest(ctx, id.String())
		if err != nil {
			return models.MergeOutcome{}, err
		}
		remoteMerged = outcome == outbound.RemoteMerged
	}
	outcome := models.MergedOutcome()
	if !remoteMerged {
		outcome, err = u.worktrees.Merge(ctx, workspace.Path, task.BaseBranch, task.Branch, message)
		if err != nil {
			return models.MergeOutcome{}, err
		}
	}
	if outcome.Conflict {
		detail := "conflictos en: " + strings.Join(outcome.Files, ", ")
		if err := u.lifecycle.Transition(ctx, id, models.StatusConflict, &detail); err != nil {
			return models.MergeOutcome{}, err
		}
		return outcome, nil
	}
	if pinned != nil {
		if err := u.lifecycle.Repository().RecordDiffRange(ctx, id, *pinned); err != nil {
			notes = append(notes, fmt.Sprintf("no se pudo guardar el rango del diff: %v", err))
		}
	}
	if err := u.worktrees.Remove(ctx, workspace.Path, task.WorktreePath, task.Branch); err != nil {
		notes = append(notes, fmt.Sprintf("integrada, pero no se pudo limpiar: %v", err))
	}
	var errorText *string
	if len(notes) > 0 {
		joined := strings.Join(notes, "; ")
		errorText = &joined
	}
	if err := u.lifecycle.Transition(ctx, id, models.StatusMerged, errorText); err != nil {
		return models.MergeOutcome{}, err
	}
	// The background loop would cover a missed trigger; a failure here does not undo the merge.
	if u.successors != nil {
		if _, err := u.successors.Trigger(ctx); err != nil {
			slog.Warn("could not evaluate the graph after integrating the task", "task_id", id.String(), "error", err)
		}
	}
	return outcome, nil
}

// SweepOrphanWorktrees deletes the folders under the worktrees root that no open task owns. It runs at
// start-up. The keep set is every open task's worktree path: closed tasks and tasks unknown to the database
// are the leftovers.
type SweepOrphanWorktrees struct {
	repository outbound.TaskRepository
	worktrees  outbound.Worktree
}

// NewSweepOrphanWorktrees builds the use case.
func NewSweepOrphanWorktrees(repository outbound.TaskRepository, worktrees outbound.Worktree) *SweepOrphanWorktrees {
	return &SweepOrphanWorktrees{repository: repository, worktrees: worktrees}
}

// Execute sweeps the leftovers and returns how many folders were deleted.
func (u *SweepOrphanWorktrees) Execute(ctx context.Context) (int, error) {
	var open []models.TaskStatus
	for _, status := range models.AllTaskStatuses {
		if !status.IsClosed() {
			open = append(open, status)
		}
	}
	tasks, err := u.repository.ListByStatus(ctx, open)
	if err != nil {
		return 0, err
	}
	keep := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if task.WorktreePath != "" {
			keep = append(keep, task.WorktreePath)
		}
	}
	return u.worktrees.SweepOrphans(ctx, keep)
}

// ListWorkspaceBranches returns the branches of a workspace repository, to base a task on.
type ListWorkspaceBranches struct {
	lifecycle *TaskLifecycle
	worktrees outbound.Worktree
}

// NewListWorkspaceBranches builds the use case.
func NewListWorkspaceBranches(lifecycle *TaskLifecycle, worktrees outbound.Worktree) *ListWorkspaceBranches {
	return &ListWorkspaceBranches{lifecycle: lifecycle, worktrees: worktrees}
}

// Execute lists the branches of the workspace.
func (u *ListWorkspaceBranches) Execute(ctx context.Context, workspaceID string) ([]models.BranchOption, error) {
	workspace, err := u.lifecycle.Workspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return u.worktrees.ListBranches(ctx, workspace.Path)
}
