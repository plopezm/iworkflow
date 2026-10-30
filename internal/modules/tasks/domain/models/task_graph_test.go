package models_test

import (
	"math"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// graphEdge is a merged-trigger edge of the workspace "w1".
func graphEdge(predecessor models.TaskID, successor models.TaskID) models.TaskDependency {
	return models.TaskDependency{
		Predecessor: predecessor,
		Successor:   successor,
		WorkspaceID: "w1",
		Trigger:     models.DependencyTriggerMerged,
	}
}

// graphTask builds a task of the workspace "w1" in the given status with a fresh id.
func graphTask(t *testing.T, status models.TaskStatus) models.Task {
	t.Helper()
	return models.Task{ID: newGraphTaskID(t), WorkspaceID: "w1", Status: status}
}

// newGraphTaskID generates a fresh task identifier.
func newGraphTaskID(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id
}

// Rust: task_graph::trigger_round_trips_and_rejects_unknown
func TestTriggerRoundTripsAndRejectsUnknown(t *testing.T) {
	t.Parallel()
	got, err := models.ParseDependencyTrigger(string(models.DependencyTriggerMerged))
	require.NoError(t, err)
	assert.Equal(t, models.DependencyTriggerMerged, got)

	_, err = models.ParseDependencyTrigger("nope")
	require.ErrorIs(t, err, werrors.InvalidTrigger("nope"))
	assert.EqualError(t, err, "disparador de dependencia desconocido: nope")
}

// Rust: task_graph::position_rejects_non_finite_coordinates
func TestPositionRejectsNonFiniteCoordinates(t *testing.T) {
	t.Parallel()
	position, err := models.NewNodePosition(1.0, -2.5)
	require.NoError(t, err)
	assert.Equal(t, models.NodePosition{X: 1.0, Y: -2.5}, position)

	for _, coordinates := range [][2]float64{
		{math.NaN(), 0.0},
		{0.0, math.Inf(1)},
		{math.Inf(-1), 0.0},
	} {
		rejected, err := models.NewNodePosition(coordinates[0], coordinates[1])
		require.ErrorIs(t, err, werrors.InvalidPosition())
		assert.Equal(t, models.NodePosition{}, rejected)
	}
}

// Rust: task_graph::rejects_self_duplicate_and_cycles
func TestRejectsSelfDuplicateAndCycles(t *testing.T) {
	t.Parallel()
	a, b, c := newGraphTaskID(t), newGraphTaskID(t), newGraphTaskID(t)

	existing := []models.TaskDependency{graphEdge(a, b), graphEdge(b, c)}
	before := slices.Clone(existing)

	require.ErrorIs(t, models.ValidateNewDependency(existing, graphEdge(a, a)), werrors.SelfDependency())
	require.ErrorIs(t, models.ValidateNewDependency(existing, graphEdge(a, b)), werrors.DuplicateDependency())
	require.ErrorIs(t, models.ValidateNewDependency(existing, graphEdge(c, a)), werrors.DependencyCycle())
	// Parallel and diamond shapes are valid.
	require.NoError(t, models.ValidateNewDependency(existing, graphEdge(a, c)))
	// Validation only reads the edges it is given.
	assert.Equal(t, before, existing)
}

// Rust: task_graph::starts_backlog_task_only_when_every_predecessor_is_merged
func TestStartsBacklogTaskOnlyWhenEveryPredecessorIsMerged(t *testing.T) {
	t.Parallel()
	merged := graphTask(t, models.StatusMerged)
	reviewing := graphTask(t, models.StatusAwaitingReview)
	ready := graphTask(t, models.StatusBacklog)
	waiting := graphTask(t, models.StatusBacklog)
	root := graphTask(t, models.StatusBacklog)
	edges := []models.TaskDependency{
		graphEdge(merged.ID, ready.ID),
		graphEdge(merged.ID, waiting.ID),
		graphEdge(reviewing.ID, waiting.ID),
	}
	tasks := []models.Task{merged, reviewing, ready, waiting, root}

	assert.Equal(t, []models.TaskID{ready.ID}, models.StartableTasks(tasks, edges))
}

// Rust: task_graph::already_started_successors_are_not_restarted
func TestAlreadyStartedSuccessorsAreNotRestarted(t *testing.T) {
	t.Parallel()
	merged := graphTask(t, models.StatusMerged)
	running := graphTask(t, models.StatusRunning)
	edges := []models.TaskDependency{graphEdge(merged.ID, running.ID)}

	assert.Empty(t, models.StartableTasks([]models.Task{merged, running}, edges))
}

// Rust: task_graph::backlog_successor_with_discarded_predecessor_is_blocked
func TestBacklogSuccessorWithDiscardedPredecessorIsBlocked(t *testing.T) {
	t.Parallel()
	discarded := graphTask(t, models.StatusDiscarded)
	merged := graphTask(t, models.StatusMerged)
	stuck := graphTask(t, models.StatusBacklog)
	fine := graphTask(t, models.StatusBacklog)
	running := graphTask(t, models.StatusRunning)
	edges := []models.TaskDependency{
		graphEdge(discarded.ID, stuck.ID),
		graphEdge(merged.ID, stuck.ID),
		graphEdge(merged.ID, fine.ID),
		graphEdge(discarded.ID, running.ID),
	}
	tasks := []models.Task{discarded, merged, stuck, fine, running}

	assert.Equal(t, []models.TaskID{stuck.ID}, models.BlockedByDiscarded(tasks, edges))
	assert.NotContains(t, models.StartableTasks(tasks, edges), stuck.ID)
}
