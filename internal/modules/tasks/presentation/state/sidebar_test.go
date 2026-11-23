package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/domain/models"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestBuildSidebarListsWorkspacesInTheGivenOrder(t *testing.T) {
	workspaces := []dto.WorkspaceOptionDTO{{ID: "w2", Name: "zeta", BaseBranch: "main"}, {ID: "w1", Name: "alpha", BaseBranch: "dev"}}

	rows := state.BuildSidebar(workspaces, nil)

	assert.Equal(t, []state.SidebarWorkspace{
		{ID: "w2", Name: "zeta", Subtitle: "main"},
		{ID: "w1", Name: "alpha", Subtitle: "dev"},
	}, rows.Workspaces)
	assert.Empty(t, rows.Attention)
}

func TestBuildSidebarAttentionHoldsOnlyTasksThatNeedTheUser(t *testing.T) {
	workspaces := []dto.WorkspaceOptionDTO{{ID: "w1", Name: "alpha"}}
	active := []dto.TaskDTO{
		{ID: "quiet", WorkspaceID: "w1", Title: "quiet", NeedsAttention: false, UpdatedAt: "2026-05-01T00:00:00+00:00"},
		{ID: "old", WorkspaceID: "w1", Title: "old", NeedsAttention: true, UpdatedAt: "2026-01-01T00:00:00+00:00"},
		{ID: "new", WorkspaceID: "w1", Title: "", Prompt: "fix it", NeedsAttention: true, UpdatedAt: "2026-02-01T00:00:00+00:00"},
	}

	rows := state.BuildSidebar(workspaces, active)

	assert.Equal(t, []state.SidebarTask{
		{ID: "new", Title: "fix it", Subtitle: "alpha"},
		{ID: "old", Title: "old", Subtitle: "alpha"},
	}, rows.Attention, "the most recent update comes first and a task without title shows its prompt")
}

func TestBuildSidebarCarriesTheWorkspaceColor(t *testing.T) {
	rows := state.BuildSidebar([]dto.WorkspaceOptionDTO{{ID: "w1", Name: "alpha", BaseBranch: "main", Color: "blue"}}, nil)

	assert.Equal(t, "blue", rows.Workspaces[0].Color)
}

func TestBuildSidebarStartsFlatWithEveryWorkspaceUngrouped(t *testing.T) {
	workspaces := []dto.WorkspaceOptionDTO{{ID: "w1", Name: "alpha"}, {ID: "w2", Name: "beta"}}

	rows := state.BuildSidebar(workspaces, nil)

	assert.Empty(t, rows.Pinned)
	assert.Empty(t, rows.Groups)
	assert.Equal(t, rows.Workspaces, rows.Ungrouped, "without a layout every workspace is listed in the flat list")
}

func TestBuildSidebarRecordsTheWorkspacesThatNeedAttention(t *testing.T) {
	active := []dto.TaskDTO{
		{ID: "t1", WorkspaceID: "w9", Title: "x", NeedsAttention: true, UpdatedAt: "2026-01-01T00:00:00+00:00"},
		{ID: "t2", WorkspaceID: "w8", Title: "y", NeedsAttention: false, UpdatedAt: "2026-01-01T00:00:00+00:00"},
	}

	rows := state.BuildSidebar(nil, active)

	assert.Equal(t, map[string]bool{"w9": true}, rows.NeedsAttentionIn)
}

func TestUngroupedRowsListEveryWorkspaceUnlessALayoutIsApplied(t *testing.T) {
	workspaces := []state.SidebarWorkspace{{ID: "w1", Name: "alpha"}, {ID: "w2", Name: "beta"}}

	assert.Equal(t, workspaces, state.SidebarRows{Workspaces: workspaces}.UngroupedRows(), "a row set that was never arranged is the flat list")
	arranged := state.SidebarRows{Workspaces: workspaces, Groups: []state.SidebarGroup{}, Ungrouped: []state.SidebarWorkspace{workspaces[1]}}
	assert.Equal(t, []state.SidebarWorkspace{workspaces[1]}, arranged.UngroupedRows())
}

func TestApplyLayoutRepartitionsTheSidebarRows(t *testing.T) {
	workspaces := []dto.WorkspaceOptionDTO{
		{ID: "w1", Name: "alpha"},
		{ID: "w2", Name: "beta"},
		{ID: "w3", Name: "gamma"},
		{ID: "w4", Name: "delta"},
	}
	ws := func(id string, name string) state.SidebarWorkspace {
		return state.SidebarWorkspace{ID: id, Name: name}
	}
	tests := []struct {
		name   string
		layout models.SidebarLayout
		active []dto.TaskDTO
		want   state.SidebarRows
	}{
		{
			name:   "a pinned workspace appears only in the pinned block, even if a group lists it too",
			layout: models.SidebarLayout{Pinned: []string{"w1"}, Groups: []models.LayoutGroup{{ID: "g1", Name: "Back", Workspaces: []string{"w1", "w2"}}}, Ungrouped: []string{"w3", "w4"}},
			want: state.SidebarRows{
				Pinned:    []state.SidebarWorkspace{{ID: "w1", Name: "alpha", Pinned: true}},
				Groups:    []state.SidebarGroup{{ID: "g1", Name: "Back", Workspaces: []state.SidebarWorkspace{{ID: "w2", Name: "beta", GroupID: "g1"}}}},
				Ungrouped: []state.SidebarWorkspace{ws("w3", "gamma"), ws("w4", "delta")},
			},
		},
		{
			name:   "groups keep the layout order and the workspaces keep the order inside each group",
			layout: models.SidebarLayout{Groups: []models.LayoutGroup{{ID: "g2", Name: "Second", Workspaces: []string{"w3", "w1"}}, {ID: "g1", Name: "First", Workspaces: []string{"w2"}}}, Ungrouped: []string{"w4"}},
			want: state.SidebarRows{
				Groups: []state.SidebarGroup{
					{ID: "g2", Name: "Second", Workspaces: []state.SidebarWorkspace{{ID: "w3", Name: "gamma", GroupID: "g2"}, {ID: "w1", Name: "alpha", GroupID: "g2"}}},
					{ID: "g1", Name: "First", Workspaces: []state.SidebarWorkspace{{ID: "w2", Name: "beta", GroupID: "g1"}}},
				},
				Ungrouped: []state.SidebarWorkspace{ws("w4", "delta")},
			},
		},
		{
			name:   "an empty group is kept so it can be filled",
			layout: models.SidebarLayout{Groups: []models.LayoutGroup{{ID: "g1", Name: "Empty", Color: "blue", Collapsed: true}}, Ungrouped: []string{"w1", "w2", "w3", "w4"}},
			want: state.SidebarRows{
				Groups:    []state.SidebarGroup{{ID: "g1", Name: "Empty", Color: "blue", Collapsed: true, Workspaces: []state.SidebarWorkspace{}}},
				Ungrouped: []state.SidebarWorkspace{ws("w1", "alpha"), ws("w2", "beta"), ws("w3", "gamma"), ws("w4", "delta")},
			},
		},
		{
			name:   "a workspace the layout does not mention is appended to the ungrouped ones, and unknown ids are ignored",
			layout: models.SidebarLayout{Groups: []models.LayoutGroup{{ID: "g1", Name: "Back", Workspaces: []string{"w2", "ghost"}}}, Ungrouped: []string{"w4"}},
			want: state.SidebarRows{
				Groups:    []state.SidebarGroup{{ID: "g1", Name: "Back", Workspaces: []state.SidebarWorkspace{{ID: "w2", Name: "beta", GroupID: "g1"}}}},
				Ungrouped: []state.SidebarWorkspace{ws("w4", "delta"), ws("w1", "alpha"), ws("w3", "gamma")},
			},
		},
		{
			name:   "a workspace listed twice is placed once, at its first place",
			layout: models.SidebarLayout{Pinned: []string{"w1", "w1"}, Groups: []models.LayoutGroup{{ID: "g1", Name: "Back", Workspaces: []string{"w2", "w2"}}}, Ungrouped: []string{"w3", "w3"}},
			want: state.SidebarRows{
				Pinned:    []state.SidebarWorkspace{{ID: "w1", Name: "alpha", Pinned: true}},
				Groups:    []state.SidebarGroup{{ID: "g1", Name: "Back", Workspaces: []state.SidebarWorkspace{{ID: "w2", Name: "beta", GroupID: "g1"}}}},
				Ungrouped: []state.SidebarWorkspace{ws("w3", "gamma"), ws("w4", "delta")},
			},
		},
		{
			name:   "a group needs attention only from its own workspaces",
			layout: models.SidebarLayout{Pinned: []string{"w1"}, Groups: []models.LayoutGroup{{ID: "g1", Name: "Back", Workspaces: []string{"w2"}}, {ID: "g2", Name: "Front", Workspaces: []string{"w3"}}}, Ungrouped: []string{"w4"}},
			active: []dto.TaskDTO{
				{ID: "pinned", WorkspaceID: "w1", Title: "p", NeedsAttention: true},
				{ID: "loose", WorkspaceID: "w4", Title: "l", NeedsAttention: true},
				{ID: "mine", WorkspaceID: "w2", Title: "m", NeedsAttention: true},
			},
			want: state.SidebarRows{
				Pinned: []state.SidebarWorkspace{{ID: "w1", Name: "alpha", Pinned: true}},
				Groups: []state.SidebarGroup{
					{ID: "g1", Name: "Back", NeedsAttention: true, Workspaces: []state.SidebarWorkspace{{ID: "w2", Name: "beta", GroupID: "g1"}}},
					{ID: "g2", Name: "Front", NeedsAttention: false, Workspaces: []state.SidebarWorkspace{{ID: "w3", Name: "gamma", GroupID: "g2"}}},
				},
				Ungrouped: []state.SidebarWorkspace{ws("w4", "delta")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := state.BuildSidebar(workspaces, tt.active)

			got := state.ApplyLayout(rows, tt.layout)

			assert.Equal(t, tt.want.Pinned, got.Pinned)
			assert.Equal(t, tt.want.Groups, got.Groups)
			assert.Equal(t, tt.want.Ungrouped, got.Ungrouped)
			assert.Equal(t, rows.Workspaces, got.Workspaces, "the flat list is never changed")
		})
	}
}
