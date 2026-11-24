package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/domain/ports/outbound"
)

// ListAssignments lists which workspaces belong to each group, for the management screen.
type ListAssignments struct{ repo outbound.GroupRepository }

// NewListAssignments builds the use case.
func NewListAssignments(repo outbound.GroupRepository) *ListAssignments {
	return &ListAssignments{repo: repo}
}

// Execute places the given workspace ids under their groups, keeping the order of the ids. Every group is listed,
// empty ones too. A workspace without group, or whose group no longer exists, is unassigned. Pins are ignored: a
// pinned workspace is still a member of its group.
func (u *ListAssignments) Execute(ctx context.Context, workspaceIDs []string) (models.Assignments, error) {
	groups, err := u.repo.ListGroups(ctx)
	if err != nil {
		return models.Assignments{}, fmt.Errorf("list assignments: %w", err)
	}
	members, err := u.repo.Memberships(ctx)
	if err != nil {
		return models.Assignments{}, fmt.Errorf("list assignments: %w", err)
	}
	view := models.Assignments{Groups: make([]models.GroupMembers, 0, len(groups)), Unassigned: []string{}}
	slot := make(map[string]int, len(groups))
	for i, group := range groups {
		slot[group.ID()] = i
		view.Groups = append(view.Groups, models.GroupMembers{Group: group, Workspaces: []string{}})
	}
	for _, id := range workspaceIDs {
		if i, ok := slot[members[id]]; ok {
			view.Groups[i].Workspaces = append(view.Groups[i].Workspaces, id)
			continue
		}
		view.Unassigned = append(view.Unassigned, id)
	}
	return view, nil
}
