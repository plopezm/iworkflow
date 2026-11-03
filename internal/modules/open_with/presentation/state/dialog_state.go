// Package state holds the view models of the open_with dialog.
package state

import "net/url"

// BasePath is the address prefix of the dialog endpoints.
const BasePath = "/open-with/tasks/"

// Row is one configured application of the list.
type Row struct {
	ID      string
	Name    string
	Path    string
	Args    int
	EnvVars int
}

// Entered keeps what the user typed in the add form, so a rejected form is not lost.
type Entered struct {
	Name string
	Path string
	Args string
	Env  string
}

// DialogView is the whole dialog: the list, the add form and the outcome of the last action.
type DialogView struct {
	TaskID  string
	Scope   string
	Rows    []Row
	Entered Entered
	// Adding shows the add form; it is collapsed behind the "Add new" entry otherwise.
	Adding  bool
	Started string
	Failure string
	// Picker is the file picker, nil while it is closed.
	Picker *Picker
}

// PickerEntry is one file or folder of the picker.
type PickerEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// Picker is the file picker: the folder shown, how to go up, the filter and the entries.
type Picker struct {
	Dir       string
	Parent    string
	Filter    string
	Entries   []PickerEntry
	Truncated bool
	Error     string
}

// FormID is the id of the add form; the picker requests include its values so that nothing typed is lost.
func FormID(scope string) string { return HostID(scope) + "-form" }

// PickerBodyID is the id of the entries of the picker, which the filter refreshes alone.
func PickerBodyID(scope string) string { return HostID(scope) + "-entries" }

// BrowsePath is the address of the picker for a task.
func BrowsePath(taskID string, scope string) string { return Path(taskID, scope, "/browse") }

// NativePath is the address of the native file dialog for a task.
func NativePath(taskID string, scope string) string { return Path(taskID, scope, "/native") }

// NewPath is the address of the dialog with the add form open.
func NewPath(taskID string, scope string) string { return Path(taskID, scope, "/new") }

// ShowPath is the address of the dialog with the add form collapsed.
func ShowPath(taskID string, scope string) string { return Path(taskID, scope, "") }

// ChoosePath is the address that closes the picker, optionally filling the path field.
func ChoosePath(taskID string, scope string) string { return Path(taskID, scope, "/choose") }

// HostID is the id of the element the dialog replaces; the instance scope keeps two open tabs apart.
func HostID(scope string) string {
	if scope == "" {
		return "task-open-with"
	}
	return "task-open-with-" + scope
}

// Path is the address of an endpoint of the dialog for a task, carrying the scope.
func Path(taskID string, scope string, rest string) string {
	path := BasePath + url.PathEscape(taskID) + rest
	if scope == "" {
		return path
	}
	return path + "?" + url.Values{"scope": {scope}}.Encode()
}
