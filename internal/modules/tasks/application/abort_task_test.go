package application_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/application"
	werrors "iworkflow/internal/modules/tasks/domain/errors"
	"iworkflow/internal/modules/tasks/domain/models"
)

// Rust: abort_task::sends_abort_to_live_agent
func TestAbortTaskSendsAbortToLiveAgent(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{}

	err := application.NewAbortTask(lifecycle, runtime).Execute(context.Background(), task.ID)

	require.NoError(t, err)
	assert.Equal(t, []models.AgentCommand{models.AbortCommand{}}, runtime.sent)
}

// Rust: abort_task::rejects_when_agent_not_live
func TestAbortTaskRejectsWhenAgentNotLive(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusMerged)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{}

	err := application.NewAbortTask(lifecycle, runtime).Execute(context.Background(), task.ID)

	require.ErrorIs(t, err, werrors.InvalidState("abortar", "merged"))
	assert.Empty(t, runtime.sent)
}

// Rule from source: every live status accepts an abort (starting and idle included), not only running.
func TestAbortTaskAcceptsEveryLiveStatus(t *testing.T) {
	t.Parallel()
	for _, status := range []models.TaskStatus{models.StatusStarting, models.StatusIdle} {
		task := newTask(status)
		lifecycle, _ := newLifecycle(newFakeRepository(task))
		runtime := &fakeRuntime{}

		err := application.NewAbortTask(lifecycle, runtime).Execute(context.Background(), task.ID)

		require.NoError(t, err, status.String())
		assert.Equal(t, []models.AgentCommand{models.AbortCommand{}}, runtime.sent, status.String())
	}
}
