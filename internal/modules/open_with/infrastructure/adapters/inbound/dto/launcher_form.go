// Package dto maps the open_with forms to the use case inputs.
package dto

import (
	"fmt"
	"net/url"
	"strings"

	"iworkflow/internal/modules/open_with/application"
	"iworkflow/internal/modules/open_with/domain/models"
)

// Form field names of the add form.
const (
	FieldName = "name"
	FieldPath = "path"
	FieldArgs = "args"
	FieldEnv  = "env"
)

// CreateLauncherFromForm reads the add form: arguments one per line and variables as NAME=VALUE per line. Blank
// lines are ignored. A variable line without "=" is rejected.
func CreateLauncherFromForm(form url.Values) (application.CreateLauncherInput, error) {
	input := application.CreateLauncherInput{Name: form.Get(FieldName), Path: form.Get(FieldPath)}
	input.Args = lines(form.Get(FieldArgs))
	for _, line := range lines(form.Get(FieldEnv)) {
		name, value, found := strings.Cut(line, "=")
		if !found {
			return input, fmt.Errorf("the variable %q must be written as NAME=VALUE", line)
		}
		input.Env = append(input.Env, models.EnvVar{Name: name, Value: value})
	}
	return input, nil
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
