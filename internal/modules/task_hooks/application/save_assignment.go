package application

import (
	"context"
	"fmt"
	"strings"

	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// SaveAssignmentInput is the form of an assignment. An empty ID creates it; a given ID edits it.
type SaveAssignmentInput struct {
	ID          string
	HookID      string
	WorkspaceID string
	Event       models.Event
	Phase       models.Phase
	Enabled     bool
	OnFailure   models.FailurePolicy
}

// SaveAssignment attaches a library hook to the event and phase of a workspace, or edits such an assignment.
type SaveAssignment struct {
	hooks       outbound.HookRepository
	assignments outbound.AssignmentRepository
	ids         outbound.IDGenerator
}

// NewSaveAssignment builds the use case.
func NewSaveAssignment(hooks outbound.HookRepository, assignments outbound.AssignmentRepository, ids outbound.IDGenerator) *SaveAssignment {
	return &SaveAssignment{hooks: hooks, assignments: assignments, ids: ids}
}

// Execute saves the assignment and returns it. An unknown hook is ErrHookNotFound, an unknown assignment is
// ErrAssignmentNotFound, and a repeated hook, event and phase for one workspace is ErrInvalidHook.
func (u *SaveAssignment) Execute(ctx context.Context, in SaveAssignmentInput) (models.Assignment, error) {
	if _, err := u.hooks.Get(ctx, in.HookID); err != nil {
		return models.Assignment{}, fmt.Errorf("load hook %s: %w", in.HookID, err)
	}
	id := strings.TrimSpace(in.ID)
	position := 0
	if id == "" {
		id = u.ids.NewID()
	} else {
		existing, err := u.assignments.Get(ctx, id)
		if err != nil {
			return models.Assignment{}, fmt.Errorf("load assignment %s: %w", id, err)
		}
		position = existing.Position()
	}
	assignment, err := models.NewAssignment(models.AssignmentParams{
		ID: id, HookID: in.HookID, WorkspaceID: in.WorkspaceID, Event: in.Event, Phase: in.Phase,
		Enabled: in.Enabled, OnFailure: in.OnFailure, Position: position,
	})
	if err != nil {
		return models.Assignment{}, fmt.Errorf("build assignment: %w", err)
	}
	if err := u.assignments.Save(ctx, assignment); err != nil {
		return models.Assignment{}, fmt.Errorf("save assignment %s: %w", id, err)
	}
	return assignment, nil
}
