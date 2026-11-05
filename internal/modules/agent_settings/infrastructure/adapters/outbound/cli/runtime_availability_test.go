package cli

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/agent_settings/domain/models"
)

func TestInstalledFindsTheBinaryInTheLoginShellAndRejectsAMissingOne(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX login shell")
	}
	t.Setenv("SHELL", "/bin/sh")
	adapter := NewAgentCLIModelCatalogAdapter(map[models.AgentRuntime]string{
		models.RuntimePi:          "sh",
		models.RuntimeGentleShell: "iwf-definitely-not-installed",
		models.RuntimeClaude:      "",
	})

	require.True(t, adapter.Installed(context.Background(), models.RuntimePi))
	require.False(t, adapter.Installed(context.Background(), models.RuntimeGentleShell), "missing binary")
	require.False(t, adapter.Installed(context.Background(), models.RuntimeClaude), "empty binary")
	require.False(t, adapter.Installed(context.Background(), models.RuntimeCursor), "unregistered runtime")
}

func TestInstalledReusesTheAnswerUntilTheCacheExpires(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX login shell")
	}
	t.Setenv("SHELL", "/bin/sh")
	now := time.Now()
	adapter := NewAgentCLIModelCatalogAdapter(map[models.AgentRuntime]string{models.RuntimePi: "sh"})
	adapter.clock = func() time.Time { return now }
	require.True(t, adapter.Installed(context.Background(), models.RuntimePi))

	adapter.bins[models.RuntimePi] = "iwf-definitely-not-installed"
	require.True(t, adapter.Installed(context.Background(), models.RuntimePi), "fresh answer is reused")

	now = now.Add(installedCacheTTL + time.Second)
	require.False(t, adapter.Installed(context.Background(), models.RuntimePi), "expired answer is probed again")
}

func TestListModelsOfAMissingBinaryIsEmptyNotAnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX login shell")
	}
	t.Setenv("SHELL", "/bin/sh")
	adapter := NewAgentCLIModelCatalogAdapter(map[models.AgentRuntime]string{models.RuntimeGentleShell: "iwf-definitely-not-installed"})

	result, err := adapter.List(context.Background(), models.RuntimeGentleShell)

	require.NoError(t, err)
	require.Empty(t, result)
}

func TestProbeArgsPerFlavor(t *testing.T) {
	require.Equal(t, []string{"-l", "-c", "command -v 'my pi'"}, probeArgs(flavorPOSIX, "my pi"))
	require.Equal(t, []string{"-NoLogo", "-NonInteractive", "-Command",
		"if (Get-Command 'pi' -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"}, probeArgs(flavorPowerShell, "pi"))
}
