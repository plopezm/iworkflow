package models

import "strings"

// PriorityHint is the iworkflow priority suggested from the Jira priority name; the user can change it.
type PriorityHint string

const (
	PriorityHintLow    PriorityHint = "low"
	PriorityHintNormal PriorityHint = "normal"
	PriorityHintHigh   PriorityHint = "high"
	PriorityHintUrgent PriorityHint = "urgent"
)

// ParsePriorityHint maps a stored or submitted value; anything unknown is Normal.
func ParsePriorityHint(value string) PriorityHint {
	switch PriorityHint(value) {
	case PriorityHintLow, PriorityHintHigh, PriorityHintUrgent:
		return PriorityHint(value)
	default:
		return PriorityHintNormal
	}
}

// PriorityHintFromJira maps the default Jira priority names (Blocker, Critical, Highest, High, Major,
// Medium, Minor, Low, Lowest, Trivial) case-insensitively. An empty or unknown name is Normal.
func PriorityHintFromJira(name string) PriorityHint {
	lower := strings.ToLower(strings.TrimSpace(name))
	contains := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(lower, word) {
				return true
			}
		}
		return false
	}
	switch {
	case contains("blocker", "critical", "highest", "urgent"):
		return PriorityHintUrgent
	case contains("high", "major"):
		return PriorityHintHigh
	case contains("low", "minor", "trivial"):
		return PriorityHintLow
	default:
		return PriorityHintNormal
	}
}

// String returns the stored form of the priority.
func (p PriorityHint) String() string {
	return string(p)
}
