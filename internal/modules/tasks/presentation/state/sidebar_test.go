package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
