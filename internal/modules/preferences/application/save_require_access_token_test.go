package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
)

func TestSaveRequireAccessTokenStoresTheChoice(t *testing.T) {
	repository := newFakeRepository()

	saved, err := application.NewSaveRequireAccessToken(repository).Execute(context.Background(), false)

	require.NoError(t, err)
	require.False(t, saved)
	require.False(t, repository.requireToken)
	require.Equal(t, 1, repository.writes)
}

func TestSaveRequireAccessTokenPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.writeErr = errStorage

	_, err := application.NewSaveRequireAccessToken(repository).Execute(context.Background(), true)

	require.ErrorIs(t, err, errStorage)
}
