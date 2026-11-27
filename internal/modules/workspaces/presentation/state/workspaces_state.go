// Package state holds the view-models of the workspaces screens. They are plain data built by the
// inbound handlers from use case results; the templ files render them and contain no logic.
package state

import (
	"encoding/json"
	"strings"
)

// Addresses of the workspaces tabs, shared by the dock routes, the handlers and the templates.
const (
	// ListPath is the workspaces list tab.
	ListPath = "/workspaces"
	// NewPath is the registration tab.
	NewPath = "/workspaces/new"
	// GroupsPath is the groups tab, served by the workspace_groups module.
	GroupsPath = "/workspaces/groups"
)

// Initialization reasons, as the template needs them to pick the message and the confirm label.
const (
	ReasonMissingFolder = "MissingFolder"
	ReasonNotAGitRepo   = "NotAGitRepository"
	ReasonNoCommits     = "NoCommits"
)

// MemoryChipState is what the Engram chip of a workspace row tells.
type MemoryChipState string

// Engram chip states, as the templates need them to pick the text and the style.
const (
	// MemoryChipAvailable means Engram has memories for the workspace.
	MemoryChipAvailable MemoryChipState = "available"
	// MemoryChipEmpty means Engram answered but the workspace has no memories yet.
	MemoryChipEmpty MemoryChipState = "empty"
	// MemoryChipUnavailable means Engram could not be read; the chip is muted, not an error.
	MemoryChipUnavailable MemoryChipState = "unavailable"
	// MemoryChipHidden means no Engram is wired: the row carries no chip at all.
	MemoryChipHidden MemoryChipState = "hidden"
)

// MemoryChip is the Engram summary shown on a workspace row. The texts are chosen by the components.
type MemoryChip struct {
	State MemoryChipState
	Count int
}

// WorkspaceRow is one registered workspace in the list.
type WorkspaceRow struct {
	ID         string
	Name       string
	Path       string
	BaseBranch string
	// Color is the palette key of the workspace; empty means none.
	Color string
	// BoardHref opens the task board of the workspace.
	BoardHref string
	// EditHref opens the edit tab of the workspace.
	EditHref string
	// FilesHref opens the file explorer of the workspace (served by the project_tree module).
	FilesHref string
	// MemoriesHref opens the read-only memories page of the workspace (served by the memories module).
	MemoriesHref string
	// TerminalHref opens the terminal tab of the workspace (served by the terminal module).
	TerminalHref string
	// BranchesHref opens the branch tab of the workspace (served by the workspace_branches module).
	BranchesHref string
	// Memory is the Engram chip of the row.
	Memory MemoryChip
	// GroupName and GroupColor describe the group of the workspace; GroupName is empty when it has none.
	GroupName  string
	GroupColor string
}

// AgentsOption is a stored AGENTS.md offered in a drop-down.
type AgentsOption struct {
	Value string
	Label string
}

// InitializationPrompt asks the user to confirm initializing a folder. Resubmit holds the values of
// the attempt that needs confirmation, so the confirm button sends the same registration again.
type InitializationPrompt struct {
	Path           string
	Reason         string
	Name           string
	BaseBranch     string
	AgentsFileID   string
	DefaultRuntime string
	Color          string
	GroupID        string
}

// ResubmitJSON encodes the values the confirm button sends, as the hx-vals attribute.
func (p InitializationPrompt) ResubmitJSON() string {
	values := map[string]string{
		"name":           p.Name,
		"path":           p.Path,
		"base_branch":    p.BaseBranch,
		"agents_file_id": p.AgentsFileID,
		"initialize":     "true",
	}
	if p.DefaultRuntime != "" {
		values["default_runtime"] = p.DefaultRuntime
	}
	if p.Color != "" {
		values["color"] = p.Color
	}
	if p.GroupID != "" {
		values["group_id"] = p.GroupID
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// RegisterForm is the registration form and the notices under it.
type RegisterForm struct {
	Name         string
	Path         string
	BaseBranch   string
	AgentsFileID string
	Options      []AgentsOption
	// DefaultRuntime is the chosen default agent; empty is the system agent.
	DefaultRuntime string
	// Runtimes are the agents that can be chosen as the default.
	Runtimes []AgentsOption
	// Color is the chosen palette key; empty means none.
	Color string
	// GroupID is the group chosen for the new workspace; the groups themselves load in the background (RegisterGroups).
	GroupID string
	Error   string
	// Prompt is set when the server asks for confirmation before initializing a folder.
	Prompt *InitializationPrompt
}

// TableView is the list of registered workspaces, with the message of the last delete when it failed.
type TableView struct {
	Rows  []WorkspaceRow
	Error string
	// OutOfBand asks htmx to swap the table wherever it is, as a side effect of another request.
	OutOfBand bool
}

// WorkspacesPage is the whole workspaces tab: the list and the way to the registration tab.
type WorkspacesPage struct {
	// NewHref opens the registration tab.
	NewHref string
	// GroupsHref opens the groups tab.
	GroupsHref string
	Table      TableView
}

// NewPage is the registration tab: the form and the folder picker.
type NewPage struct {
	Form RegisterForm
	// ListHref is where the tab goes on cancel and after a successful registration.
	ListHref string
}

// GroupOption is a group the workspace can be put in.
type GroupOption struct {
	ID   string
	Name string
}

// RegisterGroups is the group selector of the registration form, loaded after the form is shown: the groups the new
// workspace can be put in (empty hides the selector) and the chosen one.
type RegisterGroups struct {
	Options  []GroupOption
	Selected string
}

// EditGroups is the group selector of the edit tab, loaded in the background: the groups the workspace can be put in
// (empty hides the selector) and the one it is in.
type EditGroups struct {
	ID       string
	Options  []GroupOption
	Selected string
}

// Rules of a rejected path change. Each one names the rule of the domain that failed, so the screen shows its exact
// reason in the request language.
const (
	RulePathBlocked            = "path_blocked"
	RulePathNotAbsolute        = "path_not_absolute"
	RulePathNotFound           = "path_not_found"
	RuleNotAGitRepository      = "not_a_git_repository"
	RulePathNotRepositoryRoot  = "path_not_repository_root"
	RulePathNotSameProject     = "path_not_same_project"
	RulePathAlreadyRegistered  = "path_already_registered"
	RuleSameProjectUnconfirmed = "same_project_unconfirmed"
	RuleWorktreeRepair         = "worktree_repair"
)

// EditFailure is why the edit was rejected, or saved with a notice. Rule is empty for any other error, whose Message is
// shown as it is. Detail is the path or the git output that the rule names.
type EditFailure struct {
	Rule    string
	Detail  string
	Message string
}

// EditPage is the edit tab of one workspace.
type EditPage struct {
	ID   string
	Name string
	Path string
	// PathLocked disables the path field; PathLockReason is the exact reason shown under it (S2, S6).
	PathLocked     bool
	PathLockReason string
	BaseBranch     string
	AgentsFileID   string
	Options        []AgentsOption
	// DefaultRuntime is the saved default agent; empty is the system agent, which is also what a saved
	// agent that no longer exists shows.
	DefaultRuntime string
	// Runtimes are the agents that can be chosen as the default.
	Runtimes []AgentsOption
	// Color is the saved palette key; empty means none.
	Color string
	// OptionsError is shown when the AGENTS.md files cannot be listed; the form stays usable.
	OptionsError string
	Error        string
}

// DirectoryEntry is a sub-folder shown by the picker.
type DirectoryEntry struct {
	Name      string
	Path      string
	IsGitRepo bool
}

// DirectoryPicker is the folder picker. Entries are already filtered by Filter.
type DirectoryPicker struct {
	Open    bool
	Path    string
	Parent  string
	Filter  string
	Entries []DirectoryEntry
	Error   string
}

// FilterEntries keeps the entries whose name contains needle, ignoring case and surrounding spaces.
// An empty needle keeps everything.
func FilterEntries(entries []DirectoryEntry, needle string) []DirectoryEntry {
	needle = strings.ToLower(strings.TrimSpace(needle))
	filtered := make([]DirectoryEntry, 0, len(entries))
	for _, entry := range entries {
		if strings.Contains(strings.ToLower(entry.Name), needle) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}
