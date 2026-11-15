package app_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The usage bar sits in the sidebar footer, right above the agent settings bar, and loads on page load.
func TestTheUsageBarIsInTheFooterAboveTheAgentSettingsBar(t *testing.T) {
	body := get(t, newRouter(t), "/").Body.String()

	usageIdx := strings.Index(body, `id="usage-bar"`)
	agentIdx := strings.Index(body, `id="agent-settings-bar"`)
	require.Greater(t, usageIdx, 0, "the usage bar placeholder must be rendered")
	require.Greater(t, agentIdx, 0, "the agent settings bar placeholder must be rendered")
	require.Less(t, usageIdx, agentIdx, "the usage bar must precede the agent settings bar")
	require.Contains(t, body, `hx-get="/usage-limits/bar"`)
}
