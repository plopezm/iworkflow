package env

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// kv builds Vars from alternating key, value arguments.
func kv(entries ...string) []Var {
	vars := make([]Var, 0, len(entries)/2)
	for index := 0; index+1 < len(entries); index += 2 {
		vars = append(vars, Var{Key: entries[index], Value: entries[index+1]})
	}
	return vars
}

func TestSessionEnvDropsVariablesSetByTheParent(t *testing.T) {
	input := kv(
		"LEPTOS_SITE_ADDR", "127.0.0.1:3000",
		"IWF_DATABASE_URL", "x.db",
		"CARGO_MANIFEST_DIR", "/src",
		"RUST_LOG", "debug",
		"VIRTUAL_ENV", "/venv",
		"PATH", "/venv/bin:/usr/bin",
		"HOME", "/home/u",
		"LC_ALL", "C",
		"MY_HOME", "/nope",
	)

	got := SessionEnvFor(Posix, input)

	assert.Equal(t, kv("HOME", "/home/u", "LC_ALL", "C"), got)
}

func TestSessionEnvKeepsSystemSessionVariables(t *testing.T) {
	input := kv(
		"DISPLAY", ":0",
		"WAYLAND_DISPLAY", "wayland-1",
		"XDG_RUNTIME_DIR", "/run/user/1000",
		"DBUS_SESSION_BUS_ADDRESS", "unix:path=/run/user/1000/bus",
		"LANG", "es_ES.UTF-8",
		"LC_TIME", "es_ES.UTF-8",
		"CARGO_PKG_NAME", "iworkflow",
	)

	got := SessionEnvFor(Posix, input)

	assert.Len(t, got, 6)
	for _, entry := range got {
		assert.NotEqual(t, "CARGO_PKG_NAME", entry.Key)
	}
}

func TestSessionEnvKeepsNonUTF8LocaleKeysWithoutPanicking(t *testing.T) {
	odd := Var{Key: "LC_\xff", Value: "v"}

	got := SessionEnvFor(Posix, []Var{odd})

	assert.Equal(t, []Var{odd}, got)
}

func TestShellQuoteWrapsInSingleQuotes(t *testing.T) {
	assert.Equal(t, "'a b'", ShellQuote("a b"))
	assert.Equal(t, "''", ShellQuote(""))
}

func TestShellQuoteEscapesEmbeddedSingleQuotes(t *testing.T) {
	assert.Equal(t, `'it'\''s'`, ShellQuote("it's"))
}

func TestInvocationRunsTheQuotedProgramThroughTheSessionShell(t *testing.T) {
	input := kv(
		"SHELL", "/bin/zsh",
		"HOME", "/home/u",
		"PATH", "/venv/bin",
		"CARGO_MANIFEST_DIR", "/src",
	)

	got := InvocationFor(Posix, "/opt/my agent", []string{"--mode", "it's"}, input)

	assert.Equal(t, "/bin/zsh", got.Shell)
	assert.Equal(t, []string{"-l", "-c", `exec '/opt/my agent' '--mode' 'it'\''s'`}, got.Args)
	assert.Equal(t, kv("SHELL", "/bin/zsh", "HOME", "/home/u", "PATH", DefaultPath), got.Env)
}

func TestInvocationFallsBackToBinShWithoutAShell(t *testing.T) {
	missing := InvocationFor(Posix, "agent", nil, kv("HOME", "/h"))
	empty := InvocationFor(Posix, "agent", nil, kv("SHELL", ""))

	assert.Equal(t, "/bin/sh", missing.Shell)
	assert.Equal(t, "/bin/sh", empty.Shell)
	assert.Equal(t, "exec 'agent'", missing.Args[2])
}

func TestPowerShellEnvKeepsSystemVariablesIgnoringCaseAndInheritsPath(t *testing.T) {
	input := kv(
		"Path", `C:\Windows\System32`,
		"SystemRoot", `C:\Windows`,
		"USERPROFILE", `C:\Users\u`,
		"LC_ALL", "C",
		"VIRTUAL_ENV", `C:\venv`,
		"CARGO_MANIFEST_DIR", `C:\src`,
		"SHELL", `C:\Git\bash.exe`,
	)

	got := SessionEnvFor(PowerShell, input)

	assert.Equal(t, kv(
		"Path", `C:\Windows\System32`,
		"SystemRoot", `C:\Windows`,
		"USERPROFILE", `C:\Users\u`,
		"LC_ALL", "C",
	), got)
}

func TestPowerShellQuoteDoublesSingleQuotes(t *testing.T) {
	assert.Equal(t, "'a b'", PowerShellQuote("a b"))
	assert.Equal(t, "'it''s'", PowerShellQuote("it's"))
}

func TestPowerShellQuoteDoublesTypographicSingleQuotes(t *testing.T) {
	got := PowerShellQuote("a\u2018b\u2019c\u201Ad\u201Be")

	assert.Equal(t, "'a\u2018\u2018b\u2019\u2019c\u201A\u201Ad\u201B\u201Be'", got)
}

func TestPowerShellInvocationIgnoresTheShellAndKeepsTheSystemPath(t *testing.T) {
	input := kv(
		"SHELL", `C:\Git\bash.exe`,
		"Path", `C:\Windows`,
		"USERPROFILE", `C:\Users\u`,
	)

	got := InvocationFor(PowerShell, `C:\Program Files\agent.exe`, []string{"--mode", "it's"}, input)

	assert.Equal(t, "powershell.exe", got.Shell)
	assert.Equal(t, []string{
		"-NoLogo",
		"-NonInteractive",
		"-Command",
		`& 'C:\Program Files\agent.exe' '--mode' 'it''s'; exit $LASTEXITCODE`,
	}, got.Args)
	assert.Equal(t, kv("Path", `C:\Windows`, "USERPROFILE", `C:\Users\u`), got.Env)
}

func TestInteractiveArgsAndBasePathDependOnTheFlavor(t *testing.T) {
	assert.Equal(t, []string{"-l"}, Posix.InteractiveArgs())
	assert.Equal(t, []string{"-NoLogo"}, PowerShell.InteractiveArgs())

	base, ok := Posix.BasePath()
	assert.True(t, ok)
	assert.Equal(t, DefaultPath, base)

	_, ok = PowerShell.BasePath()
	assert.False(t, ok)

	assert.Equal(t, "powershell.exe", PowerShell.SelectShell("/bin/zsh"))
	assert.Equal(t, "/bin/sh", Posix.SelectShell(""))
}
