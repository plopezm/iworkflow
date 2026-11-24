// Package state holds the view data of the group screens: the list of groups and the page of one group.
package state

import "net/url"

// Addresses of the group screens, shared by the dock routes, the handlers and the templates.
const (
	// WorkspacesPath is the Workspaces tab, the parent of the group screens.
	WorkspacesPath = "/workspaces"
	// ListPath is the groups tab.
	ListPath = "/workspaces/groups"
	// ListFragmentPath serves the rows of the groups tab; the tab reloads them on board:refresh.
	ListFragmentPath = "/workspace-groups/list"
	// ListTargetID is the element of the groups tab that holds the rows.
	ListTargetID = "group-list"
	// MembersTargetID is the element of the group tab that holds its workspaces.
	MembersTargetID = "group-members"
)

// DetailPath is the tab of one group.
func DetailPath(groupID string) string { return ListPath + "/" + url.PathEscape(groupID) }

// MembersPath serves the workspaces of one group; the group tab reloads them on board:refresh.
func MembersPath(groupID string) string {
	return "/workspace-groups/" + url.PathEscape(groupID) + "/members"
}

// DeletePath deletes one group and sends its tab back to the list.
func DeletePath(groupID string) string {
	return "/workspace-groups/" + url.PathEscape(groupID) + "/delete-and-close"
}

// GroupRow is one group of the list.
type GroupRow struct {
	ID    string
	Name  string
	Color string
	// Count is the number of workspaces in the group.
	Count int
	// Href opens the tab of the group.
	Href string
}

// GroupList is the rows of the groups tab.
type GroupList struct {
	Rows []GroupRow
}

// GroupsPage is the groups tab.
type GroupsPage struct{}

// Member is a workspace of a group, which can be taken out of it.
type Member struct {
	ID    string
	Name  string
	Color string
}

// Members is the workspaces of one group.
type Members struct {
	GroupID string
	Items   []Member
}

// GroupPage is the tab of one group: its name and color, and its workspaces.
type GroupPage struct {
	ID    string
	Name  string
	Color string
}
