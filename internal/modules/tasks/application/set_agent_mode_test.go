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

// Rust: set_agent_mode::sends_the_mode_to_a_live_agent_that_supports_it
func TestSetAgentModeSendsTheModeToALiveAgentThatSupportsIt(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{capabilities: models.AgentCapabilities{Modes: true}}

	err := application.NewSetAgentMode(lifecycle, runtime).Execute(context.Background(), task.ID, models.AgentModePlan)

	require.NoError(t, err)
	assert.Equal(t, []models.AgentCommand{models.SetModeCommand{Mode: models.AgentModePlan}}, runtime.sent)
}

// Rust: set_agent_mode::rejects_runtimes_without_modes_and_sends_nothing
func TestSetAgentModeRejectsRuntimesWithoutModesAndSendsNothing(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusIdle)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{capabilities: models.MinimalAgentCapabilities()}

	err := application.NewSetAgentMode(lifecycle, runtime).Execute(context.Background(), task.ID, models.AgentModeAgent)

	require.ErrorIs(t, err, werrors.Unsupported("el agente no permite cambiar de modo"))
	assert.Empty(t, runtime.sent)
}

// Rust: set_agent_mode::rejects_when_agent_not_live
func TestSetAgentModeRejectsWhenAgentNotLive(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusMerged)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{capabilities: models.FullAgentCapabilities()}

	err := application.NewSetAgentMode(lifecycle, runtime).Execute(context.Background(), task.ID, models.AgentModePlan)

	require.ErrorIs(t, err, werrors.InvalidState("cambiar el modo", "merged"))
	assert.Empty(t, runtime.sent)
}

// Rule from source: an unknown mode is refused before the runtime is asked for its capabilities, and nothing is sent.
// Rust's enum makes this unrepresentable, so the check guards the Go string type.
func TestSetAgentModeRejectsUnknownModeBeforeAskingTheRuntime(t *testing.T) {
	t.Parallel()
	task := newTask(models.StatusRunning)
	lifecycle, _ := newLifecycle(newFakeRepository(task))
	runtime := &fakeRuntime{capabilities: models.FullAgentCapabilities()}

	err := application.NewSetAgentMode(lifecycle, runtime).Execute(context.Background(), task.ID, models.AgentMode("turbo"))

	require.ErrorIs(t, err, werrors.Unsupported("modo de agente desconocido: turbo"))
	assert.Equal(t, 0, runtime.capabilityCalls)
	assert.Empty(t, runtime.sent)
}
