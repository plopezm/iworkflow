package models

// Layout is how the sidebar arranges the workspaces: the pinned ones, the groups in order with the
// workspaces they hold, and the workspaces that belong to no group. Values are workspace ids.
type Layout struct {
	Pinned    []string
	Groups    []GroupMembers
	Ungrouped []string
}

// GroupMembers is a group with the workspaces that belong to it, in the order they are listed.
type GroupMembers struct {
	Group      Group
	Workspaces []string
}
