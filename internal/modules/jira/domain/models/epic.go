package models

import "strings"

// epicIssueType is the issue type Jira gives to Epics.
const epicIssueType = "epic"

// NoEpicKey is the pseudo key of the group that holds the issues without an Epic.
const NoEpicKey = "none"

// doneStatusNames are the status names (lower case) that count as finished work. The imported data
// carries no status category, so the name is the only signal.
var doneStatusNames = map[string]struct{}{
	"done": {}, "closed": {}, "resolved": {}, "complete": {}, "completed": {},
	"hecho": {}, "cerrado": {}, "resuelto": {}, "finalizado": {}, "completado": {},
}

// IsDoneStatus reports whether a Jira status name means the work is finished.
func IsDoneStatus(status string) bool {
	_, done := doneStatusNames[strings.ToLower(strings.TrimSpace(status))]
	return done
}

// IsEpic reports whether the issue is itself an Epic.
func (i Issue) IsEpic() bool {
	return strings.EqualFold(strings.TrimSpace(i.Remote.IssueType), epicIssueType)
}

// belongsToEpic returns the key of the Epic the issue belongs to, or "" when it has none.
func (i Issue) belongsToEpic() string {
	if i.Remote.EpicKey == nil {
		return ""
	}
	return *i.Remote.EpicKey
}

// StatusCount is how many issues of a group are in one Jira status.
type StatusCount struct {
	Status string
	Count  int
}

// EpicGroup is an Epic with the issues that belong to it. The group without an Epic has the key
// NoEpicKey and a nil Epic.
type EpicGroup struct {
	Key string
	// Epic is the imported Epic issue; nil when only its key is known or for the group without Epic.
	Epic *Issue
	// Children are the issues of the group, in the order they were given.
	Children []Issue
}

// Done is how many children are finished.
func (g EpicGroup) Done() int {
	done := 0
	for _, child := range g.Children {
		if IsDoneStatus(child.Remote.Status) {
			done++
		}
	}
	return done
}

// StatusCounts lists the statuses of the children with their counts, in order of first appearance.
func (g EpicGroup) StatusCounts() []StatusCount {
	var counts []StatusCount
	index := make(map[string]int)
	for _, child := range g.Children {
		position, known := index[child.Remote.Status]
		if !known {
			position = len(counts)
			index[child.Remote.Status] = position
			counts = append(counts, StatusCount{Status: child.Remote.Status})
		}
		counts[position].Count++
	}
	return counts
}

// GroupByEpic groups issues by Epic. Epics themselves are group heads, never children. Groups keep
// the order in which their first issue appears (the input is newest first); the group of issues
// without an Epic comes last and only when it has issues.
func GroupByEpic(issues []Issue) []EpicGroup {
	imported := make(map[string]Issue)
	for _, issue := range issues {
		if issue.IsEpic() {
			imported[issue.Key()] = issue
		}
	}
	var groups []EpicGroup
	position := make(map[string]int)
	groupAt := func(key string) int {
		if at, known := position[key]; known {
			return at
		}
		group := EpicGroup{Key: key}
		if epic, found := imported[key]; found {
			copied := epic
			group.Epic = &copied
		}
		groups = append(groups, group)
		position[key] = len(groups) - 1
		return position[key]
	}
	var orphans []Issue
	for _, issue := range issues {
		switch epicKey := issue.belongsToEpic(); {
		case issue.IsEpic():
			groupAt(issue.Key())
		case epicKey != "":
			at := groupAt(epicKey)
			groups[at].Children = append(groups[at].Children, issue)
		default:
			orphans = append(orphans, issue)
		}
	}
	if len(orphans) > 0 {
		groups = append(groups, EpicGroup{Key: NoEpicKey, Children: orphans})
	}
	return groups
}
