package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence/gen"
)

// pruneMergedKey is the app_preferences key of the prune switch. The value is "false" when it is off.
const pruneMergedKey = "tasks.prune_merged"

// PrunePreferenceRepository stores the prune switch as one row of app_preferences.
type PrunePreferenceRepository struct {
	queries *gen.Queries
}

// NewPrunePreferenceRepository builds the repository on db.
func NewPrunePreferenceRepository(db *sql.DB) *PrunePreferenceRepository {
	return &PrunePreferenceRepository{queries: gen.New(db)}
}

// Enabled is true unless the switch was stored as "false".
func (r *PrunePreferenceRepository) Enabled(ctx context.Context) (bool, error) {
	value, err := r.queries.GetTaskPreference(ctx, pruneMergedKey)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("read the merged tasks prune switch: %w", err)
	}
	return value != "false", nil
}

// SetEnabled stores the switch.
func (r *PrunePreferenceRepository) SetEnabled(ctx context.Context, enabled bool) error {
	value := "true"
	if !enabled {
		value = "false"
	}
	if err := r.queries.UpsertTaskPreference(ctx, gen.UpsertTaskPreferenceParams{Key: pruneMergedKey, Value: value}); err != nil {
		return fmt.Errorf("store the merged tasks prune switch: %w", err)
	}
	return nil
}
