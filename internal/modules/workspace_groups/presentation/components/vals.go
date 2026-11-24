package components

// unassignVals is the hx-vals that take a workspace out of its group: the group endpoint reads an empty group as none.
func unassignVals() string { return `{"group":""}` }
