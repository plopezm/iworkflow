package application_test

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: set_task_position::stores_finite_positions
func TestSetTaskPositionStoresFinitePositions(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	graph := &fakeGraph{}

	err := application.NewSetTaskPosition(graph).Execute(context.Background(), task.ID, 3.0, -4.0)

	require.NoError(t, err)
	assert.Equal(t, []models.TaskPosition{{TaskID: task.ID, Position: models.NodePosition{X: 3.0, Y: -4.0}}}, graph.positions)
}

// Rust: set_task_position::rejects_non_finite_positions_without_storing
func TestSetTaskPositionRejectsNonFinitePositionsWithoutStoring(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusBacklog)
	existing := models.TaskPosition{TaskID: task.ID, Position: models.NodePosition{X: 1.0, Y: 1.0}}
	graph := &fakeGraph{positions: []models.TaskPosition{existing}}

	err := application.NewSetTaskPosition(graph).Execute(context.Background(), task.ID, math.NaN(), 0.0)

	require.ErrorIs(t, err, werrors.InvalidPosition())
	assert.Zero(t, graph.sets)
	assert.Equal(t, []models.TaskPosition{existing}, graph.positions)
}
