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
	// Memory is the Engram chip of the row.
	Memory MemoryChip
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
	Error    string
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
	Table   TableView
}

// NewPage is the registration tab: the form and the folder picker.
type NewPage struct {
	Form RegisterForm
	// ListHref is where the tab goes on cancel and after a successful registration.
	ListHref string
}

// EditPage is the edit tab of one workspace.
type EditPage struct {
	ID           string
	Name         string
	Path         string
	BaseBranch   string
	AgentsFileID string
	Options      []AgentsOption
	// DefaultRuntime is the saved default agent; empty is the system agent, which is also what a saved
	// agent that no longer exists shows.
	DefaultRuntime string
	// Runtimes are the agents that can be chosen as the default.
	Runtimes []AgentsOption
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
