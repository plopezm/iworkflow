//nolint:wrapcheck // Domain errors are returned unwrapped: their text is user-visible and must stay identical to the Rust version.
package models

import (
	"strings"
	"unicode/utf8"

	jiraerrors "iworkflow/internal/modules/jira/domain/errors"
)

// DefaultLaneID identifies the built-in lane with the issues assigned to the connected user.
const DefaultLaneID = "mine"

// MaxLaneNameChars is the longest accepted lane name, in characters.
const MaxLaneNameChars = 60

// Lane is a group of issues on the Jira page, fed by one JQL query. The default lane is built in: it
// cannot be edited or removed; custom lanes are created by the user.
type Lane struct {
	ID        string
	Name      string
	JQL       string
	IsDefault bool
	Collapsed bool
}

// LaneBatch is what Jira returned for one lane during a sync.
type LaneBatch struct {
	LaneID string
	Issues []RemoteIssue
}

// LaneDefinition is the validated name and query of a custom lane.
type LaneDefinition struct {
	name string
	jql  string
}

// NewLaneDefinition trims and validates the name and the query.
func NewLaneDefinition(name string, jql string) (LaneDefinition, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return LaneDefinition{}, jiraerrors.EmptyLaneName()
	}
	if utf8.RuneCountInString(trimmedName) > MaxLaneNameChars {
		return LaneDefinition{}, jiraerrors.LaneNameTooLong(MaxLaneNameChars)
	}
	trimmedJQL := strings.TrimSpace(jql)
	if trimmedJQL == "" {
		return LaneDefinition{}, jiraerrors.EmptyLaneJQL()
	}
	return LaneDefinition{name: trimmedName, jql: trimmedJQL}, nil
}

// Name is the trimmed lane name.
func (d LaneDefinition) Name() string { return d.name }

// JQL is the trimmed lane query.
func (d LaneDefinition) JQL() string { return d.jql }
