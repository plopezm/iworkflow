package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionEnvDropsVariablesSetByTheParent(t *testing.T) {
	vars := []string{
		"LEPTOS_SITE_ADDR=127.0.0.1:3000",
		"IWF_DATABASE_URL=x.db",
		"CARGO_MANIFEST_DIR=/src",
		"RUST_LOG=debug",
		"VIRTUAL_ENV=/venv",
		"PATH=/venv/bin:/usr/bin",
		"HOME=/home/u",
		"LC_ALL=C",
		"MY_HOME=/nope",
	}

	kept := sessionEnv(flavorPOSIX, vars)

	require.Equal(t, []string{"HOME=/home/u", "LC_ALL=C"}, kept)
}

func TestSessionEnvKeepsSystemSessionVariables(t *testing.T) {
	vars := []string{
		"DISPLAY=:0",
		"WAYLAND_DISPLAY=wayland-1",
		"XDG_RUNTIME_DIR=/run/user/1000",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1000/bus",
		"LANG=es_ES.UTF-8",
		"LC_TIME=es_ES.UTF-8",
		"CARGO_PKG_NAME=iworkflow",
	}

	kept := sessionEnv(flavorPOSIX, vars)

	require.Len(t, kept, 6)
	require.NotContains(t, kept, "CARGO_PKG_NAME=iworkflow")
}

func TestPowerShellEnvIgnoresKeyCaseAndKeepsPath(t *testing.T) {
	vars := []string{
		`Path=C:\Windows\System32`,
		`SystemRoot=C:\Windows`,
		`USERPROFILE=C:\Users\u`,
		"LC_ALL=C",
		"VIRTUAL_ENV=C:\\venv",
		`SHELL=C:\Git\bash.exe`,
	}

	kept := sessionEnv(flavorPowerShell, vars)

	require.Equal(t, []string{`Path=C:\Windows\System32`, `SystemRoot=C:\Windows`, `USERPROFILE=C:\Users\u`, "LC_ALL=C"}, kept)
}

func TestPowerShellEnvDoesNotFoldNonAsciiLetters(t *testing.T) {
	// "ſ" (long s) must not match "S" the way Unicode case folding would.
	kept := sessionEnv(flavorPowerShell, []string{"SYSTEMROOT=C:\\", "ſystemroot=C:\\"})

	require.Equal(t, []string{"SYSTEMROOT=C:\\"}, kept)
}

func TestShellQuoteWrapsInSingleQuotes(t *testing.T) {
	require.Equal(t, "'a b'", shellQuote("a b"))
	require.Equal(t, "''", shellQuote(""))
}

func TestShellQuoteEscapesEmbeddedSingleQuotes(t *testing.T) {
	require.Equal(t, `'it'\''s'`, shellQuote("it's"))
}

func TestPowerShellQuoteDoublesEveryKindOfSingleQuote(t *testing.T) {
	require.Equal(t, "'it''s'", powerShellQuote("it's"))
	require.Equal(t, "'a‘‘b'", powerShellQuote("a‘b"))
}

func TestInvocationRunsTheQuotedProgramThroughTheSessionShell(t *testing.T) {
	vars := []string{"SHELL=/bin/zsh", "HOME=/home/u", "PATH=/venv/bin", "CARGO_MANIFEST_DIR=/src"}

	invocation := loginShellInvocationFor(flavorPOSIX, "/opt/my agent", []string{"--mode", "it's"}, vars)

	require.Equal(t, "/bin/zsh", invocation.Shell)
	require.Equal(t, []string{"-l", "-c", `exec '/opt/my agent' '--mode' 'it'\''s'`}, invocation.Args)
	require.Equal(t, []string{"SHELL=/bin/zsh", "HOME=/home/u", "PATH=" + defaultPOSIXPath}, invocation.Env)
}

func TestInvocationFallsBackToBinShWithoutAShell(t *testing.T) {
	missing := loginShellInvocationFor(flavorPOSIX, "agent", nil, []string{"HOME=/h"})
	empty := loginShellInvocationFor(flavorPOSIX, "agent", nil, []string{"SHELL="})

	require.Equal(t, "/bin/sh", missing.Shell)
	require.Equal(t, "/bin/sh", empty.Shell)
	require.Equal(t, "exec 'agent'", missing.Args[2])
}

func TestInvocationFallsBackToBinShWhenShellIsNotUTF8(t *testing.T) {
	invocation := loginShellInvocationFor(flavorPOSIX, "agent", nil, []string{"SHELL=/bin/\xff"})

	require.Equal(t, "/bin/sh", invocation.Shell)
}

func TestInvocationNeverInheritsTheParentEnvironmentWhenEmpty(t *testing.T) {
	invocation := loginShellInvocationFor(flavorPowerShell, "agent", nil, []string{"VIRTUAL_ENV=/venv"})

	require.NotNil(t, invocation.Env)
	require.Empty(t, invocation.Env)
}

func TestPowerShellInvocationUsesCommandMode(t *testing.T) {
	invocation := loginShellInvocationFor(flavorPowerShell, "my agent", []string{"acp"}, []string{"USERPROFILE=C:\\u"})

	require.Equal(t, "powershell.exe", invocation.Shell)
	require.Equal(t, []string{"-NoLogo", "-NonInteractive", "-Command", "& 'my agent' 'acp'; exit $LASTEXITCODE"},
		invocation.Args)
	require.Equal(t, []string{"USERPROFILE=C:\\u"}, invocation.Env)
}
