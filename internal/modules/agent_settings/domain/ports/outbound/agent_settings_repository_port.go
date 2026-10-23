// Package outbound holds the ports the agent settings domain needs from the outside world.
package outbound

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

// AgentSettingsRepositoryPort stores the single row of global settings.
type AgentSettingsRepositoryPort interface {
	// Get returns the stored settings; a missing row yields the zero AgentSettings.
	Get(ctx context.Context) (models.AgentSettings, error)
	// Save upserts the single row of settings.
	Save(ctx context.Context, settings models.AgentSettings) error
}
