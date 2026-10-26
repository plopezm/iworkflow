package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

// The rules of the composer that the server applies: a message that is exactly /plan or /agent switches the mode
// when the runtime reports one, as the Rust composer does (composer_completion.rs and task_detail_state.rs).

func TestModeCommandMatchesOnlyTheWholeMessage(t *testing.T) {
	t.Parallel()
	mode, ok := state.ModeCommand("/plan")
	assert.True(t, ok)
	assert.Equal(t, "plan", mode)

	mode, ok = state.ModeCommand("  /agent\n")
	assert.True(t, ok, "surrounding whitespace is ignored")
	assert.Equal(t, "agent", mode)

	for _, text := range []string{"/plan now", "plan", "/compact", "hola /plan", ""} {
		_, ok = state.ModeCommand(text)
		assert.False(t, ok, "%q is not a mode command", text)
	}
}

func TestTranscriptReportsTheModeOnlyWhenTheRuntimeSentOne(t *testing.T) {
	t.Parallel()
	withMode := state.Transcript{Statuses: map[string]string{state.ModeStatusKey: "plan"}}
	mode, ok := withMode.Mode()
	assert.True(t, ok)
	assert.Equal(t, "plan", mode)

	mode, ok = state.Transcript{}.Mode()
	assert.False(t, ok, "a runtime without modes reports none")
	assert.Empty(t, mode)
}

func TestComposerViewTakesTheModeOfTheTranscriptAndNamesItsElements(t *testing.T) {
	t.Parallel()
	transcript := state.Transcript{Statuses: map[string]string{state.ModeStatusKey: "agent"}}

	view := state.NewComposerView("k3", "task-1", transcript)

	assert.Equal(t, "agent", view.Mode)
	assert.True(t, view.HasMode())
	assert.Equal(t, "task-composer-k3", view.ID())
	assert.Equal(t, "task-composer-mode-k3", view.ModeID())
	assert.Equal(t, "/tasks/task-1/messages?scope=k3", view.MessagesPath())
	assert.Equal(t, "/tasks/task-1/mode?scope=k3", view.ModePath())
	assert.Equal(t, "/tasks/task-1/composer/files", view.FilesPath())

	plain := state.NewComposerView("", "task-1", state.Transcript{})
	assert.False(t, plain.HasMode(), "no mode is reported, so the mode switch stays hidden")
	assert.Equal(t, "task-composer", plain.ID(), "a request without a scope keeps the plain names")
}
