package models

import (
	"fmt"
	"strings"
	"text/template"
)

// templateData is the set of fields a hook template may read, such as {{.Branch}}.
type templateData struct {
	TaskID, TaskTitle, TaskStatus, WorkspacePath, WorktreePath, Branch, Phase, Event, Result string
}

// ExpandTemplate expands a hook template (script argument or agent prompt) with the fields of the run. A
// missing field is an error, so a typo never silently expands to an empty value.
func ExpandTemplate(text string, run RunContext) (string, error) {
	tmpl, err := template.New("hook").Option("missingkey=error").Parse(text)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	data := templateData{
		run.TaskID, run.TaskTitle, run.TaskStatus, run.WorkspacePath, run.WorktreePath, run.Branch,
		string(run.Phase), string(run.Event), string(run.Result),
	}
	var out strings.Builder
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return out.String(), nil
}
