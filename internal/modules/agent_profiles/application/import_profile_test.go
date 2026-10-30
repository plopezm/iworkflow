package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

func TestImportProfileImportsNewProfileKeepingActive(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1")))
	exchange := &fakeExchange{parsed: profileOf(t, "b", "m/2")}
	uc := application.NewImportProfile(store, exchange, application.NewWriteGuard())

	imported, err := uc.Execute(context.Background(), "raw", nil)

	require.NoError(t, err)
	require.Equal(t, profileOf(t, "b", "m/2"), imported)
	saved := store.snapshot()
	require.Equal(t, profileName(t, "a"), *saved.Active)
	require.Contains(t, profileNames(saved.Profiles), "b")
}

func TestImportProfileRefusesCollisionWithoutRename(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "", profileOf(t, "a", "m/1")))
	exchange := &fakeExchange{parsed: profileOf(t, "a", "m/2")}
	uc := application.NewImportProfile(store, exchange, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), "raw", nil)

	require.Equal(t, agenterrors.ProfileAlreadyExists("a"), err)
	require.Zero(t, store.savedCount())
}

func TestImportProfileRenameResolvesCollision(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "", profileOf(t, "a", "m/1")))
	exchange := &fakeExchange{parsed: profileOf(t, "a", "m/2")}
	uc := application.NewImportProfile(store, exchange, application.NewWriteGuard())
	renameTo := profileName(t, "c")

	imported, err := uc.Execute(context.Background(), "raw", &renameTo)

	require.NoError(t, err)
	require.Equal(t, profileOf(t, "c", "m/2"), imported)
	require.Contains(t, profileNames(store.snapshot().Profiles), "c")
}

func TestImportProfileRenameToExistingNameIsRefused(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "", profileOf(t, "a", "m/1")))
	exchange := &fakeExchange{parsed: profileOf(t, "b", "m/2")}
	uc := application.NewImportProfile(store, exchange, application.NewWriteGuard())
	renameTo := profileName(t, "a")

	_, err := uc.Execute(context.Background(), "raw", &renameTo)

	require.Equal(t, agenterrors.ProfileAlreadyExists("a"), err)
	require.Zero(t, store.savedCount())
}

func TestImportProfileParseErrorAbortsWithoutSaving(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	exchange := &fakeExchange{parseErr: agenterrors.InvalidImport("mal")}
	uc := application.NewImportProfile(store, exchange, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), "raw", nil)

	require.Equal(t, agenterrors.InvalidImport("mal"), err)
	require.Zero(t, store.loads)
	require.Zero(t, store.savedCount())
}
