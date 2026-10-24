//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package application

import (
	"context"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/domain/ports/outbound"
)

// SetTaskPosition saves where the user left the node of a task on the board canvas.
type SetTaskPosition struct {
	graph outbound.TaskGraphRepository
}

// NewSetTaskPosition builds the use case.
func NewSetTaskPosition(graph outbound.TaskGraphRepository) *SetTaskPosition {
	return &SetTaskPosition{graph: graph}
}

// Execute validates the coordinates and stores them. Non-finite coordinates are rejected before any write.
func (u *SetTaskPosition) Execute(ctx context.Context, id models.TaskID, x float64, y float64) error {
	position, err := models.NewNodePosition(x, y)
	if err != nil {
		return err
	}
	return u.graph.SetPosition(ctx, id, position)
}
