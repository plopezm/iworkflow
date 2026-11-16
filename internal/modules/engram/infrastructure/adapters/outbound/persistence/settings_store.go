// Package persistence stores the Engram server address in the app_preferences table.
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"iworkflow/internal/modules/engram/domain/ports/outbound"
	"iworkflow/internal/modules/engram/infrastructure/adapters/outbound/persistence/sqlcgen"
)

// urlKey is the app_preferences key of the saved Engram address.
const urlKey = "engram_url"

// SettingsStore implements outbound.SettingsStore over SQLite.
type SettingsStore struct {
	queries *sqlcgen.Queries
}

var _ outbound.SettingsStore = (*SettingsStore)(nil)

// NewSettingsStore builds the store over the shared database.
func NewSettingsStore(db *sql.DB) *SettingsStore {
	return &SettingsStore{queries: sqlcgen.New(db)}
}

// SavedURL returns the saved address, or an empty string when none was saved.
func (s *SettingsStore) SavedURL(ctx context.Context) (string, error) {
	value, err := s.queries.GetSetting(ctx, urlKey)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", urlKey, err)
	}
	return value, nil
}

// SaveURL stores the address; an empty string means the default applies.
func (s *SettingsStore) SaveURL(ctx context.Context, url string) error {
	if err := s.queries.UpsertSetting(ctx, sqlcgen.UpsertSettingParams{Key: urlKey, Value: url}); err != nil {
		return fmt.Errorf("write %s: %w", urlKey, err)
	}
	return nil
}
