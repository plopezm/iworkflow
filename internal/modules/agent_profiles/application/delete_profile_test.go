package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

func TestDeleteProfileDeletesInactiveProfile(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1"), profileOf(t, "b", "m/2")))
	uc := application.NewDeleteProfile(store, application.NewWriteGuard())

	err := uc.Execute(context.Background(), profileName(t, "b"))

	require.NoError(t, err)
	saved := store.snapshot()
	require.Equal(t, profileName(t, "a"), *saved.Active)
	require.Equal(t, []string{"a"}, profileNames(saved.Profiles))
}

func TestDeleteProfileRefusesActiveProfile(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1")))
	uc := application.NewDeleteProfile(store, application.NewWriteGuard())

	err := uc.Execute(context.Background(), profileName(t, "a"))

	require.Equal(t, agenterrors.CannotDeleteActiveProfile("a"), err)
	require.Zero(t, store.savedCount())
}

func TestDeleteProfileFailsWhenMissing(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	uc := application.NewDeleteProfile(store, application.NewWriteGuard())

	err := uc.Execute(context.Background(), profileName(t, "z"))

	require.Equal(t, agenterrors.ProfileNotFound("z"), err)
	require.Zero(t, store.savedCount())
}
