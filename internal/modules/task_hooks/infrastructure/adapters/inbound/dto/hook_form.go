// Package dto maps the forms of the task_hooks editor to the use case inputs.
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
)

// Form field names of the hook editor. Checkboxes are present in the form only when checked.
const (
	FieldID        = "id"
	FieldEvent     = "event"
	FieldPhase     = "phase"
	FieldKind      = "kind"
	FieldEnabled   = "enabled"
	FieldOnFailure = "on_failure"
	FieldTimeout   = "timeout"
	FieldCommand   = "command"
	FieldArgs      = "args"
	FieldEnv       = "env"
	FieldUseShell  = "use_shell"
	FieldPrompt    = "prompt"
	FieldRuntime   = "runtime"
	FieldModel     = "model"
	FieldThinking  = "thinking"
	FieldProfile   = "profile"
	// FieldOrder carries the whole order of the workspace hooks as comma-separated ids.
	FieldOrder = "order"
)

// SaveHookFromForm reads the editor form of a hook of the workspace. Arguments and variables are one per line, blank
// lines are ignored, and a variable is written NAME=VALUE. The timeout is in whole seconds. Fields of the other kind
// are read too and dropped by the domain, so switching the kind keeps nothing it should not.
func SaveHookFromForm(form url.Values, workspaceID string) (application.SaveHookInput, error) {
	timeout, err := parseTimeout(form.Get(FieldTimeout))
	if err != nil {
		return application.SaveHookInput{}, err
	}
	env, err := parseEnv(form.Get(FieldEnv))
	if err != nil {
		return application.SaveHookInput{}, err
	}
	return application.SaveHookInput{
		ID:          strings.TrimSpace(form.Get(FieldID)),
		WorkspaceID: workspaceID,
		Phase:       models.Phase(form.Get(FieldPhase)),
		Event:       models.Event(form.Get(FieldEvent)),
		Kind:        models.Kind(form.Get(FieldKind)),
		Enabled:     form.Has(FieldEnabled),
		OnFailure:   models.FailurePolicy(form.Get(FieldOnFailure)),
		Timeout:     timeout,
		Script: models.ScriptSpec{
			Command:  form.Get(FieldCommand),
			Args:     lines(form.Get(FieldArgs)),
			Env:      env,
			UseShell: form.Has(FieldUseShell),
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

// ReorderFromForm reads the whole order of the workspace hooks. The use case checks that it names each hook once.
func ReorderFromForm(form url.Values) []string {
	var ids []string
	for _, id := range strings.Split(form.Get(FieldOrder), ",") {
		if trimmed := strings.TrimSpace(id); trimmed != "" {
			ids = append(ids, trimmed)
		}
	}
	return ids
}

// parseTimeout reads whole seconds. The bound is checked here so a huge number cannot overflow into a short timeout.
func parseTimeout(text string) (time.Duration, error) {
	seconds, err := strconv.Atoi(strings.TrimSpace(text))
	if err != nil {
		return 0, fmt.Errorf("%w: the timeout must be a whole number of seconds", hookerrors.ErrInvalidHook)
	}
	if seconds > int(models.MaxTimeout/time.Second) {
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
