// Package persistence implements the preferences repository port on top of SQLite (sqlc).
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
	"iworkflow/internal/modules/preferences/domain/models"
	"iworkflow/internal/modules/preferences/domain/ports/outbound"
	"iworkflow/internal/modules/preferences/infrastructure/adapters/outbound/persistence/sqlcgen"
)

// Keys of the app_preferences table.
const (
	languageKey      = "language"
	networkAccessKey = "network_access"
	listenPortKey    = "listen_port"
	requireTokenKey  = "require_access_token"
	accessTokenKey   = "access_token"
)

// Stored booleans. Anything other than the explicit value counts as the default.
const (
	enabledValue  = "true"
	disabledValue = "false"
)

// PreferencesRepository stores each preference as one row of app_preferences.
type PreferencesRepository struct {
	queries *sqlcgen.Queries
}

var _ outbound.PreferencesRepository = (*PreferencesRepository)(nil)

// NewPreferencesRepository builds the repository on an open database handle.
func NewPreferencesRepository(db *sql.DB) *PreferencesRepository {
	return &PreferencesRepository{queries: sqlcgen.New(db)}
}

// Language returns the stored language; a value this build does not know counts as unset.
func (r *PreferencesRepository) Language(ctx context.Context) (*models.UILanguage, error) {
	value, found, err := r.lookup(ctx, languageKey)
	if err != nil || !found {
		return nil, err
	}
	language, parseErr := models.ParseUILanguage(value)
	if parseErr != nil {
		// Written by a newer version: behave as if nothing was chosen.
		return nil, nil
	}
	return &language, nil
}

// SaveLanguage stores the language code.
func (r *PreferencesRepository) SaveLanguage(ctx context.Context, language models.UILanguage) error {
	return r.store(ctx, languageKey, language.Code())
}

// NetworkAccess reports true only for an explicit opt-in; anything else keeps the server private.
func (r *PreferencesRepository) NetworkAccess(ctx context.Context) (bool, error) {
	value, found, err := r.lookup(ctx, networkAccessKey)
	if err != nil {
		return false, err
	}
	return found && value == enabledValue, nil
}

// SaveNetworkAccess stores the flag as "true" or "false".
func (r *PreferencesRepository) SaveNetworkAccess(ctx context.Context, enabled bool) error {
	return r.store(ctx, networkAccessKey, flagValue(enabled))
}

// ListenPort returns the stored port; a damaged value counts as unset so the default port applies.
func (r *PreferencesRepository) ListenPort(ctx context.Context) (*models.ListenPort, error) {
	value, found, err := r.lookup(ctx, listenPortKey)
	if err != nil || !found {
		return nil, err
	}
	port, parseErr := models.ParseListenPort(value)
	if parseErr != nil {
		return nil, nil
	}
	return &port, nil
}

// SaveListenPort stores the port number.
func (r *PreferencesRepository) SaveListenPort(ctx context.Context, port models.ListenPort) error {
	return r.store(ctx, listenPortKey, strconv.FormatUint(uint64(port.Port()), 10))
}

// RequireAccessToken drops the requirement only on an explicit opt-out; missing or unknown values
// keep the token required.
func (r *PreferencesRepository) RequireAccessToken(ctx context.Context) (bool, error) {
	value, found, err := r.lookup(ctx, requireTokenKey)
	if err != nil {
		return false, err
	}
	return !found || value != disabledValue, nil
}

// SaveRequireAccessToken stores the flag as "true" or "false".
func (r *PreferencesRepository) SaveRequireAccessToken(ctx context.Context, required bool) error {
	return r.store(ctx, requireTokenKey, flagValue(required))
}

// AccessToken returns the stored token; a damaged value counts as missing so a fresh one is made.
func (r *PreferencesRepository) AccessToken(ctx context.Context) (*models.AccessToken, error) {
	value, found, err := r.lookup(ctx, accessTokenKey)
	if err != nil || !found {
		return nil, err
	}
	token, parseErr := models.ParseAccessToken(value)
	if parseErr != nil {
		return nil, nil
	}
	return &token, nil
}

// SaveAccessToken stores the token value.
func (r *PreferencesRepository) SaveAccessToken(ctx context.Context, token models.AccessToken) error {
	return r.store(ctx, accessTokenKey, token.Value())
}

// lookup reads one preference; found is false when the row does not exist.
func (r *PreferencesRepository) lookup(ctx context.Context, key string) (string, bool, error) {
	row, err := r.queries.GetPreference(ctx, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, &domainerrors.RepositoryError{Cause: err}
	}
	return row.Value, true, nil
}

// store upserts one preference row.
func (r *PreferencesRepository) store(ctx context.Context, key string, value string) error {
	err := r.queries.UpsertPreference(ctx, sqlcgen.UpsertPreferenceParams{Key: key, Value: value})
	if err != nil {
		return &domainerrors.RepositoryError{Cause: err}
	}
	return nil
}

func flagValue(enabled bool) string {
	if enabled {
		return enabledValue
	}
	return disabledValue
}
