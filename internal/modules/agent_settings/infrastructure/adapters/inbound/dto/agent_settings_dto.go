// Package dto holds the data that crosses the HTTP boundary of the agent settings module, and the
// mapping between those DTOs and the domain.
package dto

import (
	"fmt"

	agenterrors "iworkflow/internal/modules/agent_settings/domain/errors"
	"iworkflow/internal/modules/agent_settings/domain/models"
)

// AgentSettingsDTO is the settings payload. Empty identifiers mean "not set".
type AgentSettingsDTO struct {
	Runtime         *string `json:"runtime"`
	ModelProvider   *string `json:"model_provider"`
	ModelID         *string `json:"model_id"`
	Profile         *string `json:"profile"`
	Thinking        *string `json:"thinking"`
	MemoryLimitMB   *uint32 `json:"memory_limit_mb"`
	CPULimitPercent *uint32 `json:"cpu_limit_percent"`
	LimitsDisabled  bool    `json:"limits_disabled"`
}

// ModelOptionDTO is one orchestrator model offered to the selector.
type ModelOptionDTO struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

// ProfileCatalogDTO is the profile catalog offered to the selector.
type ProfileCatalogDTO struct {
	Active *string  `json:"active"`
	Names  []string `json:"names"`
}

// FromAgentSettings maps the domain settings to their payload.
func FromAgentSettings(settings models.AgentSettings) AgentSettingsDTO {
	dto := AgentSettingsDTO{
		MemoryLimitMB:   settings.MemoryLimitMB,
		CPULimitPercent: settings.CPULimitPercent,
		LimitsDisabled:  settings.LimitsDisabled,
	}
	if settings.Runtime != nil {
		runtime := string(*settings.Runtime)
		dto.Runtime = &runtime
	}
	if settings.Model != nil {
		provider := settings.Model.Provider
		model := settings.Model.Model
		dto.ModelProvider = &provider
		dto.ModelID = &model
	}
	if settings.Profile != nil {
		profile := settings.Profile.String()
		dto.Profile = &profile
	}
	if settings.Thinking != nil {
		thinking := string(*settings.Thinking)
		dto.Thinking = &thinking
	}
	return dto
}

// ToAgentSettings validates the payload and maps it to the domain. The order of the checks is
// part of the contract: profile, thinking, runtime, memory limit, CPU limit.
func (d AgentSettingsDTO) ToAgentSettings() (models.AgentSettings, error) {
	var settings models.AgentSettings
	if d.ModelProvider != nil && d.ModelID != nil && *d.ModelProvider != "" && *d.ModelID != "" {
		settings.Model = &models.ModelRef{Provider: *d.ModelProvider, Model: *d.ModelID}
	}
	if profile := nonEmpty(d.Profile); profile != nil {
		name, err := models.ParseProfileName(*profile)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // domain error; passed unchanged
		}
		settings.Profile = &name
	}
	if thinking := nonEmpty(d.Thinking); thinking != nil {
		level, err := models.ParseThinkingLevel(*thinking)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // domain error; passed unchanged
		}
		settings.Thinking = &level
	}
	if runtime := nonEmpty(d.Runtime); runtime != nil {
		parsed, err := models.ParseAgentRuntime(*runtime)
		if err != nil {
			return models.AgentSettings{}, err //nolint:wrapcheck // domain error; passed unchanged
		}
		settings.Runtime = &parsed
	}
	memory, err := positiveLimit(d.MemoryLimitMB, "memoria")
	if err != nil {
		return models.AgentSettings{}, err
	}
	cpu, err := positiveLimit(d.CPULimitPercent, "CPU")
	if err != nil {
		return models.AgentSettings{}, err
	}
	settings.MemoryLimitMB = memory
	settings.CPULimitPercent = cpu
	settings.LimitsDisabled = d.LimitsDisabled
	return settings, nil
}

// FromModelRef maps a model to the selector option.
func FromModelRef(model models.ModelRef) ModelOptionDTO {
	return ModelOptionDTO{Provider: model.Provider, Model: model.Model}
}

// FromProfileCatalog maps the profile catalog to its payload; names is never null.
func FromProfileCatalog(catalog models.ProfileCatalog) ProfileCatalogDTO {
	names := make([]string, 0, len(catalog.Names))
	names = append(names, catalog.Names...)
	return ProfileCatalogDTO{Active: catalog.Active, Names: names}
}

// ThinkingLevelIDs returns the identifiers of the levels that the runtime accepts. A nil or empty
// runtime means the default one. The domain is the only source of truth; the UI only labels them.
func ThinkingLevelIDs(runtime *string) ([]string, error) {
	selected := models.RuntimeGentleShell
	if raw := nonEmpty(runtime); raw != nil {
		parsed, err := models.ParseAgentRuntime(*raw)
		if err != nil {
			return nil, err //nolint:wrapcheck // domain error; passed unchanged
		}
		selected = parsed
	}
	levels := models.SupportedBy(selected)
	ids := make([]string, 0, len(levels))
	for _, level := range levels {
		ids = append(ids, string(level))
	}
	return ids, nil
}

// RuntimeOptions returns the identifiers of every agent runtime, in selector order.
func RuntimeOptions() []string {
	runtimes := models.AllRuntimes()
	ids := make([]string, 0, len(runtimes))
	for _, runtime := range runtimes {
		ids = append(ids, string(runtime))
	}
	return ids
}

// positiveLimit rejects a defined limit of zero: a limit of 0 means nothing for cgroups.
func positiveLimit(value *uint32, name string) (*uint32, error) {
	if value != nil && *value == 0 {
		return nil, agenterrors.InvalidLimit(fmt.Sprintf("el límite de %s debe ser mayor que 0", name)) //nolint:wrapcheck // domain constructor returns a domain error
	}
	return value, nil
}

// nonEmpty returns the value unless it is nil or the empty string.
func nonEmpty(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}
