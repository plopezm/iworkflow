package dto

import "net/url"

// RegisterWorkspaceFromForm reads the registration form. Absent fields are empty, which the domain
// reads as "not given"; values are passed through untouched, the domain normalizes them.
func RegisterWorkspaceFromForm(values url.Values) RegisterWorkspaceDTO {
	return RegisterWorkspaceDTO{
		Name:           values.Get("name"),
		Path:           values.Get("path"),
		BaseBranch:     formOptional(values, "base_branch"),
		Initialize:     values.Get("initialize") == "true",
		AgentsFileID:   formOptional(values, "agents_file_id"),
		DefaultRuntime: formOptional(values, "default_runtime"),
		Color:          formOptional(values, "color"),
	}
}

// UpdateWorkspaceFromForm reads the edit form. The path is not part of it: it cannot change. The default
// agent is only set when the field is in the form: the select is hidden when no agent is installed, and
// an absent field must keep the saved agent instead of clearing it.
func UpdateWorkspaceFromForm(values url.Values) UpdateWorkspaceDTO {
	dto := UpdateWorkspaceDTO{
		Name:         values.Get("name"),
		BaseBranch:   formOptional(values, "base_branch"),
		AgentsFileID: formOptional(values, "agents_file_id"),
	}
	if values.Has("color") {
		dto.Color = formOptional(values, "color")
	}
	if values.Has("default_runtime") {
		dto.DefaultRuntime = formOptional(values, "default_runtime")
	}
	return dto
}

// formOptional keeps the field present but possibly empty, so that the optional DTO fields are set
// the same way a JSON client would set them.
func formOptional(values url.Values, key string) *string {
	value := values.Get(key)
	return &value
}
