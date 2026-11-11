// Package dto maps the forms of the Hooks screen to the use case inputs.
package dto

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"iworkflow/internal/modules/task_hooks/application"
	hookerrors "iworkflow/internal/modules/task_hooks/domain/errors"
	"iworkflow/internal/modules/task_hooks/domain/models"
	"iworkflow/internal/modules/task_hooks/domain/ports/outbound"
)

// Form field names of the Hooks screen. Checkboxes are present in a form only when checked.
const (
	FieldID         = "id"
	FieldName       = "name"
	FieldKind       = "kind"
	FieldTimeout    = "timeout"
	FieldCommand    = "command"
	FieldArgs       = "args"
	FieldEnv        = "env"
	FieldUseShell   = "use_shell"
	FieldWorkingDir = "working_dir"
	FieldPrompt     = "prompt"
	FieldRuntime    = "runtime"
	FieldModel      = "model"
	FieldThinking   = "thinking"
	FieldProfile    = "profile"

	FieldProject   = "project"
	FieldEvent     = "event"
	FieldPhase     = "phase"
	FieldOnFailure = "on_failure"
	FieldEnabled   = "enabled"
	// FieldOrder carries the whole order of a group of assignments as comma-separated ids.
	FieldOrder = "order"

	FieldHook    = "hook"
	FieldFailed  = "failed"
	FieldApplied = "applied"
)

// SaveHookFromForm reads the editor of a hook. Arguments and variables are one per line, blank lines are ignored, and
// a variable is written NAME=VALUE. The timeout is in whole seconds. Fields of the other kind are read too and dropped
// by the domain, so switching the kind keeps nothing it should not.
func SaveHookFromForm(form url.Values) (application.SaveHookInput, error) {
	timeout, err := parseTimeout(form.Get(FieldTimeout))
	if err != nil {
		return application.SaveHookInput{}, err
	}
	env, err := parseEnv(form.Get(FieldEnv))
	if err != nil {
		return application.SaveHookInput{}, err
	}
	return application.SaveHookInput{
		ID:      strings.TrimSpace(form.Get(FieldID)),
		Name:    form.Get(FieldName),
		Kind:    models.Kind(form.Get(FieldKind)),
		Timeout: timeout,
		Script: models.ScriptSpec{
			Command:    form.Get(FieldCommand),
			Args:       lines(form.Get(FieldArgs)),
			Env:        env,
			UseShell:   form.Has(FieldUseShell),
			WorkingDir: form.Get(FieldWorkingDir),
		},
		Agent: models.AgentSpec{
			Prompt:   form.Get(FieldPrompt),
			Runtime:  form.Get(FieldRuntime),
			Model:    form.Get(FieldModel),
			Thinking: form.Get(FieldThinking),
			Profile:  form.Get(FieldProfile),
		},
	}, nil
}

// SaveAssignmentFromForm reads an assignment of the hook: a new one when the form has no id, the edit of a stored one
// otherwise. The project is the workspace the hook runs for.
func SaveAssignmentFromForm(form url.Values, hookID string) application.SaveAssignmentInput {
	return application.SaveAssignmentInput{
		ID:          strings.TrimSpace(form.Get(FieldID)),
		HookID:      hookID,
		WorkspaceID: strings.TrimSpace(form.Get(FieldProject)),
		Event:       models.Event(form.Get(FieldEvent)),
		Phase:       models.Phase(form.Get(FieldPhase)),
		Enabled:     form.Has(FieldEnabled),
		OnFailure:   models.FailurePolicy(form.Get(FieldOnFailure)),
	}
}

// TestHookFromForm reads the project, event and moment a test of the hook pretends to be.
func TestHookFromForm(form url.Values, hookID string) application.TestHookInput {
	return application.TestHookInput{
		WorkspaceID: strings.TrimSpace(form.Get(FieldProject)),
		HookID:      hookID,
		Event:       models.Event(form.Get(FieldEvent)),
		Phase:       models.Phase(form.Get(FieldPhase)),
	}
}

// Group is the project, event and moment whose order the screen shows or sets.
type Group struct {
	Project string
	Event   models.Event
	Phase   models.Phase
}

// GroupFromForm reads the group of a form or of a query. Unknown values are kept as typed: the use cases reject them.
func GroupFromForm(values url.Values) Group {
	return Group{
		Project: strings.TrimSpace(values.Get(FieldProject)),
		Event:   models.Event(values.Get(FieldEvent)),
		Phase:   models.Phase(values.Get(FieldPhase)),
	}
}

// ReorderFromForm reads the whole order of a group. The use case checks that it names each assignment once.
func ReorderFromForm(values url.Values) []string {
	var ids []string
	for _, id := range strings.Split(values.Get(FieldOrder), ",") {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

// History is the filter of the run history. Applied says whether the filter came from a form: without it the
// failed-only filter is on, as the screen starts.
type History struct {
	HookID    string
	ProjectID string
	Failed    bool
	Applied   bool
}

// HistoryFromQuery reads the filter of the history. Failed is on by default and only an applied filter can turn it off.
func HistoryFromQuery(values url.Values) History {
	if !values.Has(FieldApplied) {
		return History{Failed: true}
	}
	return History{
		HookID:    strings.TrimSpace(values.Get(FieldHook)),
		ProjectID: strings.TrimSpace(values.Get(FieldProject)),
		Failed:    values.Has(FieldFailed),
		Applied:   true,
	}
}

// RunFilter maps the history filter to the port filter, bounded by limit.
func (h History) RunFilter(limit int) outbound.RunFilter {
	return outbound.RunFilter{HookID: h.HookID, WorkspaceID: h.ProjectID, FailedOnly: h.Failed, Limit: limit}
}

// minTimeoutSeconds is the smallest timeout a hook can have: a zero or negative timeout would stop it before it starts.
const minTimeoutSeconds = 1

// maxTimeoutSeconds is the domain maximum in whole seconds. Both bounds are checked here, in seconds, so a huge number
// cannot overflow into a short timeout.
const maxTimeoutSeconds = int(models.MaxTimeout / time.Second)

// parseTimeout reads whole seconds between minTimeoutSeconds and maxTimeoutSeconds, both included.
func parseTimeout(text string) (time.Duration, error) {
	seconds, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("%w: the timeout must be a whole number of seconds", hookerrors.ErrInvalidHook)
	}
	if seconds < minTimeoutSeconds {
		return 0, fmt.Errorf("%w: the timeout must be at least %d second", hookerrors.ErrInvalidHook, minTimeoutSeconds)
	}
	if seconds > maxTimeoutSeconds {
		return 0, fmt.Errorf("%w: the timeout is too long", hookerrors.ErrInvalidHook)
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseEnv(text string) ([]models.EnvVar, error) {
	var env []models.EnvVar
	for _, line := range lines(text) {
		name, value, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("%w: the variable %q must be written as NAME=VALUE", hookerrors.ErrInvalidHook, line)
		}
		env = append(env, models.EnvVar{Name: name, Value: value})
	}
	return env, nil
}

func lines(text string) []string {
	var out []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}
