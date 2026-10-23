package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/preferences/application"
	domainerrors "iworkflow/internal/modules/preferences/domain/errors"
)

func TestSaveListenPortStoresAValidPort(t *testing.T) {
	repository := newFakeRepository()

	saved, err := application.NewSaveListenPort(repository).Execute(context.Background(), 9100)

	require.NoError(t, err)
	require.Equal(t, uint16(9100), saved.Port())
	require.NotNil(t, repository.listenPort)
	require.Equal(t, uint16(9100), repository.listenPort.Port())
}

func TestSaveListenPortRejectsPrivilegedPortsWithoutTouchingStorage(t *testing.T) {
	repository := newFakeRepository()

	_, err := application.NewSaveListenPort(repository).Execute(context.Background(), 80)

	var invalid *domainerrors.InvalidPortError
	require.ErrorAs(t, err, &invalid)
	require.Equal(t, uint16(80), invalid.Port)
	require.Equal(t, 0, repository.writes)
}

func TestSaveListenPortPropagatesStorageErrors(t *testing.T) {
	repository := newFakeRepository()
	repository.writeErr = errStorage

	_, err := application.NewSaveListenPort(repository).Execute(context.Background(), 9100)

	require.ErrorIs(t, err, errStorage)
}
