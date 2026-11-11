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

// The editor of a script reads its name, command, arguments, variables and the whole timeout in seconds.
func TestSaveHookFromFormReadsAScript(t *testing.T) {
	t.Parallel()
	form := url.Values{
		dto.FieldName:       {"lint"},
		dto.FieldKind:       {"script"},
		dto.FieldTimeout:    {"90"},
		dto.FieldCommand:    {"./scripts/check.sh"},
		dto.FieldArgs:       {"--strict\n\n{{.Branch}}\r\n"},
		dto.FieldEnv:        {"CI=1\nMODE=fast"},
		dto.FieldUseShell:   {"on"},
		dto.FieldWorkingDir: {"sub"},
	}

	input, err := dto.SaveHookFromForm(form)

	require.NoError(t, err)
	assert.Equal(t, "lint", input.Name)
	assert.Equal(t, models.KindScript, input.Kind)
	assert.Equal(t, 90*time.Second, input.Timeout)
	assert.Equal(t, models.ScriptSpec{
		Command:    "./scripts/check.sh",
		Args:       []string{"--strict", "{{.Branch}}"},
		Env:        []models.EnvVar{{Name: "CI", Value: "1"}, {Name: "MODE", Value: "fast"}},
		UseShell:   true,
		WorkingDir: "sub",
	}, input.Script)
}

// The id of the form edits a stored hook; a form without it creates one.
func TestSaveHookFromFormKeepsTheIdOnlyWhenEditing(t *testing.T) {
	t.Parallel()
	input, err := dto.SaveHookFromForm(url.Values{dto.FieldID: {" h-1 "}, dto.FieldTimeout: {"5"}, dto.FieldKind: {"script"}})

	require.NoError(t, err)
	assert.Equal(t, "h-1", input.ID)
}

// Invalid timeouts and variables are refused before the use case runs; the message names the rule that broke.
func TestSaveHookFromFormRejectsBadValues(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		form url.Values
		want string
	}{
		{name: "timeout is not a number", form: url.Values{dto.FieldTimeout: {"soon"}}, want: "whole number of seconds"},
		{name: "timeout is zero", form: url.Values{dto.FieldTimeout: {"0"}}, want: "at least 1 second"},
		{name: "timeout is over the maximum", form: url.Values{dto.FieldTimeout: {"86401"}}, want: "too long"},
		{name: "variable without a value separator", form: url.Values{dto.FieldTimeout: {"5"}, dto.FieldEnv: {"CI"}}, want: "NAME=VALUE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := dto.SaveHookFromForm(tt.form)
			require.ErrorIs(t, err, hookerrors.ErrInvalidHook)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// An assignment is a project, event, moment, policy and enabled flag; a checkbox is enabled only when present.
func TestSaveAssignmentFromFormReadsTheProjectAndTheFlags(t *testing.T) {
	t.Parallel()
	form := url.Values{
		dto.FieldProject:   {"ws-2"},
		dto.FieldEvent:     {"task_finish"},
		dto.FieldPhase:     {"post"},
		dto.FieldOnFailure: {"warn"},
		dto.FieldEnabled:   {"on"},
	}

	input := dto.SaveAssignmentFromForm(form, "h-1")

	assert.Equal(t, application.SaveAssignmentInput{
		HookID: "h-1", WorkspaceID: "ws-2", Event: models.EventTaskFinish, Phase: models.PhasePost,
		Enabled: true, OnFailure: models.FailureWarn,
	}, input)
	assert.False(t, dto.SaveAssignmentFromForm(url.Values{}, "h-1").Enabled, "no checkbox means disabled")
}

// The history is failed-only until a form applies a filter: without the applied marker the failed filter is on.
func TestHistoryFromQueryIsFailedOnlyUntilAFilterIsApplied(t *testing.T) {
	t.Parallel()
	assert.Equal(t, dto.History{Failed: true}, dto.HistoryFromQuery(url.Values{}))
	assert.Equal(t, dto.History{HookID: "h-1", ProjectID: "ws-1", Failed: true, Applied: true},
		dto.HistoryFromQuery(url.Values{"applied": {"1"}, "hook": {"h-1"}, "project": {"ws-1"}, "failed": {"1"}}))
	assert.Equal(t, dto.History{Applied: true},
		dto.HistoryFromQuery(url.Values{"applied": {"1"}}), "an applied filter with the box unchecked shows every run")
}

// The run filter of the history carries the hook, the project and the failed flag, bounded by the limit.
func TestHistoryRunFilterMapsEveryField(t *testing.T) {
	t.Parallel()
	filter := dto.History{HookID: "h-1", ProjectID: "ws-1", Failed: false, Applied: true}.RunFilter(50)

	assert.Equal(t, "h-1", filter.HookID)
	assert.Equal(t, "ws-1", filter.WorkspaceID)
	assert.False(t, filter.FailedOnly)
	assert.Equal(t, 50, filter.Limit)
}

// A group is read from the form or the query, and its order is the ids in the order they were sent.
func TestGroupAndReorderReadTheSentValues(t *testing.T) {
	t.Parallel()
	form := url.Values{
		dto.FieldProject: {"ws-1"},
		dto.FieldEvent:   {"task_start"},
		dto.FieldPhase:   {"pre"},
		dto.FieldOrder:   {"a2, a1,,a3"},
	}

	assert.Equal(t, dto.Group{Project: "ws-1", Event: models.EventTaskStart, Phase: models.PhasePre}, dto.GroupFromForm(form))
	assert.Equal(t, []string{"a2", "a1", "a3"}, dto.ReorderFromForm(form))
}

// The test form names the hook and the project, event and moment the test pretends to be.
func TestTestHookFromFormNamesTheProjectAndTheMoment(t *testing.T) {
	t.Parallel()
	input := dto.TestHookFromForm(url.Values{dto.FieldProject: {"ws-1"}, dto.FieldEvent: {"task_finish"}, dto.FieldPhase: {"post"}}, "h-1")

	assert.Equal(t, "ws-1", input.WorkspaceID)
	assert.Equal(t, "h-1", input.HookID)
	assert.Equal(t, models.EventTaskFinish, input.Event)
	assert.Equal(t, models.PhasePost, input.Phase)
}
