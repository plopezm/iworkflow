package components

import (
	coreui "iworkflow/internal/core/components"
	"iworkflow/internal/modules/agents_files/presentation/state"
)

// projectOptions converts the projects to the options of a select field.
func projectOptions(projects []state.ProjectOption) []coreui.FormOption {
	options := make([]coreui.FormOption, 0, len(projects))
	for _, project := range projects {
		options = append(options, coreui.FormOption{Value: project.ID, Label: project.Name})
	}
	return options
}

// applyButtonID and projectSelectID name the controls of one file, so the htmx targets stay in sync.
func applyButtonID(fileID string) string { return "apply-" + fileID }

func projectSelectID(fileID string) string { return "project-" + fileID }

func fileURL(fileID string) string { return "/agents-files/" + fileID }
