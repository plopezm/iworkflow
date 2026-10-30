// Package components holds the templ components of the code_hosting module. They render the props
// they receive and contain no logic: the state package builds the props, and every text in them is
// already translated.
package components

import "net/url"

// Route roots of the module. The templates post and get to these paths, and the handlers mount them.
const (
	settingsRoot = "/code-host/settings/"
	tasksRoot    = "/code-host/tasks/"
)

// SettingsPath is the endpoint that saves the settings of a provider (POST).
func SettingsPath(provider string) string {
	return settingsRoot + url.PathEscape(provider)
}

// SettingsTestPath is the endpoint that tests the saved connection of a provider (POST).
func SettingsTestPath(provider string) string {
	return SettingsPath(provider) + "/test"
}

// SettingsCardID is the root of the settings card of a provider. Saving swaps it.
func SettingsCardID(provider string) string {
	return "code-host-settings-" + provider
}

// TestResultID is the slot where the connection test writes its answer.
func TestResultID(provider string) string {
	return "code-host-test-" + provider
}

// MergeRequestPath is the endpoint of the merge request panel of a task: GET reads it (add
// `confirm=1` to show the merge confirmation) and POST creates the request.
func MergeRequestPath(taskID string) string {
	return tasksRoot + url.PathEscape(taskID) + "/merge-request"
}

// MergePath is the endpoint that merges the open request of a task (POST).
func MergePath(taskID string) string {
	return tasksRoot + url.PathEscape(taskID) + "/merge"
}

// PushPath is the endpoint that publishes the branch of a task to origin (POST).
func PushPath(taskID string) string {
	return tasksRoot + url.PathEscape(taskID) + "/push"
}

// MergeRequestPanelID is the root of the panel of a task. Every action swaps it.
func MergeRequestPanelID(taskID string) string {
	return "code-host-mr-" + taskID
}
