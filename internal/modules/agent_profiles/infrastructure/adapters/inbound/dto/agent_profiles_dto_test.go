package dto

import (
	"testing"

	"github.com/stretchr/testify/require"

	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

func stringPtr(value string) *string {
	return &value
}

func sampleRouting() map[string]models.RoutingEntry {
	return map[string]models.RoutingEntry{
		models.OrchestratorKey: {Model: stringPtr("anthropic/claude"), Thinking: stringPtr("high")},
		"sdd-apply":            {},
	}
}

func mustProfileName(t *testing.T, raw string) models.ProfileName {
	t.Helper()
	name, err := models.ParseProfileName(raw)
	require.NoError(t, err)
	return name
}

func TestProfileRoundTripsThroughDTO(t *testing.T) {
	profile := models.NewProfile(mustProfileName(t, "fast"), sampleRouting())

	back, err := ProfileFromDomain(profile).ToDomain()

	require.NoError(t, err)
	require.Equal(t, profile, back)
}

func TestProfileDTOWithInvalidNameIsRejected(t *testing.T) {
	_, err := ProfileDTO{Name: "bad name"}.ToDomain()

	require.Equal(t, agenterrors.InvalidProfileName("bad name"), err)
}

func TestStoreMapsActiveAndProfiles(t *testing.T) {
	active := mustProfileName(t, "fast")
	store := models.NewProfileStore(&active, []models.Profile{models.NewProfile(active, sampleRouting())})

	converted := StoreFromDomain(store)

	require.NotNil(t, converted.Active)
	require.Equal(t, "fast", *converted.Active)
	require.Len(t, converted.Profiles, 1)
	require.Equal(t, "fast", converted.Profiles[0].Name)
	require.Equal(t, RoutingEntryDTO{Model: stringPtr("anthropic/claude"), Thinking: stringPtr("high")},
		converted.Profiles[0].Routing[models.OrchestratorKey])
}

func TestEmptyStoreMapsToEmptyDTO(t *testing.T) {
	converted := StoreFromDomain(models.NewProfileStore(nil, nil))

	require.Nil(t, converted.Active)
	require.Empty(t, converted.Profiles)
}

func TestRoutingRoundTripsThroughDTO(t *testing.T) {
	original := sampleRouting()

	back := RoutingToDomain(RoutingFromDomain(original))

	require.Equal(t, original, back)
}

func TestAgentRoleMapsToDTO(t *testing.T) {
	name, err := models.ParseRoleName("reviewer")
	require.NoError(t, err)
	role := models.AgentRole{Name: name, Description: "Revisa", Model: stringPtr("p/m")}

	converted := AgentRoleFromDomain(role)

	require.Equal(t, AgentRoleDTO{Name: "reviewer", Description: "Revisa", Model: stringPtr("p/m")}, converted)
}

func TestNewRoleDTOMapsToDomain(t *testing.T) {
	role, err := NewAgentRoleDTO{Name: "reviewer", Description: "Revisa", Prompt: "Eres un revisor"}.ToDomain()

	require.NoError(t, err)
	require.Equal(t, "reviewer", role.Name.String())
	require.Equal(t, "Revisa", role.Description)
	require.Equal(t, "Eres un revisor", role.Prompt)
}

func TestNewRoleDTOMapsModelAndThinking(t *testing.T) {
	role, err := NewAgentRoleDTO{Name: "reviewer", Model: stringPtr("p/m"), Thinking: stringPtr("high")}.ToDomain()

	require.NoError(t, err)
	require.Equal(t, "p/m", *role.Model)
	require.Equal(t, "high", *role.Thinking)
}

func TestNewRoleDTORejectsInvalidThinkingBlankModelAndNewlines(t *testing.T) {
	build := func(model *string, thinking *string) NewAgentRoleDTO {
		return NewAgentRoleDTO{Name: "reviewer", Model: model, Thinking: thinking}
	}

	_, err := build(nil, stringPtr("ultra")).ToDomain()
	require.ErrorIs(t, err, agenterrors.InvalidRouting(""))
	_, err = build(stringPtr("  "), nil).ToDomain()
	require.Error(t, err)
	_, err = build(stringPtr("a\nb"), nil).ToDomain()
	require.Error(t, err)
}

func TestNewRoleDTOWithInvalidNameIsRejected(t *testing.T) {
	_, err := NewAgentRoleDTO{Name: "a/b"}.ToDomain()

	require.Equal(t, agenterrors.InvalidRoleName("a/b"), err)
}

func TestModelOptionMapsToDTO(t *testing.T) {
	converted := ModelOptionFromDomain(outbound.ModelOption{ID: "p/m", Label: "p/m"})

	require.Equal(t, ModelOptionDTO{ID: "p/m", Label: "p/m"}, converted)
}
