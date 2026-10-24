package tasks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// dependencyIn builds a merged-trigger edge inside the workspace.
func dependencyIn(workspaceID string, predecessor models.TaskID, successor models.TaskID) models.TaskDependency {
	return models.TaskDependency{
		Predecessor: predecessor,
		Successor:   successor,
		WorkspaceID: workspaceID,
		Trigger:     models.DependencyTriggerMerged,
	}
}

// Rust: task_graph_repository_adapter_test::stores_lists_and_removes_dependencies
func TestGraphStoresListsAndRemovesDependencies(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ws := insertWorkspace(t, repo.DB, "/repos/demo")
	otherWs := insertWorkspace(t, repo.DB, "/repos/other")
	a := newTask(t, ws, "a", models.StatusBacklog)
	b := newTask(t, ws, "b", models.StatusBacklog)
	c := newTask(t, ws, "c", models.StatusBacklog)
	foreign := newTask(t, otherWs, "x", models.StatusBacklog)
	insertTask(t, repo, a)
	insertTask(t, repo, b)
	insertTask(t, repo, c)
	insertTask(t, repo, foreign)
	ctx := bg()

	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, a.ID, b.ID)))
	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, a.ID, c.ID)))
	before := allDependencies(t, repo)
	require.ErrorIs(t, repo.Graph.InsertDependency(ctx, dependencyIn(otherWs, foreign.ID, foreign.ID)), werrors.SelfDependency())
	assert.ElementsMatch(t, before, allDependencies(t, repo), "the rejected self edge stores nothing")

	listed, err := repo.Graph.ListDependencies(ctx, ptr(ws))
	require.NoError(t, err)
	assert.Len(t, listed, 2)
	assert.Contains(t, listed, dependencyIn(ws, a.ID, b.ID))
	other, err := repo.Graph.ListDependencies(ctx, ptr(otherWs))
	require.NoError(t, err)
	assert.Empty(t, other)

	removed, err := repo.Graph.RemoveDependency(ctx, a.ID, b.ID)
	require.NoError(t, err)
	assert.True(t, removed)
	removedAgain, err := repo.Graph.RemoveDependency(ctx, a.ID, b.ID)
	require.NoError(t, err)
	assert.False(t, removedAgain)
	assert.Equal(t, []models.TaskDependency{dependencyIn(ws, a.ID, c.ID)}, allDependencies(t, repo))
}

// Rust: task_graph_repository_adapter_test::rejects_duplicates_and_cycles_without_changing_stored_edges
func TestGraphRejectsDuplicatesAndCyclesWithoutChangingStoredEdges(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ws := insertWorkspace(t, repo.DB, "/repos/demo")
	a := newTask(t, ws, "a", models.StatusBacklog)
	b := newTask(t, ws, "b", models.StatusBacklog)
	c := newTask(t, ws, "c", models.StatusBacklog)
	insertTask(t, repo, a)
	insertTask(t, repo, b)
	insertTask(t, repo, c)
	ctx := bg()
	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, a.ID, b.ID)))
	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, b.ID, c.ID)))
	before := allDependencies(t, repo)

	require.ErrorIs(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, a.ID, b.ID)), werrors.DuplicateDependency())
	require.ErrorIs(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, c.ID, a.ID)), werrors.DependencyCycle())
	require.ErrorIs(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, a.ID, a.ID)), werrors.SelfDependency())

	assert.ElementsMatch(t, before, allDependencies(t, repo))
}

// Rust: task_graph_repository_adapter_test::deleting_a_task_removes_its_edges_and_position
func TestGraphDeletingATaskRemovesItsEdgesAndPosition(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ws := insertWorkspace(t, repo.DB, "/repos/demo")
	a := newTask(t, ws, "a", models.StatusBacklog)
	b := newTask(t, ws, "b", models.StatusBacklog)
	insertTask(t, repo, a)
	insertTask(t, repo, b)
	ctx := bg()
	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, a.ID, b.ID)))
	require.NoError(t, repo.Graph.SetPosition(ctx, b.ID, models.NodePosition{X: 1.0, Y: 2.0}))

	deleted, err := repo.Tasks.DeleteIfBacklog(ctx, b.ID)
	require.NoError(t, err)
	assert.True(t, deleted)

	assert.Empty(t, allDependencies(t, repo))
	positions, err := repo.Graph.ListPositions(ctx, ws)
	require.NoError(t, err)
	assert.Empty(t, positions)
}

// Rust: task_graph_repository_adapter_test::positions_are_upserted_per_task_and_scoped_to_the_workspace
func TestGraphPositionsAreUpsertedPerTaskAndScopedToTheWorkspace(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ws := insertWorkspace(t, repo.DB, "/repos/demo")
	otherWs := insertWorkspace(t, repo.DB, "/repos/other")
	a := newTask(t, ws, "a", models.StatusBacklog)
	foreign := newTask(t, otherWs, "x", models.StatusBacklog)
	insertTask(t, repo, a)
	insertTask(t, repo, foreign)
	ctx := bg()

	require.NoError(t, repo.Graph.SetPosition(ctx, a.ID, models.NodePosition{X: 10.0, Y: 20.0}))
	require.NoError(t, repo.Graph.SetPosition(ctx, a.ID, models.NodePosition{X: -5.5, Y: 7.25}))
	require.NoError(t, repo.Graph.SetPosition(ctx, foreign.ID, models.NodePosition{X: 1.0, Y: 1.0}))
	// A task that does not exist is ignored without an error, and stores nothing.
	require.NoError(t, repo.Graph.SetPosition(ctx, newTaskID(t), models.NodePosition{X: 0.0, Y: 0.0}))

	positions, err := repo.Graph.ListPositions(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, []models.TaskPosition{{TaskID: a.ID, Position: models.NodePosition{X: -5.5, Y: 7.25}}}, positions)
	otherPositions, err := repo.Graph.ListPositions(ctx, otherWs)
	require.NoError(t, err)
	assert.Equal(t, []models.TaskPosition{{TaskID: foreign.ID, Position: models.NodePosition{X: 1.0, Y: 1.0}}}, otherPositions)
}

// Rust: task_graph_repository_adapter_test::detaching_a_task_removes_its_edges_in_both_directions_and_its_position
func TestGraphDetachingATaskRemovesItsEdgesInBothDirectionsAndItsPosition(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ws := insertWorkspace(t, repo.DB, "/repos/demo")
	a := newTask(t, ws, "a", models.StatusBacklog)
	b := newTask(t, ws, "b", models.StatusBacklog)
	c := newTask(t, ws, "c", models.StatusBacklog)
	d := newTask(t, ws, "d", models.StatusBacklog)
	e := newTask(t, ws, "e", models.StatusBacklog)
	for _, task := range []models.Task{a, b, c, d, e} {
		insertTask(t, repo, task)
	}
	ctx := bg()
	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, a.ID, b.ID)))
	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, b.ID, c.ID)))
	require.NoError(t, repo.Graph.InsertDependency(ctx, dependencyIn(ws, d.ID, e.ID)))
	require.NoError(t, repo.Graph.SetPosition(ctx, b.ID, models.NodePosition{X: 1.0, Y: 2.0}))
	require.NoError(t, repo.Graph.SetPosition(ctx, d.ID, models.NodePosition{X: 3.0, Y: 4.0}))

	require.NoError(t, repo.Graph.DetachTask(ctx, b.ID))

	assert.Equal(t, []models.TaskDependency{dependencyIn(ws, d.ID, e.ID)}, allDependencies(t, repo))
	positions, err := repo.Graph.ListPositions(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, []models.TaskPosition{{TaskID: d.ID, Position: models.NodePosition{X: 3.0, Y: 4.0}}}, positions)
}

// Rust: task_graph_repository_adapter_test::detaching_is_idempotent_and_accepts_unknown_tasks
func TestGraphDetachingIsIdempotentAndAcceptsUnknownTasks(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	ws := insertWorkspace(t, repo.DB, "/repos/demo")
	a := newTask(t, ws, "a", models.StatusBacklog)
	insertTask(t, repo, a)
	ctx := bg()

	require.NoError(t, repo.Graph.DetachTask(ctx, a.ID), "nothing to remove")
	require.NoError(t, repo.Graph.DetachTask(ctx, a.ID), "twice")
	require.NoError(t, repo.Graph.DetachTask(ctx, newTaskID(t)), "unknown")

	assert.Empty(t, allDependencies(t, repo))
	positions, err := repo.Graph.ListPositions(ctx, ws)
	require.NoError(t, err)
	assert.Empty(t, positions)
}

// allDependencies reads every edge of every workspace; the tests compare whole lists to prove that rejected
// writes changed nothing.
func allDependencies(t *testing.T, repo *Repo) []models.TaskDependency {
	t.Helper()
	edges, err := repo.Graph.ListDependencies(bg(), nil)
	require.NoError(t, err)
	return edges
}
