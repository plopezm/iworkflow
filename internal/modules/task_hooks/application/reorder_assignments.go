package application

import (
	"context"
	"fmt"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// ReorderAssignments sets the order of the assignments of one workspace, event and phase. The order must name each
// assignment of the group exactly once: a partial list would leave the positions of the rest ambiguous.
type ReorderAssignments struct {
	assignments outbound.AssignmentRepository
}

// NewReorderAssignments builds the use case.
func NewReorderAssignments(assignments outbound.AssignmentRepository) *ReorderAssignments {
	return &ReorderAssignments{assignments: assignments}
}

// Execute stores the order. A list that is incomplete, repeats an id or names an assignment of another group is
// ErrInvalidHook, and nothing changes.
func (u *ReorderAssignments) Execute(ctx context.Context, workspaceID string, event models.Event, phase models.Phase, ids []string) error {
	current, err := u.assignments.ListFor(ctx, workspaceID, event, phase)
	if err != nil {
		return fmt.Errorf("list assignments: %w", err)
	}
	if len(ids) != len(current) {
		return fmt.Errorf("%w: the order must name the %d assignments of the group", hookerrors.ErrInvalidHook, len(current))
	}
	known := make(map[string]bool, len(current))
	for _, item := range current {
		known[item.Assignment.ID()] = true
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !known[id] {
			return fmt.Errorf("%w: assignment %s is not in the group", hookerrors.ErrInvalidHook, id)
		}
		if seen[id] {
			return fmt.Errorf("%w: assignment %s is listed twice", hookerrors.ErrInvalidHook, id)
		}
		seen[id] = true
	}
	if err := u.assignments.Reorder(ctx, workspaceID, event, phase, ids); err != nil {
		return fmt.Errorf("reorder assignments: %w", err)
	}
	return nil
}
