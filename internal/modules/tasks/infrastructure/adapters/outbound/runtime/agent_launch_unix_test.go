//go:build unix

package runtime

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/env"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: agent_process.rs::build_command_clears_the_environment_down_to_session_variables_and_extras
func TestBuildCommandClearsTheEnvironmentDownToSessionVariablesAndExtras(t *testing.T) {
	hostEnv := []env.Var{
		{Key: "LEPTOS_ENV", Value: "dev"},
		{Key: "CARGO_MANIFEST_DIR", Value: "/src"},
		{Key: "VIRTUAL_ENV", Value: "/venv"},
		{Key: "PATH", Value: "/venv/bin"},
		{Key: "HOME", Value: "/home/u"},
		{Key: "SHELL", Value: "/bin/zsh"},
	}
	spec := LaunchSpec{ExtraEnv: []env.Var{{Key: "AGENT_FLAG", Value: "1"}}}

	command := buildCommand("agent", spec, models.ResourceLimits{}, "/work", hostEnv, false)

	values := envMap(t, command.Env)
	assert.ElementsMatch(t, []string{"AGENT_FLAG", "HOME", "PATH", "SHELL"}, keysOf(values))
	assert.Equal(t, "1", values["AGENT_FLAG"])
	assert.Equal(t, "/home/u", values["HOME"])
	assert.Equal(t, "/bin/zsh", values["SHELL"])
	assert.Equal(t, env.DefaultPath, values["PATH"])
	assert.Equal(t, "/work", command.Dir)
}

// Rust: agent_process.rs::build_command_launches_the_agent_with_exec_through_the_login_shell
func TestBuildCommandLaunchesTheAgentWithExecThroughTheLoginShell(t *testing.T) {
	hostEnv := []env.Var{{Key: "HOME", Value: "/home/u"}, {Key: "SHELL", Value: "/bin/zsh"}}
	spec := LaunchSpec{AgentArgs: []string{"--mode", "rpc"}}

	command := buildCommand("agent", spec, models.ResourceLimits{}, ".", hostEnv, false)

	assert.Equal(t, "/bin/zsh", command.Path)
	assert.Equal(t, []string{"/bin/zsh", "-l", "-c", "exec 'agent' '--mode' 'rpc'"}, command.Args)
}

// Rule from source: env.SelectShell falls back to /bin/sh when the parent has no SHELL, so the agent is
// still launched through a login-capable shell. Pinned here because the runtime depends on it.
func TestBuildCommandFallsBackToBinShWhenTheParentHasNoShell(t *testing.T) {
	command := buildCommand("agent", LaunchSpec{}, models.ResourceLimits{}, ".", []env.Var{{Key: "HOME", Value: "/h"}}, false)

	assert.Equal(t, "/bin/sh", command.Path)
}

func envMap(t *testing.T, entries []string) map[string]string {
	t.Helper()
	values := make(map[string]string, len(entries))
	for _, entry := range entries {
		key, value, found := strings.Cut(entry, "=")
		require.True(t, found, "malformed environment entry %q", entry)
		values[key] = value
	}
	return values
}

func keysOf(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
