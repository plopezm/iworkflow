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

func TestSetTaskHookSkippedTogglesTheSkipForOneHook(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{ids: []string{"other"}}
	hooks := &fakeHooks{hooks: []models.Hook{savedHook(t, "a", "w1", models.KindScript, models.PhasePre, models.EventTaskStart)}}
	uc := application.NewSetTaskHookSkipped(hooks, skips)
	ctx := context.Background()

	require.NoError(t, uc.Execute(ctx, "w1", "t1", "a", true))
	assert.ElementsMatch(t, []string{"other", "a"}, skips.ids)

	require.NoError(t, uc.Execute(ctx, "w1", "t1", "a", true), "skipping twice keeps one skip")
	assert.ElementsMatch(t, []string{"other", "a"}, skips.ids)

	require.NoError(t, uc.Execute(ctx, "w1", "t1", "a", false))
	assert.Equal(t, []string{"other"}, skips.ids)
}

func TestSetTaskHookSkippedRejectsMissingIDs(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{}
	hooks := &fakeHooks{hooks: []models.Hook{savedHook(t, "a", "w1", models.KindScript, models.PhasePre, models.EventTaskStart)}}
	uc := application.NewSetTaskHookSkipped(hooks, skips)

	require.ErrorIs(t, uc.Execute(context.Background(), "", "t1", "a", true), hookerrors.ErrInvalidHook)
	require.ErrorIs(t, uc.Execute(context.Background(), "w1", " ", "a", true), hookerrors.ErrInvalidHook)
	require.ErrorIs(t, uc.Execute(context.Background(), "w1", "t1", "", true), hookerrors.ErrInvalidHook)
	assert.Empty(t, skips.ids)
}

// A hook of another workspace cannot be skipped for a task of this one: the skip is refused and nothing is stored.
func TestSetTaskHookSkippedRefusesAHookOfAnotherWorkspace(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{}
	hooks := &fakeHooks{hooks: []models.Hook{savedHook(t, "a", "w2", models.KindScript, models.PhasePre, models.EventTaskStart)}}
	uc := application.NewSetTaskHookSkipped(hooks, skips)

	err := uc.Execute(context.Background(), "w1", "t1", "a", true)

	require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
	assert.Empty(t, skips.ids, "a refused skip stores nothing")
}

// An unknown hook is not skipped either, and the lookup failure is reported as such.
func TestSetTaskHookSkippedRefusesAnUnknownHook(t *testing.T) {
	t.Parallel()
	skips := &fakeSkips{}
	uc := application.NewSetTaskHookSkipped(&fakeHooks{}, skips)

	err := uc.Execute(context.Background(), "w1", "t1", "missing", true)

	require.ErrorIs(t, err, hookerrors.ErrHookNotFound)
	assert.Empty(t, skips.ids)
}

func TestListTaskHookStateShowsEachHookWithItsSkipAndLatestRun(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	hooks := &fakeHooks{hooks: []models.Hook{
		workspaceHook(t, "a", "w1", "x"), workspaceHook(t, "b", "w1", "x"), workspaceHook(t, "c", "w2", "x"),
	}}
	runs := &fakeRuns{saved: []models.HookRun{
		{ID: "r1", TaskID: "t1", HookID: "a", Status: models.RunRunning, StartedAt: start},
		{ID: "r1", TaskID: "t1", HookID: "a", Status: models.RunFailed, StartedAt: start, FinishedAt: start.Add(time.Second)},
		{ID: "r9", TaskID: "t2", HookID: "b", Status: models.RunOK, StartedAt: start},
	}}
	skips := &fakeSkips{ids: []string{"b"}}

	got, err := application.NewListTaskHookState(hooks, skips, runs).Execute(context.Background(), "w1", "t1")

	require.NoError(t, err)
	require.Len(t, got, 2, "hooks of other workspaces are not listed")
	assert.Equal(t, "a", got[0].Hook.ID())
	assert.False(t, got[0].Skipped)
	require.NotNil(t, got[0].LastRun)
	assert.Equal(t, models.RunFailed, got[0].LastRun.Status, "the last record of the hook wins")
	assert.Equal(t, "b", got[1].Hook.ID())
	assert.True(t, got[1].Skipped)
	assert.Nil(t, got[1].LastRun, "runs of another task are not shown")
}

func TestListTaskHookStateReportsRepositoryFailures(t *testing.T) {
	t.Parallel()
	runs := &failingRuns{err: hookerrors.ErrRepository}

	_, err := application.NewListTaskHookState(&fakeHooks{}, &fakeSkips{}, runs).Execute(context.Background(), "w1", "t1")

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
