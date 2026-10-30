package state

import (
	"net/url"
)

// Tabs of the detail page of a task, in the order the strip shows them. Actions is the tab the page opens on.
const (
	TaskTabActions  = "actions"
	TaskTabFiles    = "files"
	TaskTabChanges  = "changes"
	TaskTabCodeHost = "code-host"
)

// ParseTaskTab reads the tab of a request. A tab that the strip does not show is refused, not served.
func ParseTaskTab(raw string) (string, bool) {
	switch raw {
	case TaskTabActions, TaskTabFiles, TaskTabChanges, TaskTabCodeHost:
		return raw, true
	default:
		return "", false
	}
}

// TaskTabs is the strip of the detail page of a task and the tab it shows. The strip and its body are one region,
// so each tab swaps the region for its own body from the same address shape.
type TaskTabs struct {
	Scope  string
	TaskID string
	Active string
}

// RegionID is the element that holds the strip and the body of the active tab.
func (t TaskTabs) RegionID() string {
	return Scoped("task-tabs", t.Scope)
}

// Path is the address that answers the region with the given tab active. It carries the scope of the instance, so the
// answer swaps the page that asked.
func (t TaskTabs) Path(tab string) string {
	path := "/tasks/" + t.TaskID + "/tabs/" + tab
	if t.Scope == "" {
		return path
	}
	values := url.Values{}
	values.Set("scope", t.Scope)
	return path + "?" + values.Encode()
}

// TaskPanelScope is the scope of a task worktree in the wire form the project views read: `task:<id>`.
func TaskPanelScope(taskID string) string {
	return "task:" + taskID
}
