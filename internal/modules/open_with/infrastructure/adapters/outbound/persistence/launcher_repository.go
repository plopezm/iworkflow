// Package persistence implements the launcher repository port on SQLite through sqlc.
package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	openerrors "iworkflow/internal/modules/open_with/domain/errors"
	"iworkflow/internal/modules/open_with/domain/models"
	"iworkflow/internal/modules/open_with/domain/ports/outbound"
	"iworkflow/internal/modules/open_with/infrastructure/adapters/outbound/persistence/gen"
)

// LauncherRepository stores the launchers in the open_with_launchers table. The arguments and the
// environment are stored as JSON text.
type LauncherRepository struct {
	queries *gen.Queries
}

var _ outbound.LauncherRepository = (*LauncherRepository)(nil)

// NewLauncherRepository builds the repository on an open database.
func NewLauncherRepository(db *sql.DB) *LauncherRepository {
	return &LauncherRepository{queries: gen.New(db)}
}

type envRecord struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// List returns the launchers in creation order.
func (r *LauncherRepository) List(ctx context.Context) ([]models.Launcher, error) {
	rows, err := r.queries.ListLaunchers(ctx)
	if err != nil {
		return nil, repositoryError(err)
	}
	launchers := make([]models.Launcher, 0, len(rows))
	for _, row := range rows {
		launcher, err := toDomain(row)
		if err != nil {
			return nil, err
		}
		launchers = append(launchers, launcher)
	}
	return launchers, nil
}

// Get returns the launcher or errors.ErrLauncherNotFound.
func (r *LauncherRepository) Get(ctx context.Context, id string) (models.Launcher, error) {
	row, err := r.queries.GetLauncher(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Launcher{}, openerrors.ErrLauncherNotFound
	}
	if err != nil {
		return models.Launcher{}, repositoryError(err)
	}
	return toDomain(row)
}

// Save inserts a new launcher at the end of the list.
func (r *LauncherRepository) Save(ctx context.Context, launcher models.Launcher) error {
	args, err := json.Marshal(launcher.Args())
	if err != nil {
		return repositoryError(err)
	}
	records := make([]envRecord, 0, len(launcher.Env()))
	for _, variable := range launcher.Env() {
		records = append(records, envRecord{Name: variable.Name, Value: variable.Value})
	}
	env, err := json.Marshal(records)
	if err != nil {
		return repositoryError(err)
	}
	err = r.queries.InsertLauncher(ctx, gen.InsertLauncherParams{
		ID: launcher.ID(), Name: launcher.Name(), Path: launcher.Path(), Args: string(args), Env: string(env),
	})
	if err != nil {
		return repositoryError(err)
	}
	return nil
}

// Delete removes the launcher or reports errors.ErrLauncherNotFound.
func (r *LauncherRepository) Delete(ctx context.Context, id string) error {
	removed, err := r.queries.DeleteLauncher(ctx, id)
	if err != nil {
		return repositoryError(err)
	}
	if removed == 0 {
		return openerrors.ErrLauncherNotFound
	}
	return nil
}

// toDomain rebuilds the launcher through its constructor, so a stored row that breaks an invariant is reported.
func toDomain(row gen.OpenWithLauncher) (models.Launcher, error) {
	var args []string
	if err := json.Unmarshal([]byte(row.Args), &args); err != nil {
		return models.Launcher{}, repositoryError(err)
	}
	var records []envRecord
	if err := json.Unmarshal([]byte(row.Env), &records); err != nil {
		return models.Launcher{}, repositoryError(err)
	}
	env := make([]models.EnvVar, 0, len(records))
	for _, record := range records {
		env = append(env, models.EnvVar{Name: record.Name, Value: record.Value})
	}
	launcher, err := models.NewLauncher(row.ID, row.Name, row.Path, args, env)
	if err != nil {
		return models.Launcher{}, fmt.Errorf("stored launcher %s: %w", row.ID, err)
	}
	return launcher, nil
}

func repositoryError(err error) error {
	return fmt.Errorf("%w: %w", openerrors.ErrRepository, err)
}
