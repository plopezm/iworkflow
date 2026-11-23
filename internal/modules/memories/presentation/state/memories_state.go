// Package state holds the view-models of the memories pages. The handlers build them; the templates only print them.
package state

import "strconv"

// ListPathFor is the address of the memories page of a workspace.
func ListPathFor(workspaceID string) string { return "/workspaces/" + workspaceID + "/memories" }

// DetailPathFor is the address of one memory of a workspace.
func DetailPathFor(workspaceID string, memoryID int) string {
	return ListPathFor(workspaceID) + "/" + strconv.Itoa(memoryID)
}

// Availability is what the page shows instead of rows when there is nothing to list.
type Availability string

const (
	// Ready means there are rows to show.
	Ready Availability = "ready"
	// Unavailable means the memory provider is off or cannot be reached.
	Unavailable Availability = "unavailable"
	// Empty means the provider holds no memory for the workspace.
	Empty Availability = "empty"
)

// Row is one memory of the list, already formatted.
type Row struct {
	Type  string
	Title string
	Date  string
	Href  string
}

// ListPage is the memories page of a workspace.
type ListPage struct {
	WorkspaceName string
	// WorkspaceColor is the palette key of the workspace; empty means none.
	WorkspaceColor string
	BoardHref      string
	Availability   Availability
	Rows           []Row
}

// DetailPage is one memory with its content.
type DetailPage struct {
	WorkspaceName string
	// WorkspaceColor is the palette key of the workspace; empty means none.
	WorkspaceColor string
	ListHref       string
	Availability   Availability
	Type           string
	Title          string
	Date           string
	Content        string
}

// TaskPage is the memories page of a task: the memories written since its branch started and the earlier ones.
type TaskPage struct {
	TaskTitle     string
	Branch        string
	WorkspaceName string
	TaskHref      string
	ListHref      string
	Availability  Availability
	// Started is false while the task is in the backlog, so it has no branch yet.
	Started bool
	// BranchRows are the memories written since the task started; EarlierRows are the rest.
	BranchRows  []Row
	EarlierRows []Row
}
