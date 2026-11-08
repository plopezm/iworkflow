// Package state holds the view models of the workspace Hooks tab. They carry values and closed keys only: the
// components turn the keys into the text of the language of the request.
package state

import "net/url"

// Notice is the outcome of the last action on the tab. Empty means there is nothing to report.
type Notice string

// Values of Notice.
const (
	NoticeNone      Notice = ""
	NoticeSaved     Notice = "saved"
	NoticeUpdated   Notice = "updated"
	NoticeDeleted   Notice = "deleted"
	NoticeReordered Notice = "reordered"
)

// Result statuses of a test run, as the components name them.
const (
	ResultOK      = "ok"
	ResultFailed  = "failed"
	ResultTimeout = "timeout"
	// ResultError means the hook could not even be attempted; the failure says why.
	ResultError = "error"
)

// HookRow is one hook of a group. UpOrder and DownOrder are the whole order of the workspace after moving the hook one
// place within its group; empty means it cannot move that way. A moved order is submitted in full.
type HookRow struct {
	ID        string
	Kind      string
	Enabled   bool
	OnFailure string
	// Timeout is the whole number of seconds.
	Timeout   string
	Summary   string
	UpOrder   string
	DownOrder string
	// Test is the outcome of the last test of this hook, nil when it was not tested.
	Test *TestResult
}

// TestResult is the outcome of a test run. ExitCode is -1 when the hook did not exit (an agent).
type TestResult struct {
	Status   string
	ExitCode int
	Output   string
	Failure  string
}

// PhaseGroup is the list of one phase of an event, with the address that opens the editor for a new hook there.
type PhaseGroup struct {
	Phase   string
	Rows    []HookRow
	NewPath string
}

// EventSection holds the two groups of one event: the hooks before it and the hooks after it.
type EventSection struct {
	Event  string
	Before PhaseGroup
	After  PhaseGroup
}

// HookForm is what the editor shows. An empty ID means a new hook. The values are kept as typed, so a rejected form
// is shown again with the same content.
type HookForm struct {
	ID         string
	Event      string
	Phase      string
	Kind       string
	Enabled    bool
	OnFailure  string
	Timeout    string
	Command    string
	Args       string
	Env        string
	UseShell   bool
	WorkingDir string
	Prompt     string
	Runtime    string
	Model      string
	Thinking   string
	Profile    string
}

// Query fields of the file and folder browser of the editor. The browser is the editor loaded again with them, so the
// form keeps what was typed.
const (
	// BrowseField names the field the browser fills: the command or the working directory. Present while it is open.
	BrowseField = "browse"
	// BrowsePathField is the folder the browser lists.
	BrowsePathField = "browse_path"
	// PickField is the chosen path; the editor is then loaded with it in the field named by BrowseField.
	PickField = "pick"
)

// BrowserEntry is one file or folder shown in the browser.
type BrowserEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// Browser is the open file and folder picker of the editor. Files is true when a file can be chosen (the command),
// false when only a folder can (the working directory).
type Browser struct {
	Field   string
	Path    string
	Parent  string
	Files   bool
	Entries []BrowserEntry
	Error   string
}

// View is the whole tab: the sections, the editor when it is open and the outcome of the last action.
type View struct {
	WorkspaceID string
	Sections    []EventSection
	Editor      *HookForm
	Browser     *Browser
	Notice      Notice
	Failure     string
}

// HostID is the id of the element the tab replaces; the workspace keeps two open tabs apart.
func HostID(workspaceID string) string { return "workspace-hooks-" + workspaceID }

// Path is the address of an endpoint of the tab of a workspace.
func Path(workspaceID string, rest string) string {
	return "/workspaces/" + url.PathEscape(workspaceID) + "/hooks" + rest
}

// PanelPath is the address of the tab without the editor, used to close it.
func PanelPath(workspaceID string) string { return Path(workspaceID, "/panel") }

// EditorPath is the address of the editor. The query carries the id of the hook to edit or the values of a form
// being changed, see the handlers.
func EditorPath(workspaceID string, query url.Values) string {
	path := Path(workspaceID, "/editor")
	if len(query) == 0 {
		return path
	}
	return path + "?" + query.Encode()
}

// EditPath is the address of the editor loaded with the stored values of one hook.
func EditPath(workspaceID string, hookID string) string {
	return EditorPath(workspaceID, url.Values{"hook": {hookID}})
}

// SavePath is the address that stores the editor form (create or edit).
func SavePath(workspaceID string) string { return Path(workspaceID, "") }

// ReorderPath is the address that stores the whole order of the hooks.
func ReorderPath(workspaceID string) string { return Path(workspaceID, "/reorder") }

// EnabledPath is the address that turns one hook on or off.
func EnabledPath(workspaceID string, hookID string) string {
	return Path(workspaceID, "/"+url.PathEscape(hookID)+"/enabled")
}

// DeletePath is the address that deletes one hook.
func DeletePath(workspaceID string, hookID string) string {
	return Path(workspaceID, "/"+url.PathEscape(hookID)+"/delete")
}

// TestPath is the address that runs one hook as a test.
func TestPath(workspaceID string, hookID string) string {
	return Path(workspaceID, "/"+url.PathEscape(hookID)+"/test")
}
