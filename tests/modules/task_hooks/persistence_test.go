package task_hooks_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/db/dbtest"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence"
)

// setup returns the hook repository over a migrated database that holds two workspaces, w1 and w2.
func setup(t *testing.T) (*persistence.HookRepository, *sql.DB) {
	t.Helper()
	db := dbtest.New(t)
	_, err := db.Exec(`INSERT INTO workspaces (id, name, path, base_branch, created_at) VALUES ('w1', 'one', '/tmp/one', 'main', '2026-01-01T00:00:00Z'), ('w2', 'two', '/tmp/two', 'main', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	return persistence.NewHookRepository(db), db
}

// seedTask inserts a task of the workspace, so skips and runs can point at it.
func seedTask(t *testing.T, db *sql.DB, id string, workspace string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO tasks (id, workspace_id, title, prompt, branch, base_branch, worktree_path, status, created_at, updated_at)
		VALUES (?, ?, 't', 'p', 'b', 'main', '/tmp/wt', 'backlog', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, id, workspace)
	require.NoError(t, err)
}

// libraryHook builds a valid script hook of the library.
func libraryHook(t *testing.T, id string, name string) models.Hook {
	t.Helper()
	hook, err := models.NewHook(models.HookParams{
		ID: id, Name: name, Kind: models.KindScript, Timeout: 90 * time.Second,
		Script: models.ScriptSpec{Command: "echo", Args: []string{"{{.Branch}}"}, Env: []models.EnvVar{{Name: "A", Value: "1"}}, UseShell: true},
	})
	require.NoError(t, err)
	return hook
}

// newAssignment builds a valid enabled assignment of a library hook.
func newAssignment(t *testing.T, id string, hookID string, workspace string, event models.Event, phase models.Phase, policy models.FailurePolicy) models.Assignment {
	t.Helper()
	assignment, err := models.NewAssignment(models.AssignmentParams{
		ID: id, HookID: hookID, WorkspaceID: workspace, Event: event, Phase: phase, Enabled: true, OnFailure: policy,
	})
	require.NoError(t, err)
	return assignment
}

func assignedIDs(items []models.AssignedHook) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.Assignment.ID())
	}
	return ids
}

func TestHookSaveRoundTrip(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	script := libraryHook(t, "a", "Lint")
	agent, err := models.NewHook(models.HookParams{
		ID: "g", Name: "Review", Kind: models.KindAgent, Timeout: time.Minute,
		Agent: models.AgentSpec{Prompt: "review the diff", Runtime: "claude", Model: "m", Thinking: "high", Profile: "p"},
	})
	require.NoError(t, err)
	cases := map[string]models.Hook{"script": script, "agent": agent}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, repo.Save(ctx, want))

			got, err := repo.Get(ctx, want.ID())

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestHookSaveUpdatesTheExistingHook(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	renamed, err := models.NewHook(models.HookParams{
		ID: "a", Name: "Review", Kind: models.KindAgent, Timeout: time.Minute, Agent: models.AgentSpec{Prompt: "review"},
	})
	require.NoError(t, err)

	require.NoError(t, repo.Save(ctx, renamed))

	got, err := repo.Get(ctx, "a")
	require.NoError(t, err)
	assert.Equal(t, "Review", got.Name())
	assert.Equal(t, models.KindAgent, got.Kind())
	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1, "an update must not add a hook")
}

func TestHookListIsOrderedByNameIgnoringCase(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	for id, name := range map[string]string{"1": "beta", "2": "Alpha", "3": "gamma"} {
		require.NoError(t, repo.Save(ctx, libraryHook(t, id, name)))
	}

	list, err := repo.List(ctx)

	require.NoError(t, err)
	names := make([]string, 0, len(list))
	for _, hook := range list {
		names = append(names, hook.Name())
	}
	assert.Equal(t, []string{"Alpha", "beta", "gamma"}, names)
}

func TestHookNotFound(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()

	_, err := repo.Get(ctx, "missing")
	assert.ErrorIs(t, err, hookerrors.ErrHookNotFound)
	assert.ErrorIs(t, repo.Delete(ctx, "missing"), hookerrors.ErrHookNotFound)
}

func TestHookDeleteRemovesTheHook(t *testing.T) {
	repo, _ := setup(t)
	ctx := context.Background()
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))

	require.NoError(t, repo.Delete(ctx, "a"))

	_, err := repo.Get(ctx, "a")
	assert.ErrorIs(t, err, hookerrors.ErrHookNotFound)
}

func TestAssignmentSaveAppendsToItsGroupAndKeepsOrder(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, repo.Save(ctx, libraryHook(t, "b", "Test")))

	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "y", "b", "w1", models.EventTaskStart, models.PhasePre, models.FailureWarn)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "z", "a", "w1", models.EventTaskFinish, models.PhasePost, models.FailureIgnore)))

	got, err := assignments.ListFor(ctx, "w1", models.EventTaskStart, models.PhasePre)
	require.NoError(t, err)
	assert.Equal(t, []string{"x", "y"}, assignedIDs(got))
	assert.Equal(t, 1, got[0].Assignment.Position())
	assert.Equal(t, 2, got[1].Assignment.Position())
	assert.Equal(t, "Lint", got[0].Hook.Name(), "a list carries the hook of each assignment")
}

func TestAssignmentSaveUpdateKeepsPosition(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, repo.Save(ctx, libraryHook(t, "b", "Test")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "y", "b", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))

	disabled, err := models.NewAssignment(models.AssignmentParams{
		ID: "x", HookID: "a", WorkspaceID: "w1", Event: models.EventTaskStart, Phase: models.PhasePre,
		Enabled: false, OnFailure: models.FailureIgnore,
	})
	require.NoError(t, err)
	require.NoError(t, assignments.Save(ctx, disabled))

	got, err := assignments.Get(ctx, "x")
	require.NoError(t, err)
	assert.False(t, got.Enabled())
	assert.Equal(t, models.FailureIgnore, got.OnFailure())
	assert.Equal(t, 1, got.Position(), "an update must keep the position of the assignment")
	list, err := assignments.ListFor(ctx, "w1", models.EventTaskStart, models.PhasePre)
	require.NoError(t, err)
	assert.Equal(t, []string{"x", "y"}, assignedIDs(list))
}

func TestAssignmentSameHookInOtherGroupsIsAllowed(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))

	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "y", "a", "w1", models.EventTaskFinish, models.PhasePost, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "z", "a", "w2", models.EventTaskStart, models.PhasePre, models.FailureBlock)))

	byHook, err := assignments.ListByHook(ctx, "a")
	require.NoError(t, err)
	assert.Len(t, byHook, 3)
}

func TestAssignmentInvalidIsErrInvalidHook(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))

	cases := map[string]models.Assignment{
		"same hook, workspace, event and phase": newAssignment(t, "dup", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureWarn),
		"unknown workspace":                     newAssignment(t, "nows", "a", "missing", models.EventTaskStart, models.PhasePre, models.FailureBlock),
		"unknown hook":                          newAssignment(t, "nohook", "missing", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock),
	}
	for name, assignment := range cases {
		t.Run(name, func(t *testing.T) {
			err := assignments.Save(ctx, assignment)

			assert.ErrorIs(t, err, hookerrors.ErrInvalidHook)
		})
	}

	list, err := assignments.ListByWorkspace(ctx, "w1")
	require.NoError(t, err)
	assert.Equal(t, []string{"x"}, assignedIDs(list), "rejected assignments must not be stored")
}

func TestAssignmentListByWorkspaceOrdersByEventPhaseAndPosition(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, repo.Save(ctx, libraryHook(t, "b", "Test")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "y", "b", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "z", "a", "w1", models.EventTaskFinish, models.PhasePost, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "other", "a", "w2", models.EventTaskStart, models.PhasePre, models.FailureBlock)))

	list, err := assignments.ListByWorkspace(ctx, "w1")

	require.NoError(t, err)
	assert.Equal(t, []string{"z", "x", "y"}, assignedIDs(list))
}

func TestAssignmentListForFiltersByEventAndPhase(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "z", "a", "w1", models.EventTaskFinish, models.PhasePost, models.FailureBlock)))

	cases := map[string]struct {
		event models.Event
		phase models.Phase
		want  []string
	}{
		"start pre":   {models.EventTaskStart, models.PhasePre, []string{"x"}},
		"finish post": {models.EventTaskFinish, models.PhasePost, []string{"z"}},
		"start post":  {models.EventTaskStart, models.PhasePost, []string{}},
		"finish pre":  {models.EventTaskFinish, models.PhasePre, []string{}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			list, err := assignments.ListFor(ctx, "w1", tc.event, tc.phase)

			require.NoError(t, err)
			assert.Equal(t, tc.want, assignedIDs(list))
		})
	}
}

func TestAssignmentListByHookOrdersByWorkspaceEventAndPhase(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, repo.Save(ctx, libraryHook(t, "b", "Test")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "w2-start", "a", "w2", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "w1-start", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "w1-finish", "a", "w1", models.EventTaskFinish, models.PhasePost, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "other", "b", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))

	list, err := assignments.ListByHook(ctx, "a")

	require.NoError(t, err)
	ids := make([]string, 0, len(list))
	for _, assignment := range list {
		ids = append(ids, assignment.ID())
	}
	assert.Equal(t, []string{"w1-finish", "w1-start", "w2-start"}, ids)
}

func TestAssignmentGetAndDelete(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))

	_, err := assignments.Get(ctx, "missing")
	assert.ErrorIs(t, err, hookerrors.ErrAssignmentNotFound)
	assert.ErrorIs(t, assignments.Delete(ctx, "missing"), hookerrors.ErrAssignmentNotFound)

	got, err := assignments.Get(ctx, "x")
	require.NoError(t, err)
	assert.Equal(t, "a", got.HookID())
	require.NoError(t, assignments.Delete(ctx, "x"))
	_, err = assignments.Get(ctx, "x")
	assert.ErrorIs(t, err, hookerrors.ErrAssignmentNotFound)
}

func TestAssignmentReorderSetsTheOrderOfOneGroup(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	for _, id := range []string{"a", "b", "c"} {
		require.NoError(t, repo.Save(ctx, libraryHook(t, id, id)))
		require.NoError(t, assignments.Save(ctx, newAssignment(t, "x-"+id, id, "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	}
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "finish", "a", "w1", models.EventTaskFinish, models.PhasePost, models.FailureBlock)))

	require.NoError(t, assignments.Reorder(ctx, "w1", models.EventTaskStart, models.PhasePre, []string{"x-c", "x-a", "x-b"}))

	list, err := assignments.ListFor(ctx, "w1", models.EventTaskStart, models.PhasePre)
	require.NoError(t, err)
	assert.Equal(t, []string{"x-c", "x-a", "x-b"}, assignedIDs(list))
	for index, item := range list {
		assert.Equal(t, index+1, item.Assignment.Position())
	}
	other, err := assignments.ListFor(ctx, "w1", models.EventTaskFinish, models.PhasePost)
	require.NoError(t, err)
	assert.Equal(t, []string{"finish"}, assignedIDs(other), "a reorder must not touch the other groups")
}

func TestWorkspaceDeleteRemovesItsAssignmentsAndKeepsTheHook(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))

	_, err := db.Exec(`DELETE FROM workspaces WHERE id = 'w1'`)
	require.NoError(t, err)

	list, err := assignments.ListByWorkspace(ctx, "w1")
	require.NoError(t, err)
	assert.Empty(t, list)
	_, err = repo.Get(ctx, "a")
	assert.NoError(t, err, "the library is global and outlives a workspace")
}

func TestSkipsAreKeptPerTaskAndPerAssignment(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	seedTask(t, db, "t1", "w1")
	seedTask(t, db, "t2", "w1")
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "y", "a", "w1", models.EventTaskFinish, models.PhasePost, models.FailureBlock)))

	require.NoError(t, repo.SetSkipped(ctx, "t1", "x", true))
	require.NoError(t, repo.SetSkipped(ctx, "t1", "x", true), "skipping twice is the same as once")
	require.NoError(t, repo.SetSkipped(ctx, "t1", "y", true))
	ids, err := repo.SkippedAssignmentIDs(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, []string{"x", "y"}, ids)

	other, err := repo.SkippedAssignmentIDs(ctx, "t2")
	require.NoError(t, err)
	assert.Empty(t, other, "a skip belongs to its task only")

	require.NoError(t, repo.SetSkipped(ctx, "t1", "x", false))
	ids, err = repo.SkippedAssignmentIDs(ctx, "t1")
	require.NoError(t, err)
	assert.Equal(t, []string{"y"}, ids)
}

func TestDeletingAnAssignmentDropsItsSkips(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	seedTask(t, db, "t1", "w1")
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, repo.SetSkipped(ctx, "t1", "x", true))

	require.NoError(t, assignments.Delete(ctx, "x"))

	ids, err := repo.SkippedAssignmentIDs(ctx, "t1")
	require.NoError(t, err)
	assert.Empty(t, ids)
}

// Deleting a hook removes its assignments and their skips, but the run records it produced stay in the history.
func TestDeletingAHookCascadesItsAssignmentsAndKeepsItsRuns(t *testing.T) {
	repo, db := setup(t)
	ctx := context.Background()
	assignments := persistence.NewAssignmentRepository(db)
	runs := persistence.NewRunRepository(db)
	seedTask(t, db, "t1", "w1")
	require.NoError(t, repo.Save(ctx, libraryHook(t, "a", "Lint")))
	require.NoError(t, assignments.Save(ctx, newAssignment(t, "x", "a", "w1", models.EventTaskStart, models.PhasePre, models.FailureBlock)))
	require.NoError(t, repo.SetSkipped(ctx, "t1", "x", true))
	started := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	require.NoError(t, runs.Save(ctx, models.HookRun{
		ID: "r1", TaskID: "t1", HookID: "a", AssignmentID: "x", HookName: "Lint", WorkspaceID: "w1",
		Phase: models.PhasePre, Event: models.EventTaskStart, Status: models.RunFailed, ExitCode: 2, Output: "boom",
		StartedAt: started, FinishedAt: started.Add(time.Second),
	}))

	require.NoError(t, repo.Delete(ctx, "a"))

	byHook, err := assignments.ListByHook(ctx, "a")
	require.NoError(t, err)
	assert.Empty(t, byHook)
	byWorkspace, err := assignments.ListByWorkspace(ctx, "w1")
	require.NoError(t, err)
	assert.Empty(t, byWorkspace)
	ids, err := repo.SkippedAssignmentIDs(ctx, "t1")
	require.NoError(t, err)
	assert.Empty(t, ids)

	history, err := runs.List(ctx, outbound.RunFilter{HookID: "a"})
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "Lint", history[0].HookName)
	assert.Equal(t, "x", history[0].AssignmentID)
	assert.Equal(t, models.RunFailed, history[0].Status)
	assert.Equal(t, "boom", history[0].Output)
}
