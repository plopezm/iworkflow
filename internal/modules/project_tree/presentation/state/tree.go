package state

import (
	"strconv"
	"strings"

	"iworkflow/internal/modules/project_tree/domain/models"
)

// TreeRow is one row of a project tree: a folder that expands in place, or a file that opens a tab.
type TreeRow struct {
	Name  string
	Path  string
	IsDir bool
	// Href opens the file in a tab; it is empty for folders.
	Href string
	// Style indents the row by its depth.
	Style string
	// StatusClass colors the row by its git status; empty when the path is clean.
	StatusClass string
}

// TreeView is one listing: the root of a project, or the children of one folder.
type TreeView struct {
	Scope models.ProjectScope
	// WorkspaceName is the title of the root listing.
	WorkspaceName string
	Rows          []TreeRow
	// Error replaces the rows when the listing failed.
	Error string
	// EmptyStyle indents the "empty folder" message of a folder's children.
	EmptyStyle string
}

// PaddingStyle is the left padding of a row at depth: 0.75rem plus 1.125rem per level (pure).
func PaddingStyle(depth int) string {
	rem := 0.75 + 1.125*float64(depth)
	return "padding-left: " + strconv.FormatFloat(rem, 'f', -1, 64) + "rem"
}

// DepthOf is the depth of a `/`-separated path: children of the root have depth 0 (pure).
func DepthOf(path string) int {
	return strings.Count(path, "/")
}

// NewTreeRows maps listed entries to rows, coloring each one by its status (pure).
func NewTreeRows(scope models.ProjectScope, entries []models.TreeEntry, statuses map[string]models.GitChangeKind) []TreeRow {
	rows := make([]TreeRow, 0, len(entries))
	for _, entry := range entries {
		kind, known := statuses[entry.Path]
		row := TreeRow{
			Name:        entry.Name,
			Path:        entry.Path,
			IsDir:       entry.Kind == models.TreeEntryDirectory,
			Style:       PaddingStyle(DepthOf(entry.Path)),
			StatusClass: StatusClass(kind, known),
		}
		if !row.IsDir {
			row.Href = FileHref(scope, entry.Path)
		}
		rows = append(rows, row)
	}
	return rows
}

// NewRootView builds the root listing of a project.
func NewRootView(scope models.ProjectScope, workspaceName string, entries []models.TreeEntry, statuses map[string]models.GitChangeKind) TreeView {
	return TreeView{Scope: scope, WorkspaceName: workspaceName, Rows: NewTreeRows(scope, entries, statuses)}
}

// NewChildrenView builds the children of folder. Its empty message is indented under the folder.
func NewChildrenView(scope models.ProjectScope, folder string, entries []models.TreeEntry, statuses map[string]models.GitChangeKind) TreeView {
	return TreeView{
		Scope:      scope,
		Rows:       NewTreeRows(scope, entries, statuses),
		EmptyStyle: PaddingStyle(DepthOf(folder) + 2),
	}
}

// TreeErrorView is the listing that shows a failure instead of rows.
func TreeErrorView(scope models.ProjectScope, message string) TreeView {
	return TreeView{Scope: scope, Error: message}
}
