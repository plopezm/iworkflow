package state

import "iworkflow/internal/modules/project_tree/domain/models"

// TreePage is the tab of a project root: the listing and the links of the ⋯ menu.
type TreePage struct {
	View TreeView
	// Terminal opens a terminal in the root.
	Terminal string
	// Edit opens the workspace edit page; HasEdit is false for tasks.
	Edit    string
	HasEdit bool
}

// NewTreePage builds the tree tab of a scope.
func NewTreePage(scope models.ProjectScope, view TreeView) TreePage {
	edit, hasEdit := EditHref(scope)
	return TreePage{View: view, Terminal: TerminalHref(scope), Edit: edit, HasEdit: hasEdit}
}
