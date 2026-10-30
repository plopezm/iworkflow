package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_profiles/application"
	agenterrors "iworkflow/internal/modules/agent_profiles/domain/errors"
)

func TestExportProfileSerializesExistingProfile(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, "", profileOf(t, "a", "m/1")))
	exchange := &fakeExchange{}
	uc := application.NewExportProfile(store, exchange)

	exported, err := uc.Execute(context.Background(), profileName(t, "a"))

	require.NoError(t, err)
	require.Equal(t, "a", exported)
	require.Equal(t, 1, exchange.serializes)
}

func TestExportProfileFailsWhenMissingWithoutSerializing(t *testing.T) {
	store := newFakeProfileStore(storeOf(t, ""))
	exchange := &fakeExchange{}
	uc := application.NewExportProfile(store, exchange)

	_, err := uc.Execute(context.Background(), profileName(t, "a"))

	require.Equal(t, agenterrors.ProfileNotFound("a"), err)
	require.Zero(t, exchange.serializes)
}
