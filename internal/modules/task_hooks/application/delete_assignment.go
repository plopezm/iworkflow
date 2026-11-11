package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// DeleteAssignment detaches a hook from a workspace, event and phase. The hook stays in the library.
type DeleteAssignment struct {
	assignments outbound.AssignmentRepository
}

// NewDeleteAssignment builds the use case.
func NewDeleteAssignment(assignments outbound.AssignmentRepository) *DeleteAssignment {
	return &DeleteAssignment{assignments: assignments}
}

// Execute deletes the assignment. An unknown id is ErrAssignmentNotFound.
func (u *DeleteAssignment) Execute(ctx context.Context, id string) error {
	if err := u.assignments.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete assignment %s: %w", id, err)
	}
	return nil
}
