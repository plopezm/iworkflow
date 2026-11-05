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

func TestGetDataHomeReturnsTheStatusOfTheMover(t *testing.T) {
	want := models.HomeStatus{Current: "/data/iwf", Source: models.SourcePointer}
	mover := &fakeMover{status: want}

	got, err := application.NewGetDataHome(mover).Execute(context.Background())

	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestGetDataHomeWrapsTheFailureOfTheMover(t *testing.T) {
	mover := &fakeMover{statusErr: domainerrors.ErrUnavailable}

	_, err := application.NewGetDataHome(mover).Execute(context.Background())

	require.Error(t, err)
	require.True(t, errors.Is(err, domainerrors.ErrUnavailable))
}
