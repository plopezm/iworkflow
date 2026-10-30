//nolint:wrapcheck // Port errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"
	"strings"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// GetTaskGraph returns the edges and the saved node positions of the board of one workspace.
type GetTaskGraph struct {
	graph outbound.TaskGraphRepository
}

// NewGetTaskGraph builds the use case.
func NewGetTaskGraph(graph outbound.TaskGraphRepository) *GetTaskGraph {
	return &GetTaskGraph{graph: graph}
}

// Execute reads the graph of the trimmed workspace identifier.
func (u *GetTaskGraph) Execute(ctx context.Context, workspaceID string) (models.TaskGraph, error) {
	trimmed := strings.TrimSpace(workspaceID)
	dependencies, err := u.graph.ListDependencies(ctx, &trimmed)
	if err != nil {
		return models.TaskGraph{}, err
	}
	positions, err := u.graph.ListPositions(ctx, trimmed)
	if err != nil {
		return models.TaskGraph{}, err
	}
	return models.TaskGraph{Dependencies: dependencies, Positions: positions}, nil
}
