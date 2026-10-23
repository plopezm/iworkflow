package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
)

func TestSaveNetworkAccessStoresTheChoice(t *testing.T) {
	repository := newFakeRepository()

	saved, err := application.NewSaveNetworkAccess(repository).Execute(context.Background(), true)

	require.NoError(t, err)
	require.True(t, saved)
	require.True(t, repository.networkAccess)
	require.Equal(t, 1, repository.writes)
}

func TestSaveNetworkAccessPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.writeErr = errStorage

	_, err := application.NewSaveNetworkAccess(repository).Execute(context.Background(), false)

	require.ErrorIs(t, err, errStorage)
}
