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

// HookRepository stores the hooks of the library and the per-task skips. Arguments and environment are stored as
// JSON text.
type HookRepository struct {
	queries *gen.Queries
}

var (
	_ outbound.HookRepository = (*HookRepository)(nil)
	_ outbound.SkipRepository = (*HookRepository)(nil)
)

// NewHookRepository builds the repository on an open database.
func NewHookRepository(db *sql.DB) *HookRepository {
	return &HookRepository{queries: gen.New(db)}
}

type envRecord struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// List returns the hooks of the library ordered by name.
func (r *HookRepository) List(ctx context.Context) ([]models.Hook, error) {
	rows, err := r.queries.ListHooks(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	hooks := make([]models.Hook, 0, len(rows))
	for _, row := range rows {
		hook, err := hookFromColumns(hookColumns{
			ID: row.ID, Name: row.Name, Kind: row.Kind, TimeoutSeconds: row.TimeoutSeconds, Command: row.Command,
			Args: row.Args, Env: row.Env, UseShell: row.UseShell, WorkingDir: row.WorkingDir, Prompt: row.Prompt,
			Runtime: row.Runtime, Model: row.Model, Thinking: row.Thinking, Profile: row.Profile,
		})
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, hook)
	}
	return hooks, nil
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
	return hookFromColumns(hookColumns{
		ID: row.ID, Name: row.Name, Kind: row.Kind, TimeoutSeconds: row.TimeoutSeconds, Command: row.Command,
		Args: row.Args, Env: row.Env, UseShell: row.UseShell, WorkingDir: row.WorkingDir, Prompt: row.Prompt,
		Runtime: row.Runtime, Model: row.Model, Thinking: row.Thinking, Profile: row.Profile,
	})
}

// Save inserts the hook or updates the existing one.
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
		ID: hook.ID(), Name: hook.Name(), Kind: string(hook.Kind()), TimeoutSeconds: int64(hook.Timeout() / time.Second),
		Command: script.Command, Args: string(args), Env: string(env), UseShell: boolToInt(script.UseShell),
		WorkingDir: script.WorkingDir,
		Prompt:     agent.Prompt, Runtime: agent.Runtime, Model: agent.Model, Thinking: agent.Thinking, Profile: agent.Profile,
	})
	if err != nil {
		return repositoryError(err)
	}
	return nil
}

// Delete removes the hook, with its assignments, or returns errors.ErrHookNotFound.
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

// SetSkipped disables or re-enables one assignment for one task.
func (r *HookRepository) SetSkipped(ctx context.Context, taskID string, assignmentID string, skipped bool) error {
	var err error
	if skipped {
		err = r.queries.InsertHookSkip(ctx, gen.InsertHookSkipParams{TaskID: taskID, AssignmentID: assignmentID})
	} else {
		err = r.queries.DeleteHookSkip(ctx, gen.DeleteHookSkipParams{TaskID: taskID, AssignmentID: assignmentID})
	}
	if err != nil {
		return repositoryError(err)
	}
	return nil
}

// SkippedAssignmentIDs returns the ids of the assignments disabled for the task.
func (r *HookRepository) SkippedAssignmentIDs(ctx context.Context, taskID string) ([]string, error) {
	ids, err := r.queries.ListHookSkips(ctx, taskID)
	if err != nil {
		return nil, repositoryError(err)
	}
	return ids, nil
}

// hookColumns are the stored columns of a hook, shared by the queries that read one.
type hookColumns struct {
	ID             string
	Name           string
	Kind           string
	TimeoutSeconds int64
	Command        string
	Args           string
	Env            string
	UseShell       int64
	WorkingDir     string
	Prompt         string
	Runtime        string
	Model          string
	Thinking       string
	Profile        string
}

func hookFromColumns(row hookColumns) (models.Hook, error) {
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
		ID: row.ID, Name: row.Name, Kind: models.Kind(row.Kind), Timeout: time.Duration(row.TimeoutSeconds) * time.Second,
		Script: models.ScriptSpec{Command: row.Command, Args: args, Env: env, UseShell: row.UseShell != 0, WorkingDir: row.WorkingDir},
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
