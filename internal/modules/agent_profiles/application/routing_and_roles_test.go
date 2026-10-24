package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
	"iworkflow/internal/modules/agent_profiles/domain/ports/outbound"
)

func TestListProfilesReturnsStoreAsLoaded(t *testing.T) {
	initial := storeOf(t, "a", profileOf(t, "a", "m/1"))
	uc := application.NewListProfiles(newFakeProfileStore(initial))

	listed, err := uc.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, initial, listed)
}

func TestListProfilesPropagatesLoadError(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	store.loadErr = errIO
	uc := application.NewListProfiles(store)

	_, err := uc.Execute(context.Background())

	require.Equal(t, errIO, err)
}

func TestGetGlobalRoutingReturnsLoadedRouting(t *testing.T) {
	uc := application.NewGetGlobalRouting(&fakeGlobalRouting{routing: routingOf("m/1")})

	routing, err := uc.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, routingOf("m/1"), routing)
}

func TestGetGlobalRoutingPropagatesError(t *testing.T) {
	uc := application.NewGetGlobalRouting(&fakeGlobalRouting{loadErr: errIO})

	_, err := uc.Execute(context.Background())

	require.Equal(t, errIO, err)
}

func TestSetGlobalRoutingSavesValidRouting(t *testing.T) {
	routing := &fakeGlobalRouting{}
	uc := application.NewSetGlobalRouting(routing)

	err := uc.Execute(context.Background(), routingOf("m/1"))

	require.NoError(t, err)
	require.Equal(t, 1, routing.saves)
	require.Equal(t, routingOf("m/1"), routing.routing)
}

func TestSetGlobalRoutingRejectsInvalidRoutingWithoutSaving(t *testing.T) {
	routing := &fakeGlobalRouting{}
	uc := application.NewSetGlobalRouting(routing)
	bad := map[string]models.RoutingEntry{" ": {}}

	err := uc.Execute(context.Background(), bad)

	require.Equal(t, agenterrors.InvalidRouting("clave de agente vacía"), err)
	require.Zero(t, routing.saves)
}

func TestListAgentRolesReturnsRoles(t *testing.T) {
	role := models.AgentRole{Name: roleName(t, "sdd-apply"), Description: "d"}
	uc := application.NewListAgentRoles(&fakeAgentRoles{roles: []models.AgentRole{role}})

	roles, err := uc.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, []models.AgentRole{role}, roles)
}

func TestListAgentRolesPropagatesError(t *testing.T) {
	uc := application.NewListAgentRoles(&fakeAgentRoles{listErr: errIO})

	_, err := uc.Execute(context.Background())

	require.Equal(t, errIO, err)
}

func TestCreateAgentRoleCreatesNewRole(t *testing.T) {
	roles := &fakeAgentRoles{roles: []models.AgentRole{{Name: roleName(t, "a")}}}
	uc := application.NewCreateAgentRole(roles)
	role := newAgentRole(t, "b")

	err := uc.Execute(context.Background(), role)

	require.NoError(t, err)
	require.Equal(t, []models.NewAgentRole{role}, roles.created)
}

func TestCreateAgentRoleRefusesExistingRoleWithoutCreating(t *testing.T) {
	roles := &fakeAgentRoles{roles: []models.AgentRole{{Name: roleName(t, "a")}}}
	uc := application.NewCreateAgentRole(roles)

	err := uc.Execute(context.Background(), newAgentRole(t, "a"))

	require.Equal(t, agenterrors.RoleAlreadyExists("a"), err)
	require.Empty(t, roles.created)
}

func TestListModelOptionsReturnsOptions(t *testing.T) {
	option := outbound.ModelOption{ID: "a/b", Label: "A B"}
	uc := application.NewListModelOptions(&fakeModelOptions{options: []outbound.ModelOption{option}})

	options, err := uc.Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, []outbound.ModelOption{option}, options)
}

func TestListModelOptionsPropagatesError(t *testing.T) {
	uc := application.NewListModelOptions(&fakeModelOptions{err: errIO})

	_, err := uc.Execute(context.Background())

	require.Equal(t, errIO, err)
}

func roleName(t *testing.T, raw string) models.RoleName {
	t.Helper()
	name, err := models.ParseRoleName(raw)
	require.NoError(t, err)
	return name
}

func newAgentRole(t *testing.T, raw string) models.NewAgentRole {
	t.Helper()
	return models.NewAgentRole{Name: roleName(t, raw), Description: "d", Prompt: "p"}
}
