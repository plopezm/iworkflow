package application

import (
	"context"
	"slices"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
	"iworkflow/internal/modules/agent_settings/domain/ports/outbound"
)

// UpdateAgentSettings saves the global settings, rejecting models and profiles that do not exist.
type UpdateAgentSettings struct {
	repository     outbound.AgentSettingsRepositoryPort
	modelCatalog   outbound.ModelCatalogPort
	profileCatalog outbound.ProfileCatalogPort
}

// NewUpdateAgentSettings builds the use case.
func NewUpdateAgentSettings(
	repository outbound.AgentSettingsRepositoryPort,
	modelCatalog outbound.ModelCatalogPort,
	profileCatalog outbound.ProfileCatalogPort,
) *UpdateAgentSettings {
	return &UpdateAgentSettings{
		repository:     repository,
		modelCatalog:   modelCatalog,
		profileCatalog: profileCatalog,
	}
}

// Execute validates the settings against the catalogs, saves them and returns what was stored.
func (u *UpdateAgentSettings) Execute(ctx context.Context, settings models.AgentSettings) (models.AgentSettings, error) {
	runtime := models.RuntimeOrDefault(settings.Runtime)
	settings.Thinking = effectiveThinking(settings.Thinking, runtime)
	if settings.Model != nil {
		available, err := u.modelCatalog.List(ctx, runtime)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
		}
		if !slices.Contains(available, *settings.Model) {
			return models.AgentSettings{}, agenterrors.ModelNotFound(settings.Model.Provider, settings.Model.Model) //nolint:wrapcheck // domain constructor returns a domain error
		}
	}
	if settings.Profile != nil {
		catalog, err := u.profileCatalog.List(ctx)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
		}
		if !slices.Contains(catalog.Names, settings.Profile.String()) {
			return models.AgentSettings{}, agenterrors.ProfileNotFound(settings.Profile.String()) //nolint:wrapcheck // domain constructor returns a domain error
		}
	}
	if err := u.repository.Save(ctx, settings); err != nil {
		return models.AgentSettings{}, err //nolint:wrapcheck // port errors are domain errors; passed unchanged
	}
	return settings, nil
}
