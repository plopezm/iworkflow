package state

import "net/url"

// Sub-tabs of the Changes tab of a task. Uncommitted is the one the tab opens on.
const (
	ChangesSubUncommitted = "uncommitted"
	ChangesSubCommitted   = "committed"
)

// ParseChangesSub reads the sub-tab of a request. A sub-tab that the strip does not show is refused, not served.
func ParseChangesSub(raw string) (string, bool) {
	switch raw {
	case ChangesSubUncommitted, ChangesSubCommitted:
		return raw, true
	default:
		return "", false
	}
}

// TaskChanges is the strip of the Changes tab of a task and the sub-tab it shows. Like TaskTabs, the strip and its body
// are one region, so each sub-tab swaps the region for its own body.
type TaskChanges struct {
	Scope  string
	TaskID string
	Active string
}

// RegionID is the element that holds the strip and the body of the active sub-tab.
func (c TaskChanges) RegionID() string {
	return Scoped("task-changes", c.Scope)
}

// Path is the address that answers the region with the given sub-tab active, carrying the scope of the instance.
func (c TaskChanges) Path(sub string) string {
	path := "/tasks/" + c.TaskID + "/changes/" + sub
	if c.Scope == "" {
		return path
	}
	return path + "?" + url.Values{"scope": {c.Scope}}.Encode()
}

// CommittedChanges is what the Committed sub-tab shows: the commits of the task branch against its base and the files
// they changed. Failure replaces both when the diff could not be read.
type CommittedChanges struct {
	TaskID    string
	Commits   []string
	Files     []FileDiff
	Truncated bool
	Failure   string
}

// NewCommittedChanges builds the view of a committed diff (pure).
func NewCommittedChanges(taskID string, commits []string, patch string, truncated bool) CommittedChanges {
	return CommittedChanges{TaskID: taskID, Commits: commits, Files: ParsePatch(patch), Truncated: truncated}
}

// Empty reports whether the branch has no commits against its base.
func (c CommittedChanges) Empty() bool {
	return len(c.Commits) == 0 && len(c.Files) == 0
}
