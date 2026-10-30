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

// Rust: detach_task_from_graph::detaching_reevaluates_the_graph
func TestDetachTaskFromGraphRemovesEdgesAndPositionThenReevaluates(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	other := newTask(models.StatusMerged)
	third := newTask(models.StatusBacklog)
	kept := edgeOf(other.ID, third.ID)
	graph := &fakeGraph{
		edges: []models.TaskDependency{edgeOf(task.ID, other.ID), edgeOf(other.ID, task.ID), kept},
		positions: []models.TaskPosition{
			{TaskID: task.ID, Position: models.NodePosition{X: 1, Y: 2}},
			{TaskID: other.ID, Position: models.NodePosition{X: 3, Y: 4}},
		},
	}
	trigger := &graphTrigger{}

	err := application.NewDetachTaskFromGraph(graph, trigger).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskDependency{kept}, graph.edges)
	assert.Equal(t, []models.TaskPosition{{TaskID: other.ID, Position: models.NodePosition{X: 3, Y: 4}}}, graph.positions)
	assert.Equal(t, 1, trigger.calls)
}

// Rust: detach_task_from_graph::trigger_failure_is_only_logged
func TestDetachTaskFromGraphTriggerFailureIsOnlyLogged(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	other := newTask(models.StatusMerged)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(other.ID, task.ID)}}
	trigger := &graphTrigger{err: werrors.Repository("boom")}

	err := application.NewDetachTaskFromGraph(graph, trigger).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Empty(t, graph.edges)
	assert.Equal(t, 1, trigger.calls)
}

// Rust: detach_task_from_graph::repository_failure_is_propagated_without_triggering
func TestDetachTaskFromGraphRepositoryFailureIsPropagatedWithoutTriggering(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	other := newTask(models.StatusMerged)
	stored := edgeOf(other.ID, task.ID)
	graph := &fakeGraph{edges: []models.TaskDependency{stored}, detachErr: werrors.Repository("boom")}
	trigger := &graphTrigger{}

	err := application.NewDetachTaskFromGraph(graph, trigger).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.Repository("boom"))
	assert.Equal(t, []models.TaskDependency{stored}, graph.edges)
	assert.Zero(t, trigger.calls)
}
