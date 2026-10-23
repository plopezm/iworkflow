package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
)

var errRandom = errors.New("no entropy")

func failingGenerator() (models.AccessToken, error) {
	return models.AccessToken{}, errRandom
}

func TestGetAccessTokenReturnsTheStoredTokenWithoutWriting(t *testing.T) {
	stored, err := models.GenerateAccessToken()
	require.NoError(t, err)
	repository := newFakeRepository()
	repository.accessToken = &stored

	token, err := application.NewGetAccessToken(repository, models.GenerateAccessToken).Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, stored, token)
	require.Equal(t, 0, repository.writes)
}

func TestGetAccessTokenGeneratesAndStoresATokenTheFirstTime(t *testing.T) {
	repository := newFakeRepository()

	token, err := application.NewGetAccessToken(repository, models.GenerateAccessToken).Execute(context.Background())

	require.NoError(t, err)
	_, err = models.ParseAccessToken(token.Value())
	require.NoError(t, err)
	require.NotNil(t, repository.accessToken)
	require.Equal(t, token, *repository.accessToken)
	require.Equal(t, 1, repository.writes)
}

func TestGetAccessTokenDoesNotHandOutATokenThatFailedToPersist(t *testing.T) {
	repository := newFakeRepository()
	repository.writeErr = errStorage

	token, err := application.NewGetAccessToken(repository, models.GenerateAccessToken).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
	require.Equal(t, models.AccessToken{}, token)
}

func TestGetAccessTokenDoesNotGenerateWhenTheReadFails(t *testing.T) {
	repository := newFakeRepository()
	repository.readErr = errStorage

	_, err := application.NewGetAccessToken(repository, models.GenerateAccessToken).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
	require.Equal(t, 0, repository.writes)
}

func TestGetAccessTokenFailsWhenNoTokenCanBeGenerated(t *testing.T) {
	repository := newFakeRepository()

	_, err := application.NewGetAccessToken(repository, failingGenerator).Execute(context.Background())

	require.ErrorIs(t, err, errRandom)
	require.Equal(t, 0, repository.writes)
}
