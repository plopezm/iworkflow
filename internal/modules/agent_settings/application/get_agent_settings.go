// Package application holds the agent settings use cases. Each one is a struct with Execute.
package application

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

// GetAgentSettings returns the current settings (the zero value when they were never saved).
type GetAgentSettings struct {
	repository outbound.AgentSettingsRepositoryPort
}

// NewGetAgentSettings builds the use case.
func NewGetAgentSettings(repository outbound.AgentSettingsRepositoryPort) *GetAgentSettings {
	return &GetAgentSettings{repository: repository}
}

// Execute reads the settings.
func (u *GetAgentSettings) Execute(ctx context.Context) (models.AgentSettings, error) {
	settings, err := u.repository.Get(ctx) //nolint:wrapcheck // port errors are domain errors; passed unchanged
	return settings, err                   //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
