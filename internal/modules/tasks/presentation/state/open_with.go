package state

import "net/url"

// OpenWithHostID is the id of the element the "Open with" dialog of the actions tab replaces. The dialog belongs to
// the open_with module; the tab only knows its address and this host.
func OpenWithHostID(scope string) string {
	return Scoped("task-open-with", scope)
}

// OpenWithPath is the address of the "Open with" dialog of a task, with the page scope when it is set.
func OpenWithPath(taskID string, scope string) string {
	return ScopedPath("/open-with/tasks/"+url.PathEscape(taskID), scope)
}
