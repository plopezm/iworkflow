package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBusyClassAppendsTheBusyMarkerOnlyWhileBusy(t *testing.T) {
	require.Equal(t, "btn-primary", BusyClass("btn-primary", false))
	require.Equal(t, "btn-primary is-busy", BusyClass("btn-primary", true))
}
