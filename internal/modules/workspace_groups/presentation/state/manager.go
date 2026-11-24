// Package state holds the view data of the group management card.
package state

// Paths of the group management card. The Workspaces page loads ManagerPath; its element id is ManagerTargetID.
const (
	ManagerPath     = "/workspace-groups/manage"
	ManagerTargetID = "group-manager"
)

// ManagerWorkspace is a workspace as the assignments list shows it.
type ManagerWorkspace struct {
	ID    string
	Name  string
	Color string
	// GroupName is the group the workspace is in now, when it is offered to another group; empty when it has none.
	GroupName string
}

// ManagerGroup is one group of the card. Members and Available are filled only for the open group.
type ManagerGroup struct {
	ID    string
	Name  string
	Color string
	// Count is the number of workspaces in the group.
	Count int
	Open  bool
	// Members are the workspaces in the group, which can be unassigned.
	Members []ManagerWorkspace
	// Available are the workspaces outside the group, which can be assigned to it.
	Available []ManagerWorkspace
}

// Manager is the group management card.
type Manager struct {
	Groups []ManagerGroup
	// OpenID is the group whose assignments are shown; empty when none is.
	OpenID string
}

// ManagerHref is the address of the card with the assignments of a group open ("" keeps them all closed).
func ManagerHref(openID string) string {
	if openID == "" {
		return ManagerPath
	}
	return ManagerPath + "?open=" + openID
}
