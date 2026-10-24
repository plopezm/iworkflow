package models

import (
	"strings"
	"time"
)

// RemoteIssue is an issue as Jira reports it. Optional Jira fields are nil when Jira sends null.
type RemoteIssue struct {
	Key           string
	Summary       string
	Description   string
	Status        string
	IssueType     string
	Priority      *string
	Assignee      *string
	Reporter      *string
	Labels        []string
	URL           string
	JiraUpdatedAt time.Time
}

// Issue is an imported issue: the Jira data plus what iworkflow has done with it.
type Issue struct {
	Remote     RemoteIssue
	LocalState IssueLocalState
	// TaskID is the iworkflow task created from this issue; set only when LocalState is converted.
	TaskID   *string
	SyncedAt time.Time
}

// Key is the Jira issue key.
func (i Issue) Key() string {
	return i.Remote.Key
}

// IsValidIssueKey reports whether key looks like a Jira issue key (`PROJ-123`). Keys end up inside
// request paths, so anything else (slashes, dots, queries) must never reach the REST client.
func IsValidIssueKey(key string) bool {
	project, number, found := strings.Cut(key, "-")
	if !found || project == "" || !isASCIILetter(project[0]) {
		return false
	}
	for index := range len(project) {
		current := project[index]
		if !isASCIILetter(current) && !isASCIIDigit(current) && current != '_' {
			return false
		}
	}
	if number == "" {
		return false
	}
	for index := range len(number) {
		if !isASCIIDigit(number[index]) {
			return false
		}
	}
	return true
}

func isASCIILetter(value byte) bool {
	return (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
}

func isASCIIDigit(value byte) bool {
	return value >= '0' && value <= '9'
}
