package tasks_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/outbound/persistence"
)

// Spec workspace-default-agent T4: the first launch stores the workspace default in a task that has no runtime of
// its own, so a later change of the default does not switch the agent of that task.
func TestPinAgentRuntimeSetsTheRuntimeWhileTheTaskHasNone(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/pin")
	task := newTask(t, workspaceID, "pin", models.StatusBacklog)
	insertTask(t, repo, task)

	require.NoError(t, repo.Tasks.PinAgentRuntime(bg(), task.ID, "claude"))

	got := found(t, repo, task.ID)
	require.NotNil(t, got.Agent.Runtime)
	assert.Equal(t, "claude", got.Agent.Runtime.String())
}

// Spec workspace-default-agent T4: a runtime the task already has, set by the user or by an earlier pin, is kept.
func TestPinAgentRuntimeDoesNotOverwriteAnExistingRuntime(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/pin-existing")
	task := newTask(t, workspaceID, "pinned-before", models.StatusBacklog)
	task.Agent = models.TaskAgentOptions{Runtime: runtimeIDOf(t, "pi")}
	insertTask(t, repo, task)

	require.NoError(t, repo.Tasks.PinAgentRuntime(bg(), task.ID, "claude"))

	got := found(t, repo, task.ID)
	require.NotNil(t, got.Agent.Runtime)
	assert.Equal(t, "pi", got.Agent.Runtime.String())
	assert.Equal(t, task.Agent, got.Agent)
}

// Pinning is best effort: the store does not fail for a task that is gone, as the other conditional writes.
func TestPinAgentRuntimeForMissingTaskIsNotAnError(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)

	require.NoError(t, repo.Tasks.PinAgentRuntime(bg(), newTaskID(t), "claude"))
}

// Spec workspace-default-agent T4: the retrying decorator delegates the pin to the repository it wraps.
func TestRetryingRepositoryDelegatesPinAgentRuntime(t *testing.T) {
	t.Parallel()
	repo := newRepo(t)
	workspaceID := insertWorkspace(t, repo.DB, "/repos/pin-retry")
	task := newTask(t, workspaceID, "pin-retry", models.StatusBacklog)
	insertTask(t, repo, task)
	retrying := persistence.NewRetryingTaskRepository(repo.Tasks, persistence.BoundedRetryPolicy(), repo.clock.Now, persistence.ContextSleep)

	require.NoError(t, retrying.PinAgentRuntime(bg(), task.ID, "codex"))

	got := found(t, repo, task.ID)
	require.NotNil(t, got.Agent.Runtime)
	assert.Equal(t, "codex", got.Agent.Runtime.String())
}

// runtimeIDOf parses a runtime slug or fails the test.
func runtimeIDOf(t *testing.T, raw string) *models.RuntimeID {
	t.Helper()
	parsed, err := models.ParseRuntimeID(raw)
	require.NoError(t, err)
	return &parsed
}
