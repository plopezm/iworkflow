package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
)

// assignmentsOf builds the library and the assignment store with one assignment of hook a in workspace w.
func assignmentsOf(t *testing.T, workspaceID string) *fakeAssignments {
	t.Helper()
	library := &fakeHooks{hooks: []models.Hook{libraryHook(t, "a", models.KindScript)}}
	return &fakeAssignments{
		hooks: library,
		items: []models.Assignment{assignment(t, "a", workspaceID, models.EventTaskStart, models.PhasePre, models.FailureWarn, true)},
	}
}

func TestSetTaskHookSkippedTogglesTheSkipForOneAssignment(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{ids: []string{"other"}}
	uc := application.NewSetTaskHookSkipped(assignmentsOf(t, "w1"), skips)
	ctx := context.Background()

	require.NoError(t, uc.Execute(ctx, "w1", "t1", "as-a", true))
	assert.ElementsMatch(t, []string{"other", "as-a"}, skips.ids)

	require.NoError(t, uc.Execute(ctx, "w1", "t1", "as-a", true), "skipping twice keeps one skip")
	assert.ElementsMatch(t, []string{"other", "as-a"}, skips.ids)

	require.NoError(t, uc.Execute(ctx, "w1", "t1", "as-a", false))
	assert.Equal(t, []string{"other"}, skips.ids)
}

func TestSetTaskHookSkippedRejectsMissingIDs(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{}
	uc := application.NewSetTaskHookSkipped(assignmentsOf(t, "w1"), skips)

	require.ErrorIs(t, uc.Execute(context.Background(), "", "t1", "as-a", true), hookerrors.ErrInvalidHook)
	require.ErrorIs(t, uc.Execute(context.Background(), "w1", " ", "as-a", true), hookerrors.ErrInvalidHook)
	require.ErrorIs(t, uc.Execute(context.Background(), "w1", "t1", "", true), hookerrors.ErrInvalidHook)
	assert.Empty(t, skips.ids)
}

// An assignment of another workspace cannot be skipped for a task of this one: the skip is refused.
func TestSetTaskHookSkippedRefusesAnAssignmentOfAnotherWorkspace(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{}
	uc := application.NewSetTaskHookSkipped(assignmentsOf(t, "w2"), skips)

	err := uc.Execute(context.Background(), "w1", "t1", "as-a", true)

	require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
	assert.Empty(t, skips.ids, "a refused skip stores nothing")
}

func TestSetTaskHookSkippedRefusesAnUnknownAssignment(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{}
	uc := application.NewSetTaskHookSkipped(&fakeAssignments{hooks: &fakeHooks{}}, skips)

	err := uc.Execute(context.Background(), "w1", "t1", "missing", true)

	require.ErrorIs(t, err, hookerrors.ErrAssignmentNotFound)
	assert.Empty(t, skips.ids)
}

func TestListTaskHookStateShowsEachAssignmentWithItsSkipAndLatestRun(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	library := &fakeHooks{hooks: []models.Hook{
		libraryHook(t, "a", models.KindScript), libraryHook(t, "b", models.KindScript), libraryHook(t, "c", models.KindScript),
	}}
	assignments := &fakeAssignments{hooks: library, items: []models.Assignment{
		assignment(t, "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureWarn, true),
		assignment(t, "b", "w1", models.EventTaskStart, models.PhasePre, models.FailureWarn, true),
		assignment(t, "c", "w2", models.EventTaskStart, models.PhasePre, models.FailureWarn, true),
	}}
	runs := &fakeRuns{saved: []models.HookRun{
		{ID: "r1", TaskID: "t1", AssignmentID: "as-a", Status: models.RunRunning, StartedAt: start},
		{ID: "r1", TaskID: "t1", AssignmentID: "as-a", Status: models.RunFailed, StartedAt: start, FinishedAt: start.Add(time.Second)},
		{ID: "r9", TaskID: "t2", AssignmentID: "as-b", Status: models.RunOK, StartedAt: start},
	}}
	skips := &fakeSkips{ids: []string{"as-b"}}

	got, err := application.NewListTaskHookState(assignments, skips, runs).Execute(context.Background(), "w1", "t1")

	require.NoError(t, err)
	require.Len(t, got, 2, "assignments of other workspaces are not listed")
	assert.Equal(t, "as-a", got[0].Assignment.ID())
	assert.Equal(t, "a", got[0].Hook.ID())
	assert.False(t, got[0].Skipped)
	require.NotNil(t, got[0].LastRun)
	assert.Equal(t, models.RunFailed, got[0].LastRun.Status, "the last record of the assignment wins")
	assert.True(t, got[1].Skipped)
	assert.Nil(t, got[1].LastRun, "runs of another task are not shown")
}

func TestListTaskHookStateReportsRepositoryFailures(t *testing.T) {
	t.Parallel()
	runs := &failingRuns{err: hookerrors.ErrRepository}

	_, err := application.NewListTaskHookState(&fakeAssignments{hooks: &fakeHooks{}}, &fakeSkips{}, runs).Execute(context.Background(), "w1", "t1")

	require.ErrorIs(t, err, hookerrors.ErrRepository)
}

// failingRuns fails every read; the other methods are the fake's.
type failingRuns struct {
	fakeRuns
	err error
}

func (f *failingRuns) ListByTask(context.Context, string) ([]models.HookRun, error) {
	return nil, f.err
}
