package models

// SidebarLayout is how the sidebar arranges its workspaces: the pinned ones, the groups in order with the
// workspaces they hold, and the workspaces that belong to no group. Values are workspace ids. The tasks module
// only reads it; the workspace groups are owned by another module.
type SidebarLayout struct {
	Pinned    []string
	Groups    []LayoutGroup
	Ungrouped []string
}

// LayoutGroup is a collapsible group of the sidebar with the workspaces it holds, in the order they are listed.
type LayoutGroup struct {
	ID         string
	Name       string
	Color      string
	Collapsed  bool
	Workspaces []string
}
