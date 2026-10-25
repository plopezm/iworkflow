package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// fakeWorktree is the git port seen by the use cases. Calls are recorded; the *Err fields inject failures.
type fakeWorktree struct {
	outbound.Worktree
	exists        bool
	pinErr        error
	mergeOutcome  models.MergeOutcome
	mergeErr      error
	removeErr     error
	removed       []string
	merged        []string
	pinned        []string
	diffCalls     []string
	liveCalls     []string
	rangeCalls    []models.DiffRange
	sweepKeep     []string
	sweepRemoved  int
	sweepErr      error
	branches      []models.BranchOption
	branchesErr   error
	branchesAsked []string
	// created records the Create calls of the start tests; createErr fails them.
	created     []worktreeCreate
	createErr   error
	existsCalls int
	// committed records the CommitAll calls as path|message; commitErr fails them.
	committed []string
	commitErr error
}

// worktreeCreate is one Create call the start tests observe.
type worktreeCreate struct {
	repo   string
	base   string
	branch string
	path   string
}

// CommitAll records the commit and answers with commitErr.
func (w *fakeWorktree) CommitAll(_ context.Context, path string, message string) (bool, error) {
	w.committed = append(w.committed, path+"|"+message)
	return true, w.commitErr
}

func (w *fakeWorktree) Exists(context.Context, string) bool {
	w.existsCalls++
	return w.exists
}

// WorktreePath places the worktree under "/wt", the way the test fixtures expect.
func (w *fakeWorktree) WorktreePath(workspaceName string, dirName string) string {
	return "/wt/" + workspaceName + "/" + dirName
}

func (w *fakeWorktree) Create(_ context.Context, repo string, base string, branch string, path string) error {
	w.created = append(w.created, worktreeCreate{repo: repo, base: base, branch: branch, path: path})
	return w.createErr
}

func (w *fakeWorktree) PinRange(_ context.Context, repo string, base string, branch string, pinRef string) (models.DiffRange, error) {
	w.pinned = append(w.pinned, repo+"|"+base+"|"+branch+"|"+pinRef)
	if w.pinErr != nil {
		return models.DiffRange{}, w.pinErr
	}
	return models.DiffRange{BaseSHA: "base-sha", HeadSHA: "head-sha"}, nil
}

func (w *fakeWorktree) Merge(_ context.Context, repo string, base string, branch string, message string) (models.MergeOutcome, error) {
	w.merged = append(w.merged, repo+"|"+base+"|"+branch+"|"+message)
	return w.mergeOutcome, w.mergeErr
}

func (w *fakeWorktree) Remove(_ context.Context, repo string, path string, branch string) error {
	w.removed = append(w.removed, repo+"|"+path+"|"+branch)
	return w.removeErr
}

func (w *fakeWorktree) Diff(_ context.Context, repo string, base string, branch string) (models.TaskDiff, error) {
	w.diffCalls = append(w.diffCalls, repo+"|"+base+"|"+branch)
	return models.TaskDiff{Patch: "committed"}, nil
}

func (w *fakeWorktree) DiffLive(_ context.Context, worktree string, base string) (models.TaskDiff, error) {
	w.liveCalls = append(w.liveCalls, worktree+"|"+base)
	return models.TaskDiff{Patch: "live"}, nil
}

func (w *fakeWorktree) DiffRange(_ context.Context, _ string, rng models.DiffRange) (models.TaskDiff, error) {
	w.rangeCalls = append(w.rangeCalls, rng)
	return models.TaskDiff{Patch: "range"}, nil
}

func (w *fakeWorktree) SweepOrphans(_ context.Context, keep []string) (int, error) {
	w.sweepKeep = keep
	return w.sweepRemoved, w.sweepErr
}

func (w *fakeWorktree) ListBranches(_ context.Context, repo string) ([]models.BranchOption, error) {
	w.branchesAsked = append(w.branchesAsked, repo)
	return w.branches, w.branchesErr
}

// fakeTrigger counts the graph evaluations of the merge.
type fakeTrigger struct {
	calls int
	err   error
}

func (f *fakeTrigger) Trigger(context.Context) (int, error) {
	f.calls++
	return 0, f.err
}

// fakeRemote is the provider bridge; open decides whether the task has an open merge request.
type fakeRemote struct {
	outcome outbound.RemoteMergeOutcome
	err     error
	asked   []string
}

func (f *fakeRemote) MergeOpenRequest(_ context.Context, taskID string) (outbound.RemoteMergeOutcome, error) {
	f.asked = append(f.asked, taskID)
	return f.outcome, f.err
}

// mergeFixture is a reviewable task on ws-1 with its worktree, and the fakes behind the merge.
func mergeFixture(status models.TaskStatus) (models.Task, *fakeRepository, *fakeWorktree) {
	task := newTask(status)
	return task, newFakeRepository(task), &fakeWorktree{mergeOutcome: models.MergedOutcome()}
}

// Rust: merge_task::merges_and_removes_worktree
func TestMergeTaskMergesAndRemovesTheWorktree(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	lifecycle, _ := newLifecycle(repo)

	outcome, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.False(t, outcome.Conflict)
	assert.Equal(t, []models.TaskStatus{models.StatusMerged}, repo.statuses)
	assert.Len(t, worktrees.removed, 1)
	assert.Equal(t, models.StatusMerged, repo.tasks[task.ID].Status)
}

// Rust: merge_task::pins_range_before_merge_and_records_it_after
func TestMergeTaskPinsTheRangeBeforeMergingAndRecordsIt(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	require.Len(t, worktrees.pinned, 1)
	assert.Contains(t, worktrees.pinned[0], "refs/iworkflow/tasks/"+task.ID.String())
	assert.Equal(t, []models.DiffRange{{BaseSHA: "base-sha", HeadSHA: "head-sha"}}, repo.diffRanges)
}

// Rust: merge_task::pin_failure_does_not_block_merge_and_leaves_note
func TestMergeTaskPinFailureDoesNotBlockTheMergeAndLeavesANote(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	worktrees.pinErr = werrors.Git("sin rama")
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Len(t, worktrees.merged, 1)
	assert.Empty(t, repo.diffRanges)
	require.NotNil(t, repo.errorTexts[len(repo.errorTexts)-1])
	assert.Contains(t, *repo.errorTexts[len(repo.errorTexts)-1], "no se pudo fijar el rango del diff")
}

// Rust: merge_task::cleanup_failure_still_marks_merged_with_error
func TestMergeTaskCleanupFailureStillMarksMergedWithAnError(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	worktrees.removeErr = werrors.Git("bloqueado")
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusMerged, repo.tasks[task.ID].Status)
	require.NotNil(t, repo.errorTexts[len(repo.errorTexts)-1])
	assert.Contains(t, *repo.errorTexts[len(repo.errorTexts)-1], "integrada, pero no se pudo limpiar")
}

// Rust: merge_task::notes_and_cleanup_error_are_combined
func TestMergeTaskNotesAndCleanupErrorAreCombined(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	worktrees.pinErr = werrors.Git("pin")
	worktrees.removeErr = werrors.Git("remove")
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	text := *repo.errorTexts[len(repo.errorTexts)-1]
	assert.Contains(t, text, "no se pudo fijar el rango del diff")
	assert.Contains(t, text, "; integrada, pero no se pudo limpiar")
}

// Rust: merge_task::record_failure_still_marks_merged_with_note
func TestMergeTaskRecordFailureStillMarksMergedWithANote(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	repo.recordRangeErr = werrors.Repository("disco")
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusMerged, repo.tasks[task.ID].Status)
	require.NotNil(t, repo.errorTexts[len(repo.errorTexts)-1])
	assert.Contains(t, *repo.errorTexts[len(repo.errorTexts)-1], "no se pudo guardar el rango del diff")
}

// Rust: merge_task::conflict_keeps_worktree_and_records_nothing
func TestMergeTaskConflictKeepsTheWorktreeAndRecordsNothing(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	worktrees.mergeOutcome = models.ConflictOutcome([]string{"README.md", "a.go"})
	lifecycle, _ := newLifecycle(repo)

	outcome, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.True(t, outcome.Conflict)
	assert.Empty(t, worktrees.removed)
	assert.Empty(t, repo.diffRanges)
	assert.Equal(t, models.StatusConflict, repo.tasks[task.ID].Status)
	assert.Equal(t, "conflictos en: README.md, a.go", *repo.errorTexts[len(repo.errorTexts)-1])
}

// Rust: merge_task::blocked_merge_does_not_change_status
func TestMergeTaskBlockedMergeDoesNotChangeStatus(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	worktrees.mergeErr = werrors.MergeBlocked("sucio")
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.MergeBlocked(""))
	assert.Empty(t, repo.statuses)
	assert.Empty(t, worktrees.removed)
}

// Rust: merge_task::rejects_task_not_reviewed
func TestMergeTaskRejectsTaskNotReviewed(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusRunning)
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, nil).Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.InvalidState("integrar", ""))
	assert.Empty(t, worktrees.merged)
}

// Rust: merge_task::merged_evaluates_successors_once
func TestMergeTaskMergedEvaluatesSuccessorsOnce(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	trigger := &fakeTrigger{}
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, trigger, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, 1, trigger.calls)
}

// Rust: merge_task::conflict_does_not_evaluate_successors
func TestMergeTaskConflictDoesNotEvaluateSuccessors(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	worktrees.mergeOutcome = models.ConflictOutcome([]string{"a.go"})
	trigger := &fakeTrigger{}
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, trigger, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Zero(t, trigger.calls)
}

// Rust: merge_task::successor_trigger_error_does_not_fail_the_merge
func TestMergeTaskSuccessorTriggerErrorDoesNotFailTheMerge(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	trigger := &fakeTrigger{err: errors.New("grafo")}
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, trigger, nil).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, models.StatusMerged, repo.tasks[task.ID].Status)
}

// Rust: merge_task::open_remote_request_is_merged_remotely_instead_of_locally
func TestMergeTaskOpenRemoteRequestIsMergedRemotelyInsteadOfLocally(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	remote := &fakeRemote{outcome: outbound.RemoteMerged}
	lifecycle, _ := newLifecycle(repo)

	outcome, err := application.NewMergeTask(lifecycle, worktrees, nil, remote).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.False(t, outcome.Conflict)
	assert.Equal(t, []string{task.ID.String()}, remote.asked)
	assert.Empty(t, worktrees.merged, "the local merge is not used")
	assert.Equal(t, models.StatusMerged, repo.tasks[task.ID].Status)
}

// Rust: merge_task::without_open_remote_request_the_local_merge_is_used
func TestMergeTaskWithoutOpenRemoteRequestUsesTheLocalMerge(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	remote := &fakeRemote{outcome: outbound.RemoteNoOpenRequest}
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, remote).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Len(t, worktrees.merged, 1)
}

// Rust: merge_task::remote_failure_aborts_without_merging_locally
func TestMergeTaskRemoteFailureAbortsWithoutMergingLocally(t *testing.T) {
	task, repo, worktrees := mergeFixture(models.StatusAwaitingReview)
	remote := &fakeRemote{err: werrors.Git("red")}
	lifecycle, _ := newLifecycle(repo)

	_, err := application.NewMergeTask(lifecycle, worktrees, nil, remote).Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.Git(""))
	assert.Empty(t, worktrees.merged)
	assert.Empty(t, repo.statuses)
}

// Rust: get_task_diff: a merged task is regenerated from its recorded range.
func TestGetTaskDiffOfMergedTaskUsesTheRecordedRange(t *testing.T) {
	task := newTask(models.StatusMerged)
	task.DiffRange = &models.DiffRange{BaseSHA: "a", HeadSHA: "b"}
	worktrees := &fakeWorktree{}
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	diff, err := application.NewGetTaskDiff(lifecycle, worktrees).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, "range", diff.Patch)
	assert.Equal(t, []models.DiffRange{{BaseSHA: "a", HeadSHA: "b"}}, worktrees.rangeCalls)
}

// Rust: get_task_diff: a closed task without a range has no diff.
func TestGetTaskDiffOfDiscardedTaskIsRefused(t *testing.T) {
	task := newTask(models.StatusDiscarded)
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	_, err := application.NewGetTaskDiff(lifecycle, &fakeWorktree{}).Execute(context.Background(), task.ID)

	assert.ErrorIs(t, err, werrors.InvalidState("ver diff", ""))
}

// Rust: get_task_diff: while the worktree exists its live diff is read (uncommitted changes included).
func TestGetTaskDiffReadsTheLiveWorktreeWhileItExists(t *testing.T) {
	task := newTask(models.StatusRunning)
	worktrees := &fakeWorktree{exists: true}
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	diff, err := application.NewGetTaskDiff(lifecycle, worktrees).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, "live", diff.Patch)
	assert.Empty(t, worktrees.diffCalls)
}

// Rust: get_task_diff: without a worktree the branch diff against the base is read.
func TestGetTaskDiffReadsTheBranchWithoutAWorktree(t *testing.T) {
	task := newTask(models.StatusFailed)
	worktrees := &fakeWorktree{exists: false}
	lifecycle, _ := newLifecycle(newFakeRepository(task))

	diff, err := application.NewGetTaskDiff(lifecycle, worktrees).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, "committed", diff.Patch)
	assert.Equal(t, []string{"/repos/demo|main|" + task.Branch}, worktrees.diffCalls)
}

// Rust: sweep_orphan_worktrees: the keep set is the worktree path of every open task; closed ones are not.
func TestSweepOrphanWorktreesKeepsOnlyOpenTasks(t *testing.T) {
	open := newTask(models.StatusRunning)
	closed := newTask(models.StatusMerged)
	repo := newFakeRepository(open, closed)
	worktrees := &fakeWorktree{sweepRemoved: 2}

	removed, err := application.NewSweepOrphanWorktrees(repo, worktrees).Execute(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 2, removed)
	assert.Equal(t, []string{open.WorktreePath}, worktrees.sweepKeep)
}

// Rust: list_workspace_branches: the branches of the workspace repository are listed.
func TestListWorkspaceBranchesListsTheWorkspaceRepository(t *testing.T) {
	worktrees := &fakeWorktree{branches: []models.BranchOption{{Name: "main", Current: true}}}
	lifecycle, _ := newLifecycle(newFakeRepository())

	branches, err := application.NewListWorkspaceBranches(lifecycle, worktrees).Execute(context.Background(), "ws-1")

	require.NoError(t, err)
	assert.Equal(t, worktrees.branches, branches)
	assert.Equal(t, []string{"/repos/demo"}, worktrees.branchesAsked)
}

// Rust: list_workspace_branches: an unknown workspace is refused before git runs.
func TestListWorkspaceBranchesOfUnknownWorkspaceIsRefused(t *testing.T) {
	worktrees := &fakeWorktree{}
	lifecycle, _ := newLifecycle(newFakeRepository())

	_, err := application.NewListWorkspaceBranches(lifecycle, worktrees).Execute(context.Background(), "missing")

	assert.ErrorIs(t, err, werrors.WorkspaceNotFound(""))
	assert.Empty(t, worktrees.branchesAsked)
}
