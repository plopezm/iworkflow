package components_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/components"
)

func TestSidebarIsOpenUnlessExplicitlyStoredClosed(t *testing.T) {
	require.True(t, components.ParseSidebarOpen("", false))
	require.True(t, components.ParseSidebarOpen("1", true))
	require.True(t, components.ParseSidebarOpen("garbage", true))
	require.False(t, components.ParseSidebarOpen("0", true))
}

func TestHandleGlyphPointsTowardTheCollapseDirection(t *testing.T) {
	require.Equal(t, "‹", components.HandleGlyph(components.LeftPanel, true))
	require.Equal(t, "›", components.HandleGlyph(components.LeftPanel, false))
	require.Equal(t, "›", components.HandleGlyph(components.RightPanel, true))
	require.Equal(t, "‹", components.HandleGlyph(components.RightPanel, false))
}
