package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// addDependencyUseCase wires AddTaskDependency over the in-memory repository, graph and trigger.
func addDependencyUseCase(repo *fakeRepository, graph *fakeGraph, trigger *graphTrigger) *application.AddTaskDependency {
	lifecycle, _ := newLifecycle(repo)
	return application.NewAddTaskDependency(lifecycle, graph, trigger)
}

// Rust: add_task_dependency::stores_the_edge_and_evaluates_the_graph
func TestAddTaskDependencyStoresTheEdgeAndEvaluatesTheGraph(t *testing.T) {
	t.Parallel()
	from := newTask(models.StatusMerged)
	to := newTask(models.StatusBacklog)
	repo := newFakeRepository(from, to)
	graph := &fakeGraph{}
	trigger := &graphTrigger{result: 1}

	edge, err := addDependencyUseCase(repo, graph, trigger).Execute(context.Background(), from.ID, to.ID)

	require.NoError(t, err)
	want := edgeOf(from.ID, to.ID)
	assert.Equal(t, want, edge)
	assert.Equal(t, models.DependencyTriggerMerged, edge.Trigger)
	assert.Equal(t, []models.TaskDependency{want}, graph.edges)
	assert.Equal(t, 1, trigger.calls)
}

// Rust: add_task_dependency::rejects_self_edges_without_touching_storage
func TestAddTaskDependencyRejectsSelfEdgesWithoutTouchingStorage(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	repo := newFakeRepository(task)
	graph := &fakeGraph{}
	trigger := &graphTrigger{}

	_, err := addDependencyUseCase(repo, graph, trigger).Execute(context.Background(), task.ID, task.ID)

	require.ErrorIs(t, err, werrors.SelfDependency())
	assert.Zero(t, graph.inserts)
	assert.Empty(t, graph.edges)
	assert.Zero(t, trigger.calls)
}

// Rust: add_task_dependency::rejects_tasks_of_different_workspaces
func TestAddTaskDependencyRejectsTasksOfDifferentWorkspaces(t *testing.T) {
	t.Parallel()
	from := newTask(models.StatusMerged)
	to := newTask(models.StatusBacklog)
	to.WorkspaceID = "other"
	repo := newFakeRepository(from, to)
	graph := &fakeGraph{}
	trigger := &graphTrigger{}

	_, err := addDependencyUseCase(repo, graph, trigger).Execute(context.Background(), from.ID, to.ID)

	require.ErrorIs(t, err, werrors.CrossWorkspaceDependency())
	assert.Zero(t, graph.inserts)
	assert.Empty(t, graph.edges)
	assert.Zero(t, trigger.calls)
}

// Rust: add_task_dependency::rejects_started_successors_and_discarded_predecessors
func TestAddTaskDependencyRejectsStartedSuccessorsAndDiscardedPredecessors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		from      models.TaskStatus
		to        models.TaskStatus
		wantError error
	}{
		{
			name:      "started successor",
			from:      models.StatusBacklog,
			to:        models.StatusRunning,
			wantError: werrors.InvalidState("añadir una dependencia a", "running"),
		},
		{
			name:      "discarded predecessor",
			from:      models.StatusDiscarded,
			to:        models.StatusBacklog,
			wantError: werrors.InvalidState("depender de", "discarded"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			from := newTask(tc.from)
			to := newTask(tc.to)
			repo := newFakeRepository(from, to)
			graph := &fakeGraph{}
			trigger := &graphTrigger{}

			_, err := addDependencyUseCase(repo, graph, trigger).Execute(context.Background(), from.ID, to.ID)

			require.ErrorIs(t, err, tc.wantError)
			assert.Zero(t, graph.inserts)
			assert.Empty(t, graph.edges)
			assert.Zero(t, trigger.calls)
		})
	}
}

// Rust: add_task_dependency::a_failing_trigger_does_not_fail_the_add
func TestAddTaskDependencyAFailingTriggerDoesNotFailTheAdd(t *testing.T) {
	t.Parallel()
	from := newTask(models.StatusMerged)
	to := newTask(models.StatusBacklog)
	repo := newFakeRepository(from, to)
	graph := &fakeGraph{}
	trigger := &graphTrigger{err: werrors.Repository("x")}

	edge, err := addDependencyUseCase(repo, graph, trigger).Execute(context.Background(), from.ID, to.ID)

	require.NoError(t, err)
	assert.Equal(t, edgeOf(from.ID, to.ID), edge)
	assert.Equal(t, []models.TaskDependency{edgeOf(from.ID, to.ID)}, graph.edges)
	assert.Equal(t, 1, trigger.calls)
}

// Triangulation (no Rust counterpart at this layer): the graph rejects a duplicate edge, the error reaches
// the caller and the stored edges stay as they were.
func TestAddTaskDependencyRejectedDuplicateLeavesStoredEdgesUnchanged(t *testing.T) {
	t.Parallel()
	from := newTask(models.StatusMerged)
	to := newTask(models.StatusBacklog)
	repo := newFakeRepository(from, to)
	stored := edgeOf(from.ID, to.ID)
	graph := &fakeGraph{edges: []models.TaskDependency{stored}}
	trigger := &graphTrigger{}

	_, err := addDependencyUseCase(repo, graph, trigger).Execute(context.Background(), from.ID, to.ID)

	require.ErrorIs(t, err, werrors.DuplicateDependency())
	assert.Equal(t, []models.TaskDependency{stored}, graph.edges)
	assert.Zero(t, trigger.calls)
}
