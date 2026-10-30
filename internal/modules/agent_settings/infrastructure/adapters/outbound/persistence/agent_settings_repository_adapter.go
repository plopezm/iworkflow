// Package persistence implements the agent settings outbound ports on SQLite. Only this package
// talks to the database; the queries live in queries/*.sql and the typed code in gen/ (sqlc).
package persistence

import (
	"context"
	"database/sql"
	"errors"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
	"iworkflow/internal/modules/agent_settings/infrastructure/adapters/outbound/persistence/gen"
)

// settingsRowID is the id of the only row of agent_settings (enforced by a CHECK in the schema).
const settingsRowID int64 = 1

// AgentSettingsRepositoryAdapter reads and writes the global agent settings row.
type AgentSettingsRepositoryAdapter struct {
	queries *gen.Queries
}

var _ outbound.AgentSettingsRepositoryPort = (*AgentSettingsRepositoryAdapter)(nil)

// NewAgentSettingsRepositoryAdapter builds the adapter over an open database.
func NewAgentSettingsRepositoryAdapter(db *sql.DB) *AgentSettingsRepositoryAdapter {
	return &AgentSettingsRepositoryAdapter{queries: gen.New(db)}
}

// Get returns the stored settings; a missing row yields the zero AgentSettings.
func (a *AgentSettingsRepositoryAdapter) Get(ctx context.Context) (models.AgentSettings, error) {
	row, err := a.queries.FindAgentSettings(ctx, settingsRowID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.AgentSettings{}, nil
	}
	if err != nil {
		return models.AgentSettings{}, agenterrors.Repository(err) //nolint:wrapcheck // domain constructor returns a domain error
	}
	return settingsFromRow(row)
}

// Save upserts the single row of settings.
func (a *AgentSettingsRepositoryAdapter) Save(ctx context.Context, settings models.AgentSettings) error {
	if err := a.queries.ReplaceAgentSettings(ctx, rowFromSettings(settingsRowID, settings)); err != nil {
		return agenterrors.Repository(err) //nolint:wrapcheck // domain constructor returns a domain error
	}
	return nil
}
