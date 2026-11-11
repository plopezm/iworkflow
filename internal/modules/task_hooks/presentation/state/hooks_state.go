// Package state holds the view models of the Hooks screen. The server builds them from the use cases; the components
// only render them.
package state

import (
	"net/url"
)

// Tab is the section of the Hooks screen that is shown.
type Tab string

// Values of Tab.
const (
	TabLibrary Tab = "library"
	TabHistory Tab = "history"
)

// Notice is the outcome of the last action of the screen, shown as a badge.
type Notice string

// Values of Notice.
const (
	NoticeNone              Notice = ""
	NoticeHookSaved         Notice = "hook_saved"
	NoticeHookUpdated       Notice = "hook_updated"
	NoticeHookDeleted       Notice = "hook_deleted"
	NoticeAssigned          Notice = "assigned"
	NoticeAssignmentUpdated Notice = "assignment_updated"
	NoticeAssignmentDeleted Notice = "assignment_deleted"
	NoticeOrdered           Notice = "ordered"
)

// Status values of a test run and of a run shown in the history. They are the statuses of the domain runs.
const (
	ResultOK      = "ok"
	ResultFailed  = "failed"
	ResultTimeout = "timeout"
	ResultError   = "error"
	ResultSkipped = "skipped"
	ResultRunning = "running"
)

// HostID is the id of the element the screen replaces on every action.
const HostID = "hooks-screen"

// Screen is the whole Hooks screen. Failure is the error of the last action or of a read; the screen keeps what it
// can show. Selected is the hook whose assignments are open, Group the order panel, History the run history.
type Screen struct {
	Tab      Tab
	Notice   Notice
	Failure  string
	Projects []Choice
	Hooks    []HookRow
	Editor   *HookForm
	Browser  *Browser
	Selected *HookDetail
	Group    *Group
	History  *History
}

// Choice is a value of a drop-down: a project or a hook, with the name shown.
type Choice struct {
	ID   string
	Name string
}

// HookRow is a hook of the library. LastError is nil when the hook never failed.
type HookRow struct {
	ID          string
	Name        string
	Kind        string
	Assignments int
	LastError   *LastError
	Selected    bool
}

// LastError is the latest failed run of a hook, with the address of its history.
type LastError struct {
	Status      string
	StartedAt   string
	HistoryPath string
}

// HookForm is the editor of a hook. Fields of the other kind are kept, so switching the kind loses nothing typed.
type HookForm struct {
	ID         string
	Name       string
	Kind       string
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

// Browser is the folder listing of the path field being browsed. Files says whether files can be chosen too.
type Browser struct {
	Field   string
	Path    string
	Parent  string
	Files   bool
	Error   string
	Entries []BrowserEntry
}

// BrowserEntry is one folder or file of a Browser.
type BrowserEntry struct {
	Name  string
	Path  string
	IsDir bool
}

// AssignmentForm is the form of an assignment: a new one, or the values of a stored one that a toggle posts back.
// Project, Event and Phase are the domain values. Test uses the same fields to choose what the test pretends to be.
type AssignmentForm struct {
	ID        string
	Project   string
	Event     string
	Phase     string
	OnFailure string
	Enabled   bool
}

// AssignmentRow is an assignment of the selected hook, with the name of its project.
type AssignmentRow struct {
	Form        AssignmentForm
	ProjectName string
	Position    int
}

// HookDetail is the assignment editor of the selected hook: its assignments, the form of a new one and the outcome of
// the last test.
type HookDetail struct {
	HookID      string
	Name        string
	Assignments []AssignmentRow
	Add         AssignmentForm
	Test        AssignmentForm
	Result      *TestResult
}

// TestResult is the outcome of a test of a hook, shown under its editor. ExitCode is -1 when the run has none.
type TestResult struct {
	Status   string
	ExitCode int
	Output   string
	Failure  string
}

// Group is the order panel: the assignments of one project, event and phase, in the order they run.
type Group struct {
	Project string
	Event   string
	Phase   string
	Rows    []GroupRow
}

// GroupRow is an assignment of a group. UpOrder and DownOrder are the whole order of the group with the row swapped
// with its neighbour; empty means the row cannot move that way.
type GroupRow struct {
	ID        string
	HookName  string
	Enabled   bool
	Policy    string
	UpOrder   string
	DownOrder string
}

// History is the run history: the filters that are applied and the runs, newest first.
type History struct {
	Failed    bool
	HookID    string
	ProjectID string
	Hooks     []Choice
	Projects  []Choice
	Runs      []RunRow
}

// RunRow is one run of the history. ExitCode is shown when HasExitCode is true (script hooks only).
type RunRow struct {
	ID          string
	HookName    string
	TaskID      string
	ProjectName string
	Status      string
	ExitCode    int
	HasExitCode bool
	Output      string
	StartedAt   string
}

// Path helpers. Every identifier is escaped, so a path always points at the value it was built from.

// ScreenPath is the address of the Hooks screen.
const ScreenPath = "/hooks"

// HistoryRoute is the address that loads the run history; its filters are its query.
const HistoryRoute = ScreenPath + "/history"

// Query keys of the path browser of the editor: the field being browsed, the folder to list and the path to pick.
const (
	BrowseField     = "browse"
	BrowsePathField = "browse_path"
	PickField       = "pick"
)

// LibraryPath is the library tab, without an open editor or selection.
func LibraryPath() string { return ScreenPath + "/library" }

// GroupPath shows the order panel of a project, event and phase.
func GroupPath() string { return ScreenPath + "/group" }

// EditorPath opens the editor of a hook (empty id: a new one).
func EditorPath() string { return ScreenPath + "/editor" }

// HookPath opens the screen with the assignments of a hook.
func HookPath(hookID string) string { return ScreenPath + "/" + url.PathEscape(hookID) }

// SavePath saves the editor form.
func SavePath() string { return ScreenPath }

// DeletePath deletes a hook.
func DeletePath(hookID string) string { return HookPath(hookID) + "/delete" }

// TestPath tests a hook for the project, event and phase of the test form.
func TestPath(hookID string) string { return HookPath(hookID) + "/test" }

// AssignmentsPath adds an assignment to a hook.
func AssignmentsPath(hookID string) string { return HookPath(hookID) + "/assignments" }

// AssignmentPath saves a stored assignment of a hook.
func AssignmentPath(hookID string, assignmentID string) string {
	return AssignmentsPath(hookID) + "/" + url.PathEscape(assignmentID)
}

// AssignmentDeletePath deletes a stored assignment of a hook.
func AssignmentDeletePath(hookID string, assignmentID string) string {
	return AssignmentPath(hookID, assignmentID) + "/delete"
}

// OrderPath sets the order of a group.
func OrderPath() string { return ScreenPath + "/order" }

// HistoryPath opens the run history with its filters. The applied marker tells a failed-only filter that is off
// (unchecked) from one that was never sent (the default, on).
func HistoryPath(hookID string, projectID string, failed bool) string {
	values := url.Values{"applied": {"1"}, "hook": {hookID}, "project": {projectID}}
	if failed {
		values.Set("failed", "1")
	}
	return HistoryRoute + "?" + values.Encode()
}
