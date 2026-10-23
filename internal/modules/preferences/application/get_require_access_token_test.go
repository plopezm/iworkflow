package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
)

func TestGetRequireAccessTokenReturnsTheStoredChoice(t *testing.T) {
	repository := newFakeRepository()
	repository.requireToken = false

	required, err := application.NewGetRequireAccessToken(repository).Execute(context.Background())

	require.NoError(t, err)
	require.False(t, required)
}

func TestGetRequireAccessTokenIsTrueUnlessSwitchedOff(t *testing.T) {
	// The fake starts with the token required, like a fresh install.
	required, err := application.NewGetRequireAccessToken(newFakeRepository()).Execute(context.Background())

	require.NoError(t, err)
	require.True(t, required)
}

func TestGetRequireAccessTokenPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.readErr = errStorage

	_, err := application.NewGetRequireAccessToken(repository).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
}
