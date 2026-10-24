package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: get_task_graph::combines_edges_and_positions_of_the_trimmed_workspace
func TestGetTaskGraphCombinesEdgesAndPositionsOfTheTrimmedWorkspace(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	position := models.TaskPosition{TaskID: task.ID, Position: models.NodePosition{X: 1.0, Y: 2.0}}
	graph := &fakeGraph{positions: []models.TaskPosition{position}}

	result, err := application.NewGetTaskGraph(graph).Execute(context.Background(), " ws-1 ")

	require.NoError(t, err)
	assert.Equal(t, []models.TaskPosition{position}, result.Positions)
	assert.Empty(t, result.Dependencies)
	workspace := "ws-1"
	assert.Equal(t, []*string{&workspace}, graph.dependencyQueries)
	assert.Equal(t, []string{"ws-1"}, graph.positionQueries)
}
