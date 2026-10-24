package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: remove_task_dependency::removing_an_edge_reevaluates_the_graph
func TestRemoveTaskDependencyRemovesTheEdgeAndReevaluatesTheGraph(t *testing.T) {
	t.Parallel()
	first := newTask(models.StatusMerged)
	second := newTask(models.StatusBacklog)
	third := newTask(models.StatusBacklog)
	kept := edgeOf(first.ID, third.ID)
	graph := &fakeGraph{edges: []models.TaskDependency{edgeOf(first.ID, second.ID), kept}}
	trigger := &graphTrigger{}

	err := application.NewRemoveTaskDependency(graph, trigger).Execute(context.Background(), first.ID, second.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskDependency{kept}, graph.edges)
	assert.Equal(t, 1, trigger.calls)
}

// Rust: remove_task_dependency::removing_a_missing_edge_is_a_noop
func TestRemoveTaskDependencyMissingEdgeIsANoop(t *testing.T) {
	t.Parallel()
	first := newTask(models.StatusMerged)
	second := newTask(models.StatusBacklog)
	third := newTask(models.StatusBacklog)
	stored := edgeOf(first.ID, third.ID)
	graph := &fakeGraph{edges: []models.TaskDependency{stored}}
	trigger := &graphTrigger{}

	err := application.NewRemoveTaskDependency(graph, trigger).Execute(context.Background(), first.ID, second.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskDependency{stored}, graph.edges)
	assert.Zero(t, trigger.calls)
}
