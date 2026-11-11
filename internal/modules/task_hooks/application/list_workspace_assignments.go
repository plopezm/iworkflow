package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// ListWorkspaceAssignments returns what runs for a workspace, grouped by event and phase in position order.
type ListWorkspaceAssignments struct {
	assignments outbound.AssignmentRepository
}

// NewListWorkspaceAssignments builds the use case.
func NewListWorkspaceAssignments(assignments outbound.AssignmentRepository) *ListWorkspaceAssignments {
	return &ListWorkspaceAssignments{assignments: assignments}
}

// Execute returns the assignments of the workspace with their hooks, enabled or not.
func (u *ListWorkspaceAssignments) Execute(ctx context.Context, workspaceID string) ([]models.AssignedHook, error) {
	assigned, err := u.assignments.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	return assigned, nil
}
