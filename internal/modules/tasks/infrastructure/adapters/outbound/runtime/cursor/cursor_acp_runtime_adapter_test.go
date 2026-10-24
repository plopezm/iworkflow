package cursor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: cursor_acp_runtime_adapter::tests::declares_dialogs_model_selection_and_resume_only
func TestCursorAcpRuntimeAdapterDeclaresDialogsModelSelectionAndResumeOnly(t *testing.T) {
	adapter := NewCursorAcpRuntimeAdapter("agent", t.TempDir())
	taskID, err := models.NewTaskID()
	require.NoError(t, err)

	caps := adapter.RuntimeCapabilities(nil)

	assert.True(t, caps.Dialogs && caps.ModelSelection && caps.Resume)
	assert.False(t, caps.Steer || caps.FollowUp || caps.ThinkingLevels || caps.Subagents)
	assert.Equal(t, caps, adapter.Capabilities(taskID))
}

// Rust: cursor_acp_runtime_adapter::tests::default_args_start_the_acp_server
func TestCursorAcpRuntimeAdapterDefaultArgsStartTheAcpServer(t *testing.T) {
	adapter := NewCursorAcpRuntimeAdapter("agent", t.TempDir())

	assert.Equal(t, []string{"acp"}, adapter.baseArgs)
}

// Rust: cursor_acp_runtime_adapter::tests::a_saved_session_round_trips_and_blank_or_missing_files_mean_none
func TestCursorAcpRuntimeAdapterASavedSessionRoundTripsAndBlankOrMissingFilesMeanNone(t *testing.T) {
	file := filepath.Join(t.TempDir(), "nested", "task")

	_, found := readSession(file)
	assert.False(t, found)
	saveSession(file, "abc-123")
	saved, found := readSession(file)
	assert.True(t, found)
	assert.Equal(t, "abc-123", saved)
	require.NoError(t, os.WriteFile(file, []byte("  \n"), 0o644))
	_, found = readSession(file)
	assert.False(t, found)
}
