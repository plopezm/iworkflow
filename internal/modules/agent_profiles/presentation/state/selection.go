package state

import "slices"

// SelectionKind is the kind of profile action whose outcome may move the selection.
type SelectionKind uint8

// Selection kinds.
const (
	SelectionAdded SelectionKind = iota + 1
	SelectionRenamed
	SelectionDeleted
)

// SelectionEvent describes what an action did. For SelectionAdded, To is the new profile; for
// SelectionRenamed, From and To; for SelectionDeleted, From is the removed profile.
type SelectionEvent struct {
	Kind SelectionKind
	From string
	To   string
}

// ResolveSelected returns the profile to show: the selected one when it exists, else the active one,
// else the first one. It returns "" when there are no profiles.
func ResolveSelected(names []string, active string, selected string) string {
	if selected != "" && slices.Contains(names, selected) {
		return selected
	}
	if active != "" && slices.Contains(names, active) {
		return active
	}
	if len(names) > 0 {
		return names[0]
	}
	return ""
}

// NextSelection returns the selection after an action. current is the profile the user was looking
// at ("" when none was explicitly chosen); known are the profile names before the action, in order.
func NextSelection(current string, known []string, event SelectionEvent) string {
	switch event.Kind {
	case SelectionAdded:
		return event.To
	case SelectionRenamed:
		if current == "" || current == event.From {
			return event.To
		}
		return current
	case SelectionDeleted:
		if current != event.From {
			return current
		}
		return neighbourAfterDelete(known, event.From)
	default:
		return current
	}
}

// neighbourAfterDelete picks the profile that takes the place of the deleted one: the next one in
// the list, or the previous one when the deleted profile was the last. It returns "" when no other
// profile remains.
func neighbourAfterDelete(known []string, deleted string) string {
	index := slices.Index(known, deleted)
	if index < 0 {
		return ""
	}
	remaining := make([]string, 0, len(known))
	for _, name := range known {
		if name != deleted {
			remaining = append(remaining, name)
		}
	}
	if index < len(remaining) {
		return remaining[index]
	}
	if len(remaining) > 0 {
		return remaining[len(remaining)-1]
	}
	return ""
}
