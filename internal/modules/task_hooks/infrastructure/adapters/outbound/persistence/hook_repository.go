// Package persistence implements the hook repository ports on SQLite through sqlc.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/outbound/persistence/gen"
)

// HookRepository stores the hooks and the per-task skips. Arguments and environment are stored as JSON text.
type HookRepository struct {
	db      *sql.DB
	queries *gen.Queries
}

var (
	_ outbound.HookRepository = (*HookRepository)(nil)
	_ outbound.SkipRepository = (*HookRepository)(nil)
)

// NewHookRepository builds the repository on an open database.
func NewHookRepository(db *sql.DB) *HookRepository {
	return &HookRepository{db: db, queries: gen.New(db)}
}

type envRecord struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// List returns the hooks of a workspace in position order.
func (r *HookRepository) List(ctx context.Context, workspaceID string) ([]models.Hook, error) {
	rows, err := r.queries.ListWorkspaceHooks(ctx, workspaceID)
	if err != nil {
		return nil, repositoryError(err)
	}
	return toDomainAll(rows)
}

// ListFor returns the hooks of a workspace attached to one event and phase, in position order.
func (r *HookRepository) ListFor(ctx context.Context, workspaceID string, event models.Event, phase models.Phase) ([]models.Hook, error) {
	rows, err := r.queries.ListHooksFor(ctx, gen.ListHooksForParams{
		WorkspaceID: workspaceID, Event: string(event), Phase: string(phase),
	})
	if err != nil {
		return nil, repositoryError(err)
	}
	return toDomainAll(rows)
}

// Get returns the hook or errors.ErrHookNotFound.
func (r *HookRepository) Get(ctx context.Context, id string) (models.Hook, error) {
	row, err := r.queries.GetHook(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Hook{}, hookerrors.ErrHookNotFound
	}
	if err != nil {
		return models.Hook{}, repositoryError(err)
	}
	return toDomain(row)
}

// Save inserts a new hook at the end of its workspace list, or updates the existing one keeping its position.
func (r *HookRepository) Save(ctx context.Context, hook models.Hook) error {
	script := hook.Script()
	args, err := json.Marshal(script.Args)
	if err != nil {
		return repositoryError(err)
	}
	records := make([]envRecord, 0, len(script.Env))
	for _, variable := range script.Env {
		records = append(records, envRecord{Name: variable.Name, Value: variable.Value})
	}
	env, err := json.Marshal(records)
	if err != nil {
		return repositoryError(err)
	}
	agent := hook.Agent()
	err = r.queries.UpsertHook(ctx, gen.UpsertHookParams{
		ID: hook.ID(), WorkspaceID: hook.WorkspaceID(), Phase: string(hook.Phase()), Event: string(hook.Event()),
		Kind: string(hook.Kind()), Enabled: boolToInt(hook.Enabled()), OnFailure: string(hook.OnFailure()),
		TimeoutSeconds: int64(hook.Timeout() / time.Second),
		Command:        script.Command, Args: string(args), Env: string(env), UseShell: boolToInt(script.UseShell),
		Prompt: agent.Prompt, Runtime: agent.Runtime, Model: agent.Model, Thinking: agent.Thinking, Profile: agent.Profile,
	})
	if err != nil {
		return repositoryError(err)
	}
	return nil
}

// Delete removes the hook or returns errors.ErrHookNotFound.
func (r *HookRepository) Delete(ctx context.Context, id string) error {
	affected, err := r.queries.DeleteHook(ctx, id)
	if err != nil {
		return repositoryError(err)
	}
	if affected == 0 {
		return hookerrors.ErrHookNotFound
	}
	return nil
}

// Reorder sets the positions of the workspace hooks to the order of ids, atomically.
func (r *HookRepository) Reorder(ctx context.Context, workspaceID string, ids []string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return repositoryError(err)
	}
	queries := r.queries.WithTx(tx)
	for index, id := range ids {
		if err := queries.SetHookPosition(ctx, gen.SetHookPositionParams{
			Position: int64(index + 1), ID: id, WorkspaceID: workspaceID,
		}); err != nil {
			return rollback(tx, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return repositoryError(err)
	}
	return nil
}

// SetSkipped disables or re-enables one hook for one task.
func (r *HookRepository) SetSkipped(ctx context.Context, taskID string, hookID string, skipped bool) error {
	var err error
	if skipped {
		err = r.queries.InsertHookSkip(ctx, gen.InsertHookSkipParams{TaskID: taskID, HookID: hookID})
	} else {
		err = r.queries.DeleteHookSkip(ctx, gen.DeleteHookSkipParams{TaskID: taskID, HookID: hookID})
	}
	if err != nil {
		return repositoryError(err)
	}
	return nil
}

// SkippedHookIDs returns the ids of the hooks disabled for the task.
func (r *HookRepository) SkippedHookIDs(ctx context.Context, taskID string) ([]string, error) {
	ids, err := r.queries.ListHookSkips(ctx, taskID)
	if err != nil {
		return nil, repositoryError(err)
	}
	return ids, nil
}

func rollback(tx *sql.Tx, cause error) error {
	if err := tx.Rollback(); err != nil {
		return repositoryError(fmt.Errorf("%w (rollback: %w)", cause, err))
	}
	return repositoryError(cause)
}

func toDomainAll(rows []gen.TaskHook) ([]models.Hook, error) {
	hooks := make([]models.Hook, 0, len(rows))
	for _, row := range rows {
		hook, err := toDomain(row)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, hook)
	}
	return hooks, nil
}

func toDomain(row gen.TaskHook) (models.Hook, error) {
	var args []string
	if err := json.Unmarshal([]byte(row.Args), &args); err != nil {
		return models.Hook{}, repositoryError(fmt.Errorf("decode hook %s args: %w", row.ID, err))
	}
	var records []envRecord
	if err := json.Unmarshal([]byte(row.Env), &records); err != nil {
		return models.Hook{}, repositoryError(fmt.Errorf("decode hook %s env: %w", row.ID, err))
	}
	env := make([]models.EnvVar, 0, len(records))
	for _, record := range records {
		env = append(env, models.EnvVar{Name: record.Name, Value: record.Value})
	}
	hook, err := models.NewHook(models.HookParams{
		ID: row.ID, WorkspaceID: row.WorkspaceID, Phase: models.Phase(row.Phase), Event: models.Event(row.Event),
		Kind: models.Kind(row.Kind), Enabled: row.Enabled != 0, OnFailure: models.FailurePolicy(row.OnFailure),
		Timeout: time.Duration(row.TimeoutSeconds) * time.Second,
		Script:  models.ScriptSpec{Command: row.Command, Args: args, Env: env, UseShell: row.UseShell != 0},
		Agent: models.AgentSpec{
			Prompt: row.Prompt, Runtime: row.Runtime, Model: row.Model, Thinking: row.Thinking, Profile: row.Profile,
		},
	})
	if err != nil {
		return models.Hook{}, repositoryError(err)
	}
	return hook, nil
}

func boolToInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}

func repositoryError(err error) error {
	return fmt.Errorf("%w: %w", hookerrors.ErrRepository, err)
}
