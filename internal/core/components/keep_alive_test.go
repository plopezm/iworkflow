package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInactivePaneIsHiddenAndActivePaneIsShown(t *testing.T) {
	require.Equal(t, "contents", paneClass(true, ""))
	require.Equal(t, "absolute inset-0", paneClass(true, "absolute inset-0"))
	require.Equal(t, "hidden", paneClass(false, "absolute inset-0"))
}
