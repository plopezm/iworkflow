//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	werrors "iworkflow/internal/modules/tasks/domain/errors"
)

// TaskPriority says how urgent a task is. Higher priorities are listed above lower ones.
type TaskPriority string

// Priorities. The persisted value is the lowercase name.
const (
	PriorityLow    TaskPriority = "low"
	PriorityNormal TaskPriority = "normal"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

// AllTaskPriorities lists every priority from the lowest to the highest.
var AllTaskPriorities = [...]TaskPriority{PriorityLow, PriorityNormal, PriorityHigh, PriorityUrgent}

// DefaultTaskPriority is the priority of a task that does not state one.
const DefaultTaskPriority = PriorityNormal

// ParseTaskPriority converts a name to a priority; unknown names are InvalidPriority.
func ParseTaskPriority(value string) (TaskPriority, error) {
	for _, priority := range AllTaskPriorities {
		if string(priority) == value {
			return priority, nil
		}
	}
	return "", werrors.InvalidPriority(value)
}

// Rank is the sorting rank: the higher the rank, the more urgent the task.
func (p TaskPriority) Rank() uint8 {
	switch p {
	case PriorityLow:
		return 0
	case PriorityNormal:
		return 1
	case PriorityHigh:
		return 2
	default:
		return 3
	}
}

// String returns the persisted name.
func (p TaskPriority) String() string {
	return string(p)
}
