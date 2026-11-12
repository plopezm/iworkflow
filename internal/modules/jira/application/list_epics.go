//nolint:wrapcheck // Port errors are returned unwrapped: their text is shown to the user.
package application

import (
	"context"
	"slices"

	"iworkflow/internal/modules/jira/domain/models"
	"iworkflow/internal/modules/jira/domain/ports/outbound"
)

// EpicListing is the imported issues grouped by Epic, plus what the filter UI needs to offer.
type EpicListing struct {
	Groups []models.EpicGroup
	// Statuses are the distinct Jira statuses among all imported issues, sorted.
	Statuses []string
	// Total is the number of imported issues before filtering.
	Total int
}

// ListEpics groups every imported issue by Epic. The filter narrows the children of each group;
// while a filter is active, groups left without children are hidden.
type ListEpics struct {
	issues outbound.IssueRepository
}

// NewListEpics builds the use case.
func NewListEpics(issues outbound.IssueRepository) *ListEpics {
	return &ListEpics{issues: issues}
}

// Execute lists the epic groups.
func (u *ListEpics) Execute(ctx context.Context, filter models.IssueFilter) (EpicListing, error) {
	all, err := u.issues.List(ctx)
	if err != nil {
		return EpicListing{}, err
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

	active := filter.Search != "" || filter.Status != nil || filter.LocalState != nil
	groups := make([]models.EpicGroup, 0)
	for _, group := range models.GroupByEpic(all) {
		group.Children = filter.Apply(group.Children)
		if active && len(group.Children) == 0 {
			continue
		}
		groups = append(groups, group)
	}
	return EpicListing{Groups: groups, Statuses: statuses, Total: len(all)}, nil
}
