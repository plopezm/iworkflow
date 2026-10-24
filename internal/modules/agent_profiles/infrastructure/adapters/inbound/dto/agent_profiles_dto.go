// Package dto holds the shapes that cross the boundary between the agent_profiles handlers and the
// clients. Mapping to and from the domain lives here, so no domain struct is serialized directly.
//
//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package dto

import (
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

// RoutingEntryDTO is the model and thinking level of one agent.
type RoutingEntryDTO struct {
	Model    *string `json:"model"`
	Thinking *string `json:"thinking"`
}

// ProfileDTO is one profile with its routing.
type ProfileDTO struct {
	Name    string                     `json:"name"`
	Routing map[string]RoutingEntryDTO `json:"routing"`
}

// ProfileStoreDTO is the profile store: the active profile name and the profiles.
type ProfileStoreDTO struct {
	Active   *string      `json:"active"`
	Profiles []ProfileDTO `json:"profiles"`
}

// AgentRoleDTO is an existing agent role.
type AgentRoleDTO struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Model       *string `json:"model"`
	Thinking    *string `json:"thinking"`
}

// NewAgentRoleDTO is the input to create an agent role.
type NewAgentRoleDTO struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Prompt      string  `json:"prompt"`
	Model       *string `json:"model"`
	Thinking    *string `json:"thinking"`
}

// ModelOptionDTO is a model that the routing editors can select.
type ModelOptionDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// RoutingFromDomain converts a routing map to its DTO form.
func RoutingFromDomain(routing map[string]models.RoutingEntry) map[string]RoutingEntryDTO {
	converted := make(map[string]RoutingEntryDTO, len(routing))
	for agent, entry := range routing {
		converted[agent] = RoutingEntryDTO{Model: entry.Model, Thinking: entry.Thinking}
	}
	return converted
}

// RoutingToDomain converts a DTO routing map back to the domain. Validation happens when the
// use case runs, not here.
func RoutingToDomain(routing map[string]RoutingEntryDTO) map[string]models.RoutingEntry {
	converted := make(map[string]models.RoutingEntry, len(routing))
	for agent, entry := range routing {
		converted[agent] = models.RoutingEntry{Model: entry.Model, Thinking: entry.Thinking}
	}
	return converted
}

// ProfileFromDomain converts a profile to its DTO form.
func ProfileFromDomain(profile models.Profile) ProfileDTO {
	return ProfileDTO{Name: profile.Name.String(), Routing: RoutingFromDomain(profile.Routing)}
}

// ToDomain converts the DTO to a profile. It fails when the name is not valid.
func (d ProfileDTO) ToDomain() (models.Profile, error) {
	name, err := models.ParseProfileName(d.Name)
	if err != nil {
		return models.Profile{}, err
	}
	return models.NewProfile(name, RoutingToDomain(d.Routing)), nil
}

// StoreFromDomain converts the profile store to its DTO form. The profile list is never nil.
func StoreFromDomain(store models.ProfileStore) ProfileStoreDTO {
	profiles := make([]ProfileDTO, 0, len(store.Profiles))
	for _, profile := range store.Profiles {
		profiles = append(profiles, ProfileFromDomain(profile))
	}
	var active *string
	if store.Active != nil {
		name := store.Active.String()
		active = &name
	}
	return ProfileStoreDTO{Active: active, Profiles: profiles}
}

// AgentRoleFromDomain converts a role to its DTO form.
func AgentRoleFromDomain(role models.AgentRole) AgentRoleDTO {
	return AgentRoleDTO{
		Name:        role.Name.String(),
		Description: role.Description,
		Model:       role.Model,
		Thinking:    role.Thinking,
	}
}

// ToDomain converts the input to a new role and validates it: name, model and thinking level.
func (d NewAgentRoleDTO) ToDomain() (models.NewAgentRole, error) {
	name, err := models.ParseRoleName(d.Name)
	if err != nil {
		return models.NewAgentRole{}, err
	}
	role := models.NewAgentRole{
		Name:        name,
		Description: d.Description,
		Prompt:      d.Prompt,
		Model:       d.Model,
		Thinking:    d.Thinking,
	}
	if err := role.Validate(); err != nil {
		return models.NewAgentRole{}, err
	}
	return role, nil
}

// ModelOptionFromDomain converts a model option to its DTO form.
func ModelOptionFromDomain(option outbound.ModelOption) ModelOptionDTO {
	return ModelOptionDTO{ID: option.ID, Label: option.Label}
}
