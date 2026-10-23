package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	"iworkflow/internal/modules/preferences/domain/models"
)

func TestGetListenPortReturnsTheStoredPort(t *testing.T) {
	repository := newFakeRepository()
	port, err := models.NewListenPort(9100)
	require.NoError(t, err)
	repository.listenPort = &port

	stored, err := application.NewGetListenPort(repository).Execute(context.Background())

	require.NoError(t, err)
	require.NotNil(t, stored)
	require.Equal(t, uint16(9100), stored.Port())
}

func TestGetListenPortReturnsNilBeforeAnyChoice(t *testing.T) {
	stored, err := application.NewGetListenPort(newFakeRepository()).Execute(context.Background())

	require.NoError(t, err)
	require.Nil(t, stored)
}

func TestGetListenPortPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.readErr = errStorage

	_, err := application.NewGetListenPort(repository).Execute(context.Background())

	require.ErrorIs(t, err, errStorage)
}
