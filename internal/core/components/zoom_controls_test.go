package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestZoomIsShownAsARoundedPercentage(t *testing.T) {
	require.Equal(t, "100%", ZoomPercentLabel(1.0))
	require.Equal(t, "46%", ZoomPercentLabel(0.4623))
	require.Equal(t, "200%", ZoomPercentLabel(2.0))
}
