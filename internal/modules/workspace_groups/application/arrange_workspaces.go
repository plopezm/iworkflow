package application

import (
	"context"
	"fmt"

	"iworkflow/internal/modules/workspace_groups/domain/models"
	"iworkflow/internal/modules/workspace_groups/domain/ports/outbound"
)

// ArrangeWorkspaces splits the workspaces of the sidebar into pinned, grouped and ungrouped.
type ArrangeWorkspaces struct{ repo outbound.GroupRepository }

// NewArrangeWorkspaces builds the use case.
func NewArrangeWorkspaces(repo outbound.GroupRepository) *ArrangeWorkspaces {
	return &ArrangeWorkspaces{repo: repo}
}

// Execute arranges the given workspace ids. The pinned block follows the pin order the repository returns (not the
// order of the ids), each workspace once; a pin of a workspace that is not in the list is ignored. The groups and the
// ungrouped list keep the order of the ids. A pinned workspace appears only among the pinned ones; a workspace whose
// group no longer exists counts as ungrouped; empty groups are kept so they can be filled.
func (u *ArrangeWorkspaces) Execute(ctx context.Context, workspaceIDs []string) (models.Layout, error) {
	groups, err := u.repo.ListGroups(ctx)
	if err != nil {
		return models.Layout{}, fmt.Errorf("arrange workspaces: %w", err)
	}
	members, err := u.repo.Memberships(ctx)
	if err != nil {
		return models.Layout{}, fmt.Errorf("arrange workspaces: %w", err)
	}
	pins, err := u.repo.Pinned(ctx)
	if err != nil {
		return models.Layout{}, fmt.Errorf("arrange workspaces: %w", err)
	}
	present := make(map[string]bool, len(workspaceIDs))
	for _, id := range workspaceIDs {
		present[id] = true
	}
	layout := models.Layout{Pinned: []string{}, Groups: make([]models.GroupMembers, 0, len(groups)), Ungrouped: []string{}}
	pinned := make(map[string]bool, len(pins))
	for _, id := range pins {
		if present[id] && !pinned[id] {
			pinned[id] = true
			layout.Pinned = append(layout.Pinned, id)
		}
	}
	slot := make(map[string]int, len(groups))
	for i, group := range groups {
		slot[group.ID()] = i
		layout.Groups = append(layout.Groups, models.GroupMembers{Group: group, Workspaces: []string{}})
	}
	for _, id := range workspaceIDs {
		if pinned[id] {
			continue
		}
		if i, ok := slot[members[id]]; ok {
			layout.Groups[i].Workspaces = append(layout.Groups[i].Workspaces, id)
			continue
		}
		layout.Ungrouped = append(layout.Ungrouped, id)
	}
	return layout, nil
}
