package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

// mkHook puts a hook of the library and its assignment (pre task_start of workspace w) in one value; the assignment
// id is "as-<id>".
type mkHooked struct {
	hook       models.Hook
	assignment models.Assignment
}

func mkHook(t *testing.T, id string, kind models.Kind, policy models.FailurePolicy, enabled bool) mkHooked {
	t.Helper()
	return mkHooked{
		hook:       libraryHook(t, id, kind),
		assignment: assignment(t, id, "w", models.EventTaskStart, models.PhasePre, policy, enabled),
	}
}

type rig struct {
	uc      *application.RunHooks
	runs    *fakeRuns
	scripts *fakeRunner
	agents  *fakeRunner
	calls   []string
}

func newRig(items []mkHooked, skipped []string) *rig {
	r := &rig{runs: &fakeRuns{}}
	library := &fakeHooks{}
	assignments := &fakeAssignments{hooks: library}
	for _, item := range items {
		library.hooks = append(library.hooks, item.hook)
		assignments.items = append(assignments.items, item.assignment)
	}
	r.scripts = &fakeRunner{outcomes: map[string]models.RunOutcome{}, errs: map[string]error{}, calls: &r.calls}
	r.agents = &fakeRunner{outcomes: map[string]models.RunOutcome{}, errs: map[string]error{}, calls: &r.calls}
	r.uc = application.NewRunHooks(assignments, &fakeSkips{skipped}, r.runs, r.scripts, r.agents,
		&fakeClock{now: time.Unix(0, 0)}, &fakeIDs{})
	return r
}

func input() application.RunHooksInput {
	return application.RunHooksInput{WorkspaceID: "w", Run: models.RunContext{
		TaskID: "t", Phase: models.PhasePre, Event: models.EventTaskStart,
	}}
}

func statuses(runs []models.HookRun) []models.RunStatus {
	out := make([]models.RunStatus, 0, len(runs))
	for _, r := range runs {
		out = append(out, r.Status)
	}
	return out
}

func TestRunsInOrderByKindAndRecords(t *testing.T) {
	r := newRig([]mkHooked{
		mkHook(t, "a", models.KindScript, models.FailureBlock, true),
		mkHook(t, "b", models.KindAgent, models.FailureBlock, true),
	}, nil)
	report, err := r.uc.Execute(context.Background(), input())
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b"}, r.calls)
	assert.Equal(t, []models.RunStatus{models.RunOK, models.RunOK}, statuses(report.Runs))
	assert.Equal(t, "as-a", report.Runs[0].AssignmentID)
	assert.Equal(t, "hook a", report.Runs[0].HookName)
	assert.Equal(t, "w", report.Runs[0].WorkspaceID)
	// each hook is saved twice: running, then its result
	assert.Len(t, r.runs.saved, 4)
	assert.Equal(t, models.RunRunning, r.runs.saved[0].Status)
	assert.False(t, r.runs.saved[1].FinishedAt.IsZero())
}

func TestDisabledHooksDoNotRunAndSkippedAreRecorded(t *testing.T) {
	r := newRig([]mkHooked{
		mkHook(t, "off", models.KindScript, models.FailureBlock, false),
		mkHook(t, "skip", models.KindScript, models.FailureBlock, true),
		mkHook(t, "go", models.KindScript, models.FailureBlock, true),
	}, []string{"as-skip"})
	report, err := r.uc.Execute(context.Background(), input())
	require.NoError(t, err)
	assert.Equal(t, []string{"go"}, r.calls)
	assert.Equal(t, []models.RunStatus{models.RunSkipped, models.RunOK}, statuses(report.Runs))
}

func TestFailurePolicies(t *testing.T) {
	failed := models.RunOutcome{Status: models.RunFailed, ExitCode: 1, Output: "no"}
	t.Run("block stops the chain", func(t *testing.T) {
		r := newRig([]mkHooked{
			mkHook(t, "a", models.KindScript, models.FailureBlock, true),
			mkHook(t, "b", models.KindScript, models.FailureBlock, true),
		}, nil)
		r.scripts.outcomes["a"] = failed
		report, err := r.uc.Execute(context.Background(), input())
		assert.ErrorIs(t, err, hookerrors.ErrHookBlocked)
		assert.Equal(t, []string{"a"}, r.calls)
		require.Len(t, report.Runs, 1)
		assert.Equal(t, "no", report.Runs[0].Output)
	})
	t.Run("warn continues with a warning", func(t *testing.T) {
		r := newRig([]mkHooked{
			mkHook(t, "a", models.KindScript, models.FailureWarn, true),
			mkHook(t, "b", models.KindScript, models.FailureBlock, true),
		}, nil)
		r.scripts.outcomes["a"] = failed
		report, err := r.uc.Execute(context.Background(), input())
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, r.calls)
		assert.Len(t, report.Warnings, 1)
	})
	t.Run("ignore continues silently", func(t *testing.T) {
		r := newRig([]mkHooked{mkHook(t, "a", models.KindScript, models.FailureIgnore, true)}, nil)
		r.scripts.outcomes["a"] = models.RunOutcome{Status: models.RunTimedOut, ExitCode: -1}
		report, err := r.uc.Execute(context.Background(), input())
		require.NoError(t, err)
		assert.Empty(t, report.Warnings)
		assert.Equal(t, models.RunTimedOut, report.Runs[0].Status)
	})
}

func TestRunnerErrorIsRecordedAsErroredAndFollowsPolicy(t *testing.T) {
	r := newRig([]mkHooked{mkHook(t, "a", models.KindScript, models.FailureBlock, true)}, nil)
	r.scripts.errs["a"] = errors.New("bad template")
	report, err := r.uc.Execute(context.Background(), input())
	assert.ErrorIs(t, err, hookerrors.ErrHookBlocked)
	assert.Equal(t, models.RunErrored, report.Runs[0].Status)
	assert.Contains(t, report.Runs[0].Output, "bad template")
}

func TestOtherEventsAndPhasesAreNotRun(t *testing.T) {
	r := newRig([]mkHooked{mkHook(t, "a", models.KindScript, models.FailureBlock, true)}, nil)
	in := input()
	in.Run.Phase = models.PhasePost
	report, err := r.uc.Execute(context.Background(), in)
	require.NoError(t, err)
	assert.Empty(t, report.Runs)
	assert.Empty(t, r.calls)
}

func TestCancelledContextStopsBeforeRunning(t *testing.T) {
	r := newRig([]mkHooked{mkHook(t, "a", models.KindScript, models.FailureBlock, true)}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.uc.Execute(ctx, input())
	assert.ErrorIs(t, err, context.Canceled)
	assert.Empty(t, r.calls)
}

// The same hook assigned to two workspaces runs only for the workspace of the task.
func TestAHookAssignedToTwoWorkspacesRunsForTheTaskWorkspaceOnly(t *testing.T) {
	item := mkHook(t, "a", models.KindScript, models.FailureBlock, true)
	other, err := models.NewAssignment(models.AssignmentParams{
		ID: "as-other", HookID: "a", WorkspaceID: "w2", Event: models.EventTaskStart, Phase: models.PhasePre,
		Enabled: true, OnFailure: models.FailureWarn,
	})
	require.NoError(t, err)
	r := newRig([]mkHooked{item}, nil)
	library := &fakeHooks{hooks: []models.Hook{item.hook}}
	assignments := &fakeAssignments{hooks: library, items: []models.Assignment{item.assignment, other}}
	uc := application.NewRunHooks(assignments, &fakeSkips{}, r.runs, r.scripts, r.agents, &fakeClock{now: time.Unix(0, 0)}, &fakeIDs{})

	report, err := uc.Execute(context.Background(), input())

	require.NoError(t, err)
	require.Len(t, report.Runs, 1)
	assert.Equal(t, "as-a", report.Runs[0].AssignmentID)
}

// The failure policy belongs to the assignment: one hook can block in a workspace and only warn in another.
func TestFailurePolicyComesFromTheAssignment(t *testing.T) {
	item := mkHook(t, "a", models.KindScript, models.FailureWarn, true)
	r := newRig([]mkHooked{item}, nil)
	r.scripts.outcomes["a"] = models.RunOutcome{Status: models.RunFailed, ExitCode: 1}

	report, err := r.uc.Execute(context.Background(), input())

	require.NoError(t, err)
	assert.Equal(t, []string{"hook hook a failed"}, report.Warnings)
}
