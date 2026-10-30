package application_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
	"iworkflow/internal/modules/agent_profiles/domain/models"
)

func TestCreateProfileCreatesEmptyProfileAndKeepsActive(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "a", profileOf(t, "a", "m/1")))
	uc := application.NewCreateProfile(store, application.NewWriteGuard())

	created, err := uc.Execute(context.Background(), profileName(t, "b"))

	require.NoError(t, err)
	require.Equal(t, models.NewProfile(profileName(t, "b"), nil), created)
	saved := store.snapshot()
	require.Equal(t, profileName(t, "a"), *saved.Active)
	require.Len(t, saved.Profiles, 2)
	require.Empty(t, saved.Find(profileName(t, "b")).Routing)
}

func TestCreateProfileRefusesExistingProfileWithoutSaving(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "", profileOf(t, "a", "m/1")))
	uc := application.NewCreateProfile(store, application.NewWriteGuard())

	_, err := uc.Execute(context.Background(), profileName(t, "a"))

	require.Equal(t, agenterrors.ProfileAlreadyExists("a"), err)
	require.Zero(t, store.savedCount())
}

func TestCreateProfileConcurrentCreatesBothPersist(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	store.yield = true
	uc := application.NewCreateProfile(store, application.NewWriteGuard())
	var group sync.WaitGroup
	var errs [2]error
	for i, raw := range []string{"a", "b"} {
		group.Add(1)
		go func() {
			defer group.Done()
			_, errs[i] = uc.Execute(context.Background(), profileName(t, raw))
		}()
	}
	group.Wait()

	require.NoError(t, errs[0])
	require.NoError(t, errs[1])
	require.Len(t, store.snapshot().Profiles, 2)
}

func TestCreateProfileStopsWaitingForLockWhenContextEnds(t *testing.T) {
	inner := newFakeProfileStore(storeOf(t, ""))
	gate := &gatedStore{fakeProfileStore: inner, entered: make(chan struct{}, 1), release: make(chan struct{})}
	guard := application.NewWriteGuard()
	first := application.NewCreateProfile(gate, guard)
	second := application.NewCreateProfile(inner, guard)
	done := make(chan error, 1)
	go func() {
		_, err := first.Execute(context.Background(), profileName(t, "a"))
		done <- err
	}()
	<-gate.entered
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := second.Execute(cancelled, profileName(t, "b"))

	require.ErrorIs(t, err, context.Canceled)
	close(gate.release)
	require.NoError(t, <-done)
	require.Len(t, inner.snapshot().Profiles, 1)
}

// gatedStore holds Load until release is closed, so a test can keep the write guard busy.
type gatedStore struct {
	*fakeProfileStore
	entered chan struct{}
	release chan struct{}
}

func (g *gatedStore) Load(ctx context.Context) (models.ProfileStore, error) {
	g.entered <- struct{}{}
	<-g.release
	return g.fakeProfileStore.Load(ctx)
}
