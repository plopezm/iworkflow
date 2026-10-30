//nolint:wrapcheck // Port errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"slices"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// LaneListing is one lane with its filtered issues.
type LaneListing struct {
	Lane models.Lane
	// Issues are the lane issues that match the filter.
	Issues []models.Issue
	// Total is the number of lane issues before filtering.
	Total int
}

// IssueListing is the lanes with their filtered issues, plus what the filter UI needs to offer.
type IssueListing struct {
	Lanes []LaneListing
	// Statuses are the distinct Jira statuses among all imported issues (not only the filtered ones),
	// sorted.
	Statuses []string
	// Total is the number of distinct imported issues before filtering.
	Total int
}

// ListIssues builds the Jira page: every lane with the issues it holds, filtered.
type ListIssues struct {
	issues outbound.IssueRepository
	lanes  outbound.LaneRepository
}

// NewListIssues builds the use case.
func NewListIssues(issues outbound.IssueRepository, lanes outbound.LaneRepository) *ListIssues {
	return &ListIssues{issues: issues, lanes: lanes}
}

// Execute lists the lanes. An issue can belong to several lanes, so it can appear in several of them.
func (u *ListIssues) Execute(ctx context.Context, filter models.IssueFilter) (IssueListing, error) {
	all, err := u.issues.List(ctx)
	if err != nil {
		return IssueListing{}, err
	}
	lanes, err := u.lanes.List(ctx)
	if err != nil {
		return IssueListing{}, err
	}
	memberships, err := u.issues.LaneMemberships(ctx)
	if err != nil {
		return IssueListing{}, err
	}
	statusSet := make(map[string]struct{}, len(all))
	for _, issue := range all {
		statusSet[issue.Remote.Status] = struct{}{}
	}
	statuses := make([]string, 0, len(statusSet))
	for status := range statusSet {
		statuses = append(statuses, status)
	}
	slices.Sort(statuses)

	listings := make([]LaneListing, 0, len(lanes))
	for _, lane := range lanes {
		keys := make(map[string]struct{}, len(memberships[lane.ID]))
		for _, key := range memberships[lane.ID] {
			keys[key] = struct{}{}
		}
		// all is already ordered (most recently updated first): the lane keeps that order.
		members := make([]models.Issue, 0, len(keys))
		for _, issue := range all {
			if _, member := keys[issue.Key()]; member {
				members = append(members, issue)
			}
		}
		listings = append(listings, LaneListing{Lane: lane, Issues: filter.Apply(members), Total: len(members)})
	}
	return IssueListing{Lanes: listings, Statuses: statuses, Total: len(all)}, nil
}
