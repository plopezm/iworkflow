package pty

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/env"
)

func kv(pairs ...string) []env.Var {
	vars := make([]env.Var, 0, len(pairs)/2)
	for index := 0; index+1 < len(pairs); index += 2 {
		vars = append(vars, env.Var{Key: pairs[index], Value: pairs[index+1]})
	}
	return vars
}

// envMap turns the KEY=VALUE entries of a command into a map for assertions.
func envMap(entries []string) map[string]string {
	result := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, _ := strings.Cut(entry, "=")
		result[key] = value
	}
	return result
}

func TestShellCommandIsALoginShellWithACleanEnvironment(t *testing.T) {
	vars := kv(
		"LEPTOS_SITE_ADDR", "127.0.0.1:3000",
		"CARGO_MANIFEST_DIR", "/src",
		"VIRTUAL_ENV", "/venv",
		"PATH", "/venv/bin:/usr/bin",
		"HOME", "/home/u",
		"TERM", "dumb",
	)

	cmd := shellCommand(env.Posix, "/bin/zsh", "/tmp", vars)

	require.Equal(t, []string{"/bin/zsh", "-l"}, cmd.Args)
	require.Equal(t, "/tmp", cmd.Dir)
	got := envMap(cmd.Env)
	require.NotContains(t, got, "LEPTOS_SITE_ADDR")
	require.NotContains(t, got, "CARGO_MANIFEST_DIR")
	require.NotContains(t, got, "VIRTUAL_ENV")
	require.Equal(t, env.DefaultPath, got["PATH"])
	require.Equal(t, "/home/u", got["HOME"])
	require.Equal(t, termValue, got["TERM"])
}

func TestParentVarsReadsTheProcessEnvironment(t *testing.T) {
	t.Setenv("IWF_TERMINAL_SHELL_TEST", "value=with-equals")

	vars := parentVars()

	found := false
	for _, entry := range vars {
		if entry.Key == "IWF_TERMINAL_SHELL_TEST" {
			require.Equal(t, "value=with-equals", entry.Value)
			found = true
		}
	}
	require.True(t, found)
}
