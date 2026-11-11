package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence/gen"
)

// AssignmentRepository stores the assignments of the library hooks to workspaces.
type AssignmentRepository struct {
	db      *sql.DB
	queries *gen.Queries
}

var _ outbound.AssignmentRepository = (*AssignmentRepository)(nil)

// NewAssignmentRepository builds the repository on an open database.
func NewAssignmentRepository(db *sql.DB) *AssignmentRepository {
	return &AssignmentRepository{db: db, queries: gen.New(db)}
}

// assignedColumns are the joined columns of an assignment and its hook.
type assignedColumns struct {
	hookColumns
	AssignmentID string
	HookID       string
	WorkspaceID  string
	Event        string
	Phase        string
	Position     int64
	Enabled      int64
	OnFailure    string
}

// ListByWorkspace returns every assignment of a workspace with its hook.
func (r *AssignmentRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]models.AssignedHook, error) {
	rows, err := r.queries.ListWorkspaceAssignments(ctx, workspaceID)
	if err != nil {
		return nil, repositoryError(err)
	}
	assigned := make([]models.AssignedHook, 0, len(rows))
	for _, row := range rows {
		item, err := assignedFromColumns(assignedColumns{
			hookColumns: hookColumns{
				ID: row.HookID, Name: row.Name, Kind: row.Kind, TimeoutSeconds: row.TimeoutSeconds, Command: row.Command,
				Args: row.Args, Env: row.Env, UseShell: row.UseShell, WorkingDir: row.WorkingDir, Prompt: row.Prompt,
				Runtime: row.Runtime, Model: row.Model, Thinking: row.Thinking, Profile: row.Profile,
			},
			AssignmentID: row.AssignmentID, HookID: row.HookID, WorkspaceID: row.WorkspaceID, Event: row.Event,
			Phase: row.Phase, Position: row.Position, Enabled: row.Enabled, OnFailure: row.OnFailure,
		})
		if err != nil {
			return nil, err
		}
		assigned = append(assigned, item)
	}
	return assigned, nil
}

// ListFor returns the assignments of a workspace attached to one event and phase, in position order.
func (r *AssignmentRepository) ListFor(ctx context.Context, workspaceID string, event models.Event, phase models.Phase) ([]models.AssignedHook, error) {
	rows, err := r.queries.ListAssignmentsFor(ctx, gen.ListAssignmentsForParams{
		WorkspaceID: workspaceID, Event: string(event), Phase: string(phase),
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	assigned := make([]models.AssignedHook, 0, len(rows))
	for _, row := range rows {
		item, err := assignedFromColumns(assignedColumns{
			hookColumns: hookColumns{
				ID: row.HookID, Name: row.Name, Kind: row.Kind, TimeoutSeconds: row.TimeoutSeconds, Command: row.Command,
				Args: row.Args, Env: row.Env, UseShell: row.UseShell, WorkingDir: row.WorkingDir, Prompt: row.Prompt,
				Runtime: row.Runtime, Model: row.Model, Thinking: row.Thinking, Profile: row.Profile,
			},
			AssignmentID: row.AssignmentID, HookID: row.HookID, WorkspaceID: row.WorkspaceID, Event: row.Event,
			Phase: row.Phase, Position: row.Position, Enabled: row.Enabled, OnFailure: row.OnFailure,
		})
		if err != nil {
			return nil, err
		}
		assigned = append(assigned, item)
	}
	return assigned, nil
}

// ListByHook returns the assignments of one hook.
func (r *AssignmentRepository) ListByHook(ctx context.Context, hookID string) ([]models.Assignment, error) {
	rows, err := r.queries.ListAssignmentsByHook(ctx, hookID)
	if err != nil {
		return nil, repositoryError(err)
	}
	assignments := make([]models.Assignment, 0, len(rows))
	for _, row := range rows {
		assignment, err := assignmentFromRow(row)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, assignment)
	}
	return assignments, nil
}

// Get returns the assignment or errors.ErrAssignmentNotFound.
func (r *AssignmentRepository) Get(ctx context.Context, id string) (models.Assignment, error) {
	row, err := r.queries.GetAssignment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Assignment{}, hookerrors.ErrAssignmentNotFound
	}
	if err != nil {
		return models.Assignment{}, repositoryError(err)
	}
	return assignmentFromRow(row)
}

// Save inserts the assignment at the end of its group, or updates the existing one. Moving an assignment to another
// event or phase keeps its old position number, which is only an order within the new group.
func (r *AssignmentRepository) Save(ctx context.Context, assignment models.Assignment) error {
	err := r.queries.UpsertAssignment(ctx, gen.UpsertAssignmentParams{
		ID: assignment.ID(), HookID: assignment.HookID(), WorkspaceID: assignment.WorkspaceID(),
		Event: string(assignment.Event()), Phase: string(assignment.Phase()),
		Enabled: boolToInt(assignment.Enabled()), OnFailure: string(assignment.OnFailure()),
	})
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("%w: the hook is already assigned to that project, event and phase", hookerrors.ErrInvalidHook)
		}
		if strings.Contains(err.Error(), "FOREIGN KEY constraint failed") {
			return fmt.Errorf("%w: the hook or the project does not exist", hookerrors.ErrInvalidHook)
		}
		return repositoryError(err)
	}
	return nil
}

// Delete removes the assignment or returns errors.ErrAssignmentNotFound.
func (r *AssignmentRepository) Delete(ctx context.Context, id string) error {
	affected, err := r.queries.DeleteAssignment(ctx, id)
	if err != nil {
		return repositoryError(err)
	}
	if affected == 0 {
		return hookerrors.ErrAssignmentNotFound
	}
	return nil
}

// Reorder sets the positions of one group to the order of ids, atomically.
func (r *AssignmentRepository) Reorder(ctx context.Context, workspaceID string, event models.Event, phase models.Phase, ids []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	queries := r.queries.WithTx(tx)
	for index, id := range ids {
		if err := queries.SetAssignmentPosition(ctx, gen.SetAssignmentPositionParams{
			Position: int64(index + 1), ID: id, WorkspaceID: workspaceID, Event: string(event), Phase: string(phase),
		}); err != nil {
			return rollback(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return repositoryError(err)
	}
	return nil
}

func rollback(tx *sql.Tx, cause error) error {
	if err := tx.Rollback(); err != nil {
		return repositoryError(fmt.Errorf("%w (rollback: %w)", cause, err))
	}
	return repositoryError(cause)
}

func assignedFromColumns(row assignedColumns) (models.AssignedHook, error) {
	hook, err := hookFromColumns(row.hookColumns)
	if err != nil {
		return models.AssignedHook{}, err
	}
	assignment, err := models.NewAssignment(models.AssignmentParams{
		ID: row.AssignmentID, HookID: row.HookID, WorkspaceID: row.WorkspaceID, Event: models.Event(row.Event),
		Phase: models.Phase(row.Phase), Enabled: row.Enabled != 0, OnFailure: models.FailurePolicy(row.OnFailure),
		Position: int(row.Position),
	})
	if err != nil {
		return models.AssignedHook{}, repositoryError(err)
	}
	return models.AssignedHook{Assignment: assignment, Hook: hook}, nil
}

func assignmentFromRow(row gen.HookAssignment) (models.Assignment, error) {
	assignment, err := models.NewAssignment(models.AssignmentParams{
		ID: row.ID, HookID: row.HookID, WorkspaceID: row.WorkspaceID, Event: models.Event(row.Event),
		Phase: models.Phase(row.Phase), Enabled: row.Enabled != 0, OnFailure: models.FailurePolicy(row.OnFailure),
		Position: int(row.Position),
	})
	if err != nil {
		return models.Assignment{}, repositoryError(err)
	}
	return assignment, nil
}
