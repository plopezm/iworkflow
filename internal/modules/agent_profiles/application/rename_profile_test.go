package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

func TestRenameProfileRenamesAndMovesActiveMarker(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1"), profileOf(t, "c", "m/3")))
	uc := application.NewRenameProfile(store, application.NewWriteGuard())

	renamed, err := uc.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))

	require.NoError(t, err)
	require.Equal(t, profileOf(t, "b", "m/1"), renamed)
	saved := store.snapshot()
	require.Equal(t, profileName(t, "b"), *saved.Active)
	require.Equal(t, []string{"b", "c"}, profileNames(saved.Profiles))
}

func TestRenameProfileKeepsActiveWhenRenamingOtherProfile(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "c", profileOf(t, "a", "m/1"), profileOf(t, "c", "m/3")))
	uc := application.NewRenameProfile(store, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))

	require.NoError(t, err)
	require.Equal(t, profileName(t, "c"), *store.snapshot().Active)
}

func TestRenameProfileFailsWhenSourceMissing(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	uc := application.NewRenameProfile(store, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))

	require.Equal(t, agenterrors.ProfileNotFound("a"), err)
	require.Zero(t, store.savedCount())
}

func TestRenameProfileRefusesExistingTarget(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1"), profileOf(t, "b", "m/2")))
	uc := application.NewRenameProfile(store, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))

	require.Equal(t, agenterrors.ProfileAlreadyExists("b"), err)
	require.Zero(t, store.savedCount())
}
