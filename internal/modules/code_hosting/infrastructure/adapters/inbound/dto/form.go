package dto

import (
	"net/url"
)

// SaveSettingsFromForm reads the settings form of one provider. The token is always passed on: a blank
// one keeps the stored token, as the use case decides. An unchecked TLS box sends no value.
func SaveSettingsFromForm(provider string, form url.Values) SaveCodeHostSettingsDTO {
	token := form.Get("token")
	return SaveCodeHostSettingsDTO{
		Provider:    provider,
		BaseURL:     form.Get("base_url"),
		Token:       &token,
		InsecureTLS: form.Get("insecure_tls") == "on",
	}
}

// CreateMergeRequestFromForm reads the optional title of a merge request to create for a task.
func CreateMergeRequestFromForm(taskID string, form url.Values) CreateMergeRequestDTO {
	title := form.Get("title")
	return CreateMergeRequestDTO{TaskID: taskID, Title: &title}
}
