package application_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

func TestDuplicateProfileCopiesRoutingAndKeepsActive(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1")))
	uc := application.NewDuplicateProfile(store, application.NewWriteGuard())

	duplicated, err := uc.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))

	require.NoError(t, err)
	require.Equal(t, profileOf(t, "b", "m/1"), duplicated)
	saved := store.snapshot()
	require.Equal(t, profileName(t, "a"), *saved.Active)
	require.Equal(t, routingOf("m/1"), saved.Find(profileName(t, "b")).Routing)
}

func TestDuplicateProfileFailsWhenSourceMissing(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	uc := application.NewDuplicateProfile(store, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))

	require.Equal(t, agenterrors.ProfileNotFound("a"), err)
	require.Zero(t, store.savedCount())
}

func TestDuplicateProfileFailsWhenTargetExists(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "", profileOf(t, "a", "m/1"), profileOf(t, "b", "m/2")))
	uc := application.NewDuplicateProfile(store, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))

	require.Equal(t, agenterrors.ProfileAlreadyExists("b"), err)
	require.Zero(t, store.savedCount())
}

func TestDuplicateProfileSharedGuardSerializesDuplicateAndCreate(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1")))
	store.yield = true
	guard := application.NewWriteGuard()
	duplicate := application.NewDuplicateProfile(store, guard)
	create := application.NewCreateProfile(store, guard)
	var group sync.WaitGroup
	var duplicateErr, createErr error
	group.Add(2)
	go func() {
		defer group.Done()
		_, duplicateErr = duplicate.Execute(context.Background(), profileName(t, "a"), profileName(t, "b"))
	}()
	go func() {
		defer group.Done()
		_, createErr = create.Execute(context.Background(), profileName(t, "c"))
	}()
	group.Wait()

	require.NoError(t, duplicateErr)
	require.NoError(t, createErr)
	require.Len(t, store.snapshot().Profiles, 3)
}
