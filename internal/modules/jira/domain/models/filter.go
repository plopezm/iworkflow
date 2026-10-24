package models

import "strings"

// IssueFilter holds the criteria of the issue list; empty fields do not filter.
type IssueFilter struct {
	// Search is matched case-insensitively against key, summary, assignee and labels.
	Search string
	// Status is an exact Jira status name.
	Status     *string
	LocalState *IssueLocalState
}

// Matches reports whether the issue passes every criterion.
func (f IssueFilter) Matches(issue Issue) bool {
	if f.LocalState != nil && issue.LocalState != *f.LocalState {
		return false
	}
	if f.Status != nil && issue.Remote.Status != *f.Status {
		return false
	}
	needle := strings.ToLower(strings.TrimSpace(f.Search))
	if needle == "" {
		return true
	}
	remote := issue.Remote
	if strings.Contains(strings.ToLower(remote.Key), needle) ||
		strings.Contains(strings.ToLower(remote.Summary), needle) {
		return true
	}
	if remote.Assignee != nil && strings.Contains(strings.ToLower(*remote.Assignee), needle) {
		return true
	}
	for _, label := range remote.Labels {
		if strings.Contains(strings.ToLower(label), needle) {
			return true
		}
	}
	return false
}

// Apply keeps the issues that match, in their original order.
func (f IssueFilter) Apply(issues []Issue) []Issue {
	kept := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if f.Matches(issue) {
			kept = append(kept, issue)
		}
	}
	return kept
}
