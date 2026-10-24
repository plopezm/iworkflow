package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func TestSetProfileRoutingReplacesRoutingAndKeepsActive(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1")))
	uc := application.NewSetProfileRouting(store, application.NewWriteGuard())

	updated, err := uc.Execute(context.Background(), profileName(t, "a"), routingOf("m/2"))

	require.NoError(t, err)
	require.Equal(t, profileOf(t, "a", "m/2"), updated)
	saved := store.snapshot()
	require.Equal(t, profileName(t, "a"), *saved.Active)
	require.Equal(t, routingOf("m/2"), saved.Find(profileName(t, "a")).Routing)
}

func TestSetProfileRoutingRejectsInvalidRoutingWithoutSaving(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "", profileOf(t, "a", "m/1")))
	uc := application.NewSetProfileRouting(store, application.NewWriteGuard())
	bad := map[string]models.RoutingEntry{"x": {Thinking: ptr("ultra")}}

	_, err := uc.Execute(context.Background(), profileName(t, "a"), bad)

	require.ErrorIs(t, err, agenterrors.InvalidRouting(""))
	require.Zero(t, store.savedCount())
}

func TestSetProfileRoutingFailsWhenProfileMissing(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	uc := application.NewSetProfileRouting(store, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), profileName(t, "a"), routingOf("m/2"))

	require.Equal(t, agenterrors.ProfileNotFound("a"), err)
	require.Zero(t, store.savedCount())
}
