package dto_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/infrastructure/adapters/inbound/dto"
)

func scriptForm() url.Values {
	return url.Values{
		dto.FieldID:        {" h1 "},
		dto.FieldEvent:     {"task_start"},
		dto.FieldPhase:     {"pre"},
		dto.FieldKind:      {"script"},
		dto.FieldEnabled:   {"on"},
		dto.FieldOnFailure: {"warn"},
		dto.FieldTimeout:   {"90"},
		dto.FieldCommand:   {"./setup.sh"},
		dto.FieldArgs:      {"--branch={{.Branch}}\n\n  \n--fast\r"},
		dto.FieldEnv:       {"MODE=ci\nEMPTY="},
		dto.FieldUseShell:  {"on"},
		dto.FieldPrompt:    {"ignored for scripts"},
	}
}

// The script form maps every field of a script hook: args and env are one per line, blank lines are ignored, the
// checkboxes are present when checked, and the timeout is in seconds.
func TestSaveHookFromScriptForm(t *testing.T) {
	t.Parallel()
	input, err := dto.SaveHookFromForm(scriptForm(), "w1")

	require.NoError(t, err)
	assert.Equal(t, application.SaveHookInput{
		ID: "h1", WorkspaceID: "w1", Phase: models.PhasePre, Event: models.EventTaskStart, Kind: models.KindScript,
		Enabled: true, OnFailure: models.FailureWarn, Timeout: 90 * time.Second,
		Script: models.ScriptSpec{
			Command:  "./setup.sh",
			Args:     []string{"--branch={{.Branch}}", "--fast"},
			Env:      []models.EnvVar{{Name: "MODE", Value: "ci"}, {Name: "EMPTY", Value: ""}},
			UseShell: true,
		},
		// The agent fields are read as typed; the domain keeps only the spec of the kind.
		Agent: models.AgentSpec{Prompt: "ignored for scripts"},
	}, input)
}

// An unchecked enabled box and an empty id mean a new disabled hook without shell.
func TestSaveHookFromAgentFormWithUncheckedBoxes(t *testing.T) {
	t.Parallel()
	form := url.Values{
		dto.FieldEvent: {"task_finish"}, dto.FieldPhase: {"post"}, dto.FieldKind: {"agent"},
		dto.FieldOnFailure: {"ignore"}, dto.FieldTimeout: {"600"}, dto.FieldPrompt: {"Check the tests"},
		dto.FieldRuntime: {"pi"}, dto.FieldModel: {"anthropic/claude"}, dto.FieldThinking: {"high"},
		dto.FieldProfile: {"fast"}, dto.FieldCommand: {"ignored"},
	}

	input, err := dto.SaveHookFromForm(form, "w1")

	require.NoError(t, err)
	assert.Empty(t, input.ID)
	assert.False(t, input.Enabled)
	assert.False(t, input.Script.UseShell)
	assert.Equal(t, models.AgentSpec{
		Prompt: "Check the tests", Runtime: "pi", Model: "anthropic/claude", Thinking: "high", Profile: "fast",
	}, input.Agent)
	assert.Equal(t, 10*time.Minute, input.Timeout)
}

// The timeout is whole seconds between 1 and the domain maximum, both included. Anything outside, or not a number,
// is refused with a form error before the domain sees it.
func TestSaveHookFromFormBoundsTheTimeout(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		text    string
		want    time.Duration
		message string
	}{
		{name: "zero is refused", text: "0", message: "at least 1 second"},
		{name: "negative is refused", text: "-1", message: "at least 1 second"},
		{name: "one second is the minimum", text: "1", want: time.Second},
		{name: "the domain maximum is accepted", text: "86400", want: models.MaxTimeout},
		{name: "one above the maximum is refused", text: "86401", message: "too long"},
		{name: "a number that overflows is refused", text: "99999999999999999999", message: "whole number of seconds"},
		{name: "text is refused", text: "abc", message: "whole number of seconds"},
		{name: "empty is refused", text: "", message: "whole number of seconds"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			form := scriptForm()
			form[dto.FieldTimeout] = []string{tt.text}

			input, err := dto.SaveHookFromForm(form, "w1")

			if tt.message != "" {
				require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
				assert.ErrorContains(t, err, tt.message)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, input.Timeout)
		})
	}
}

func TestSaveHookFromFormRefusesWhatCannotBeRead(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(url.Values)
		message string
	}{
		{name: "empty timeout", mutate: func(f url.Values) { f[dto.FieldTimeout] = []string{""} }, message: "whole number of seconds"},
		{name: "text timeout", mutate: func(f url.Values) { f[dto.FieldTimeout] = []string{"soon"} }, message: "whole number of seconds"},
		{name: "timeout above the maximum", mutate: func(f url.Values) { f[dto.FieldTimeout] = []string{"86401"} }, message: "too long"},
		{name: "env line without equals", mutate: func(f url.Values) { f[dto.FieldEnv] = []string{"NOEQUALS"} }, message: "NAME=VALUE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			form := scriptForm()
			tt.mutate(form)

			_, err := dto.SaveHookFromForm(form, "w1")

			require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func TestReorderFromFormKeepsTheOrderOfTheIDs(t *testing.T) {
	t.Parallel()
	ids := dto.ReorderFromForm(url.Values{dto.FieldOrder: {" b ,a,,c "}})

	assert.Equal(t, []string{"b", "a", "c"}, ids)
	assert.Empty(t, dto.ReorderFromForm(url.Values{}))
}
