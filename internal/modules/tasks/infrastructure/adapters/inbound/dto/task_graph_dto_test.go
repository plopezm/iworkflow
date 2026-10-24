package dto_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// graphTaskID returns a fresh task identifier for the graph tests.
func graphTaskID(t *testing.T) models.TaskID {
	t.Helper()
	id, err := models.NewTaskID()
	require.NoError(t, err)
	return id
}

// Rust: task_graph_dto::maps_graph_to_dto
func TestTaskGraphFromDomainMapsDependenciesAndPositions(t *testing.T) {
	t.Parallel()
	a := graphTaskID(t)
	b := graphTaskID(t)
	graph := models.TaskGraph{
		Dependencies: []models.TaskDependency{{Predecessor: a, Successor: b, WorkspaceID: "w", Trigger: models.DependencyTriggerMerged}},
		Positions:    []models.TaskPosition{{TaskID: a, Position: models.NodePosition{X: 1.5, Y: -2.0}}},
	}

	got := dto.TaskGraphFromDomain(graph)

	assert.Equal(t, []dto.TaskDependencyDTO{{PredecessorID: a.String(), SuccessorID: b.String(), Trigger: "merged"}}, got.Dependencies)
	assert.Equal(t, []dto.NodePositionDTO{{TaskID: a.String(), X: 1.5, Y: -2.0}}, got.Positions)
}

// Triangulation (no Rust counterpart): an empty graph is sent as empty arrays, as serde sends an empty Vec.
// A nil slice would be sent as null, which the board does not expect.
func TestTaskGraphFromDomainEmptyGraphSerializesAsEmptyArrays(t *testing.T) {
	t.Parallel()
	body, err := json.Marshal(dto.TaskGraphFromDomain(models.TaskGraph{}))
	require.NoError(t, err)

	assert.JSONEq(t, `{"dependencies":[],"positions":[]}`, string(body))
}

// Triangulation (no Rust counterpart): the wire names are the serde names of the Rust DTO (snake_case fields).
func TestTaskGraphJSONUsesTheRustFieldNames(t *testing.T) {
	t.Parallel()
	a := graphTaskID(t)
	b := graphTaskID(t)
	graph := models.TaskGraph{
		Dependencies: []models.TaskDependency{{Predecessor: a, Successor: b, WorkspaceID: "w", Trigger: models.DependencyTriggerMerged}},
		Positions:    []models.TaskPosition{{TaskID: b, Position: models.NodePosition{X: 3, Y: 4}}},
	}

	body, err := json.Marshal(dto.TaskGraphFromDomain(graph))
	require.NoError(t, err)

	assert.JSONEq(t, `{
		"dependencies": [{"predecessor_id": "`+a.String()+`", "successor_id": "`+b.String()+`", "trigger": "merged"}],
		"positions": [{"task_id": "`+b.String()+`", "x": 3, "y": 4}]
	}`, string(body))
}
