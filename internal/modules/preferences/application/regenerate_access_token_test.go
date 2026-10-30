package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
)

func TestRegenerateAccessTokenStoresAndReturnsANewToken(t *testing.T) {
	old, err := models.GenerateAccessToken()
	require.NoError(t, err)
	repository := newFakeRepository()
	repository.accessToken = &old

	token, err := application.NewRegenerateAccessToken(repository, models.GenerateAccessToken).Execute(context.Background())

	require.NoError(t, err)
	_, err = models.ParseAccessToken(token.Value())
	require.NoError(t, err)
	require.NotEqual(t, old, token)
	require.NotNil(t, repository.accessToken)
	require.Equal(t, token, *repository.accessToken)
	require.Equal(t, 1, repository.writes)
}

func TestRegenerateAccessTokenPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.writeErr = errStorage

	_, err := application.NewRegenerateAccessToken(repository, models.GenerateAccessToken).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
}

func TestRegenerateAccessTokenFailsWhenNoTokenCanBeGenerated(t *testing.T) {
	repository := newFakeRepository()

	_, err := application.NewRegenerateAccessToken(repository, failingGenerator).Execute(context.Background())

	require.ErrorIs(t, err, errRandom)
	require.Equal(t, 0, repository.writes)
}
