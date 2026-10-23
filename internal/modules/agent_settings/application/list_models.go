package application

import (
	"context"

	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

// ListModels lists the orchestrator models of the runtime saved in the settings
// (`gentle-shell --list-models` or `pi --list-models`).
type ListModels struct {
	repository outbound.AgentSettingsRepositoryPort
	catalog    outbound.ModelCatalogPort
}

// NewListModels builds the use case.
func NewListModels(repository outbound.AgentSettingsRepositoryPort, catalog outbound.ModelCatalogPort) *ListModels {
	return &ListModels{repository: repository, catalog: catalog}
}

// Execute returns the models of the selected runtime (gentle-shell when none is saved).
func (u *ListModels) Execute(ctx context.Context) ([]models.ModelRef, error) {
	settings, err := u.repository.Get(ctx)
	if err != nil {
		return nil, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	available, err := u.catalog.List(ctx, models.RuntimeOrDefault(settings.Runtime))
	return available, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
}
