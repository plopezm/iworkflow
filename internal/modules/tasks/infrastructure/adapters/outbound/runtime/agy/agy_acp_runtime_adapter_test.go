package agy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
)

func TestAgyAcpRuntimeAdapterDeclaresCapabilities(t *testing.T) {
	adapter := NewAgyAcpRuntimeAdapter("agy", t.TempDir())
	taskID, err := models.NewTaskID()
	require.NoError(t, err)

	caps := adapter.RuntimeCapabilities(nil)

	assert.True(t, caps.Dialogs && caps.ModelSelection && caps.Resume && caps.Modes)
	assert.False(t, caps.Steer || caps.FollowUp || caps.ThinkingLevels || caps.Subagents)
	assert.Equal(t, caps, adapter.Capabilities(taskID))
}

func TestAgyAcpRuntimeAdapterDefaultArgsStartTheAcpServer(t *testing.T) {
	adapter := NewAgyAcpRuntimeAdapter("agy", t.TempDir())

	assert.Equal(t, []string{"acp"}, adapter.baseArgs)
}

func TestAgyAcpRuntimeAdapterSessionStorage(t *testing.T) {
	file := filepath.Join(t.TempDir(), "nested", "task")

	_, found := readSession(file)
	assert.False(t, found)
	saveSession(file, "sess-123")
	saved, found := readSession(file)
	assert.True(t, found)
	assert.Equal(t, "sess-123", saved)
	require.NoError(t, os.WriteFile(file, []byte("  \n"), 0o644))
	_, found = readSession(file)
	assert.False(t, found)
}
