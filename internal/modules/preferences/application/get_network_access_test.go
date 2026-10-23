package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
)

func TestGetNetworkAccessReturnsTheStoredChoice(t *testing.T) {
	repository := newFakeRepository()
	repository.networkAccess = true

	enabled, err := application.NewGetNetworkAccess(repository).Execute(context.Background())

	require.NoError(t, err)
	require.True(t, enabled)
}

func TestGetNetworkAccessDefaultsToPrivate(t *testing.T) {
	enabled, err := application.NewGetNetworkAccess(newFakeRepository()).Execute(context.Background())

	require.NoError(t, err)
	require.False(t, enabled)
}

func TestGetNetworkAccessPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.readErr = errStorage

	_, err := application.NewGetNetworkAccess(repository).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
}
