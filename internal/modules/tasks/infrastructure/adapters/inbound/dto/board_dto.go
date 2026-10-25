package dto

import (
	"net/url"
	"strings"
)

// WorkspaceOptionDTO is a workspace the board can show, as the workspace switcher lists it.
type WorkspaceOptionDTO struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BaseBranch string `json:"base_branch"`
}

// CreateTaskFromForm reads the create form of the board. The checkbox sends "on" when checked. The optional
// fields that are left blank keep their domain defaults.
func CreateTaskFromForm(values url.Values) CreateTaskDTO {
	return CreateTaskDTO{
		WorkspaceID: strings.TrimSpace(values.Get("workspace_id")),
		Title:       strings.TrimSpace(values.Get("title")),
		Prompt:      values.Get("prompt"),
		Priority:    strings.TrimSpace(values.Get("priority")),
		StartNow:    values.Get("start_now") == "on",
	}
}
