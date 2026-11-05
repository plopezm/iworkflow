package system

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/datahome"
	domainerrors "iworkflow/internal/modules/data_home/domain/errors"
)

func TestMapRequestErrorGivesEverySentinelItsReason(t *testing.T) {
	cases := []struct {
		name     string
		sentinel error
		reason   domainerrors.DestinationReason
	}{
		{name: "not absolute", sentinel: datahome.ErrNotAbsolute, reason: domainerrors.ReasonRelative},
		{name: "same folder", sentinel: datahome.ErrSameFolder, reason: domainerrors.ReasonSame},
		{name: "nested", sentinel: datahome.ErrNested, reason: domainerrors.ReasonNested},
		{name: "not empty", sentinel: datahome.ErrNotEmpty, reason: domainerrors.ReasonNotEmpty},
		{name: "not writable", sentinel: datahome.ErrNotWritable, reason: domainerrors.ReasonNotWritable},
		{name: "no space", sentinel: datahome.ErrNoSpace, reason: domainerrors.ReasonNoSpace},
		{name: "unsafe source", sentinel: datahome.ErrUnsafeSource, reason: domainerrors.ReasonSourceUnsafe},
		{name: "not a data folder", sentinel: datahome.ErrNotDataFolder, reason: domainerrors.ReasonSourceMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := mapRequestError(fmt.Errorf("validate: %w", tc.sentinel))

			var destinationErr *domainerrors.DestinationError
			require.True(t, errors.As(err, &destinationErr), "got %v", err)
			require.Equal(t, tc.reason, destinationErr.Reason)
		})
	}
}

func TestMapRequestErrorKeepsAPendingMoveAndUnknownFailuresApart(t *testing.T) {
	require.ErrorIs(t, mapRequestError(datahome.ErrPending), domainerrors.ErrMovePending)

	unknown := errors.New("disk on fire")
	err := mapRequestError(unknown)
	require.ErrorIs(t, err, unknown)
	var destinationErr *domainerrors.DestinationError
	require.False(t, errors.As(err, &destinationErr), "an unexpected failure is not a destination reason")
}
