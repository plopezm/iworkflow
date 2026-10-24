// Package persistence implements the code_hosting repository port on SQLite through sqlc.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package persistence

import (
	"context"
	"database/sql"
	"errors"

	codeerrors "iworkflow/internal/modules/code_hosting/domain/errors"
	"iworkflow/internal/modules/code_hosting/domain/models"
	"iworkflow/internal/modules/code_hosting/domain/ports/outbound"
	"iworkflow/internal/modules/code_hosting/infrastructure/adapters/outbound/persistence/gen"
)

// CodeHostSettingsRepository stores one row per provider in the code_host_settings table.
type CodeHostSettingsRepository struct {
	queries *gen.Queries
}

var _ outbound.CodeHostSettingsRepository = (*CodeHostSettingsRepository)(nil)

// NewCodeHostSettingsRepository builds the repository on an open database.
func NewCodeHostSettingsRepository(db *sql.DB) *CodeHostSettingsRepository {
	return &CodeHostSettingsRepository{queries: gen.New(db)}
}

// Get returns nil and no error when the provider has no row.
func (r *CodeHostSettingsRepository) Get(ctx context.Context, provider models.HostingProvider) (*models.CodeHostSettings, error) {
	row, err := r.queries.GetCodeHostSettings(ctx, string(provider))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, codeerrors.Repository(err.Error())
	}
	settings, err := rowToDomain(row)
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

// List returns the settings of every provider, ordered by provider.
func (r *CodeHostSettingsRepository) List(ctx context.Context) ([]models.CodeHostSettings, error) {
	rows, err := r.queries.ListCodeHostSettings(ctx)
	if err != nil {
		return nil, codeerrors.Repository(err.Error())
	}
	settings := make([]models.CodeHostSettings, 0, len(rows))
	for _, row := range rows {
		converted, err := rowToDomain(row)
		if err != nil {
			return nil, err
		}
		settings = append(settings, converted)
	}
	return settings, nil
}

// Save inserts the row of the provider or replaces it.
func (r *CodeHostSettingsRepository) Save(ctx context.Context, settings models.CodeHostSettings) error {
	err := r.queries.ReplaceCodeHostSettings(ctx, gen.ReplaceCodeHostSettingsParams{
		Provider:    string(settings.Provider()),
		BaseUrl:     settings.BaseURL(),
		Token:       settings.Token(),
		InsecureTls: boolToInt(settings.InsecureTLS),
	})
	if err != nil {
		return codeerrors.Repository(err.Error())
	}
	return nil
}

// rowToDomain rebuilds the settings through the constructor, so a stored row that violates the
// invariants is reported instead of trusted.
func rowToDomain(row gen.CodeHostSetting) (models.CodeHostSettings, error) {
	provider, err := models.ParseHostingProvider(row.Provider)
	if err != nil {
		return models.CodeHostSettings{}, err
	}
	return models.NewCodeHostSettings(provider, row.BaseUrl, row.Token, row.InsecureTls != 0)
}

// boolToInt stores flags the way the Rust schema does: INTEGER 0 or 1.
func boolToInt(value bool) int64 {
	if value {
		return 1
	}
	return 0
}
