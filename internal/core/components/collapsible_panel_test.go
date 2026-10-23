package components

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPanelClassMarksTheCollapsedState(t *testing.T) {
	require.Equal(t, "side-panel", collapsiblePanelClass(false))
	require.Equal(t, "side-panel side-panel-collapsed", collapsiblePanelClass(true))
}

func TestToggleLabelOffersTheOppositeAction(t *testing.T) {
	require.Equal(t, "Expand", toggleLabel(true, "Expand", "Collapse"))
	require.Equal(t, "Collapse", toggleLabel(false, "Expand", "Collapse"))
}
