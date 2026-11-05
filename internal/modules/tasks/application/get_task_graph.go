//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"strings"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// GetTaskGraph returns the edges and the saved node positions of one workspace, or of the whole global graph
// when the workspace identifier is empty.
type GetTaskGraph struct {
	graph outbound.TaskGraphRepository
}

// NewGetTaskGraph builds the use case.
func NewGetTaskGraph(graph outbound.TaskGraphRepository) *GetTaskGraph {
	return &GetTaskGraph{graph: graph}
}

// Execute reads the graph of the trimmed workspace identifier; empty means every workspace.
func (u *GetTaskGraph) Execute(ctx context.Context, workspaceID string) (models.TaskGraph, error) {
	var filter *string
	if trimmed := strings.TrimSpace(workspaceID); trimmed != "" {
		filter = &trimmed
	}
	dependencies, err := u.graph.ListDependencies(ctx, filter)
	if err != nil {
		return models.TaskGraph{}, err
	}
	positions, err := u.graph.ListPositions(ctx, filter)
	if err != nil {
		return models.TaskGraph{}, err
	}
	return models.TaskGraph{Dependencies: dependencies, Positions: positions}, nil
}
