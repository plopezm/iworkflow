package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/data_home/application"
)

func TestCancelDataHomeMoveWithdrawsThePendingMove(t *testing.T) {
	mover := &fakeMover{}

	require.NoError(t, application.NewCancelDataHomeMove(mover).Execute(context.Background()))
	require.Equal(t, 1, mover.cancels)
}

func TestCancelDataHomeMoveWrapsTheFailure(t *testing.T) {
	boom := errors.New("cannot remove")
	mover := &fakeMover{cancelErr: boom}

	err := application.NewCancelDataHomeMove(mover).Execute(context.Background())

	require.True(t, errors.Is(err, boom))
}
