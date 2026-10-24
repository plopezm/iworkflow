package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

func TestClaudeStreamRuntimeAdapterDefaultArgsRunClaudeInStreamJSONMode(t *testing.T) {
	// Rust: claude_stream_runtime_adapter::tests::default_args_run_claude_in_stream_json_mode
	adapter := NewStreamRuntimeAdapter("claude")

	require.Equal(t, streamJSONArgs(), launchArgs(adapter.baseArgs, false, models.LaunchOptions{}))
}

func TestClaudeStreamRuntimeAdapterDefaultArgsAcceptEditsAndLeaveTheRestToTheUser(t *testing.T) {
	// Rust: claude_stream_runtime_adapter::tests::default_args_accept_edits_and_leave_the_rest_to_the_user
	adapter := NewStreamRuntimeAdapter("claude")
	args := launchArgs(adapter.baseArgs, false, models.LaunchOptions{})

	position := -1
	for index, arg := range args {
		if arg == "--permission-mode" {
			position = index
			break
		}
	}
	require.NotEqual(t, -1, position)
	require.Less(t, position+1, len(args))
	assert.Equal(t, "acceptEdits", args[position+1])
}

func TestClaudeStreamRuntimeAdapterThinkingLevelsClaudeDoesNotSupportAreMapped(t *testing.T) {
	// Rust: claude_stream_runtime_adapter::tests::thinking_levels_claude_does_not_support_are_mapped
	argsFor := func(level string) []string {
		return launchArgs(nil, false, models.LaunchOptions{Thinking: &level})
	}

	assert.Empty(t, argsFor("off"))
	assert.Equal(t, []string{"--effort", "low"}, argsFor("minimal"))
	for _, level := range []string{"low", "medium", "high", "xhigh", "max"} {
		assert.Equal(t, []string{"--effort", level}, argsFor(level), level)
	}
}

func TestClaudeStreamRuntimeAdapterLaunchArgsAppendResumeModelAndEffortWithoutProvider(t *testing.T) {
	// Rust: claude_stream_runtime_adapter::tests::launch_args_append_resume_model_and_effort_without_provider
	high := "high"
	options := models.LaunchOptions{
		Model:    &models.LaunchModel{Provider: "anthropic", Model: "opus"},
		Thinking: &high,
	}

	require.Equal(t,
		[]string{"-p", "--continue", "--model", "opus", "--effort", "high"},
		launchArgs([]string{"-p"}, true, options),
	)
}
