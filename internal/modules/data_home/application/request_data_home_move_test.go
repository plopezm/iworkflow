package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/data_home/application"
	domainerrors "iworkflow/internal/modules/data_home/domain/errors"
	"iworkflow/internal/modules/data_home/domain/models"
)

const destination = "/mnt/disk/iwf"

func TestRequestDataHomeMoveRefusesWhenIWFHomeFixesTheFolder(t *testing.T) {
	mover := &fakeMover{status: models.HomeStatus{Current: "/env/iwf", Source: models.SourceEnv, EnvLocked: true}}
	tasks := &fakeTasks{}
	restarter := &fakeRestarter{}

	err := application.NewRequestDataHomeMove(mover, tasks, restarter).Execute(context.Background(), application.MoveInput{Destination: destination})

	require.True(t, errors.Is(err, domainerrors.ErrEnvLocked))
	require.Empty(t, mover.requested, "a locked folder must not record a move")
	require.Zero(t, restarter.requests, "a refused move must not restart the server")
}

func TestRequestDataHomeMoveRefusesWhileATaskIsActive(t *testing.T) {
	mover := &fakeMover{status: models.HomeStatus{Current: "/data/iwf", Source: models.SourceDefault}}
	tasks := &fakeTasks{active: true}
	restarter := &fakeRestarter{}

	err := application.NewRequestDataHomeMove(mover, tasks, restarter).Execute(context.Background(), application.MoveInput{Destination: destination})

	require.True(t, errors.Is(err, domainerrors.ErrActiveTasks))
	require.Empty(t, mover.requested)
	require.Zero(t, restarter.requests)
}

func TestRequestDataHomeMoveKeepsTheReasonOfARefusedDestination(t *testing.T) {
	mover := &fakeMover{
		status:     models.HomeStatus{Current: "/data/iwf", Source: models.SourceDefault},
		requestErr: &domainerrors.DestinationError{Reason: domainerrors.ReasonNotEmpty},
	}
	restarter := &fakeRestarter{}

	err := application.NewRequestDataHomeMove(mover, &fakeTasks{}, restarter).Execute(context.Background(), application.MoveInput{Destination: destination})

	var destinationErr *domainerrors.DestinationError
	require.True(t, errors.As(err, &destinationErr))
	require.Equal(t, domainerrors.ReasonNotEmpty, destinationErr.Reason)
	require.Zero(t, restarter.requests, "a refused destination must not restart the server")
}

func TestRequestDataHomeMoveRecordsThenRestarts(t *testing.T) {
	mover := &fakeMover{status: models.HomeStatus{Current: "/data/iwf", Source: models.SourceDefault}}
	restarter := &fakeRestarter{}

	err := application.NewRequestDataHomeMove(mover, &fakeTasks{}, restarter).Execute(context.Background(), application.MoveInput{Destination: destination})

	require.NoError(t, err)
	require.Equal(t, []string{destination}, mover.requested)
	require.Equal(t, 1, restarter.requests)
}

func TestRequestDataHomeMoveWrapsAFailedStatusRead(t *testing.T) {
	mover := &fakeMover{statusErr: domainerrors.ErrUnavailable}
	restarter := &fakeRestarter{}

	err := application.NewRequestDataHomeMove(mover, &fakeTasks{}, restarter).Execute(context.Background(), application.MoveInput{Destination: destination})

	require.True(t, errors.Is(err, domainerrors.ErrUnavailable))
	require.Empty(t, mover.requested)
	require.Zero(t, restarter.requests)
}

func TestRequestDataHomeMoveWrapsAFailedTaskCheck(t *testing.T) {
	mover := &fakeMover{status: models.HomeStatus{Current: "/data/iwf", Source: models.SourceDefault}}
	boom := errors.New("tasks unreadable")

	err := application.NewRequestDataHomeMove(mover, &fakeTasks{err: boom}, &fakeRestarter{}).Execute(context.Background(), application.MoveInput{Destination: destination})

	require.True(t, errors.Is(err, boom))
	require.Empty(t, mover.requested)
}
