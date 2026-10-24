package pi

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

func TestPiRpcRuntimeLaunchArgsAreTheBaseArgsByDefault(t *testing.T) {
	// Rust: pi_rpc_runtime_adapter::tests::launch_args_are_the_base_args_by_default
	assert.Equal(t, []string{"--mode", "rpc"}, launchArgs(rpcArgs(), false, models.LaunchOptions{}))
}

func TestPiRpcRuntimeLaunchArgsAppendResumeModelAndThinking(t *testing.T) {
	// Rust: pi_rpc_runtime_adapter::tests::launch_args_append_resume_model_and_thinking
	options := models.LaunchOptions{
		Model:    &models.LaunchModel{Provider: "anthropic", Model: "claude 5"},
		Thinking: ptrTo("high"),
	}

	assert.Equal(t,
		[]string{"--mode", "rpc", "--continue", "--provider", "anthropic", "--model", "claude 5", "--thinking", "high"},
		launchArgs(rpcArgs(), true, options),
	)
}

func TestPiRpcRuntimeLaunchArgsDoNotModifyTheBaseArgs(t *testing.T) {
	// Go-only triangulation: appending to a base slice with spare capacity must not leak into the next launch.
	base := make([]string, 2, 16)
	copy(base, []string{"--mode", "rpc"})
	options := models.LaunchOptions{Thinking: ptrTo("low")}

	first := launchArgs(base, true, options)
	second := launchArgs(base, false, models.LaunchOptions{})

	require.Equal(t, []string{"--mode", "rpc"}, second)
	require.Equal(t, []string{"--mode", "rpc", "--continue", "--thinking", "low"}, first)
	assert.Equal(t, []string{"--mode", "rpc"}, base[:2])
}

func TestPiRpcRuntimeDeclaresTheSubagentsCapability(t *testing.T) {
	// Rust: pi_rpc_runtime_adapter_test.rs::declares_the_subagents_capability
	adapter := NewRpcRuntimeAdapter("pi")
	assert.True(t, adapter.Capabilities(models.TaskID{}).Subagents)
	assert.True(t, adapter.RuntimeCapabilities(nil).Subagents)
}

func TestPiRpcRuntimeDeclaresNoModes(t *testing.T) {
	// Go-only triangulation: pi has no modes, so the SetMode command is filtered by capability before it reaches the adapter.
	adapter := NewRpcRuntimeAdapter("pi")
	assert.False(t, adapter.Capabilities(models.TaskID{}).Modes)
	assert.False(t, adapter.RuntimeCapabilities(nil).Modes)
}
