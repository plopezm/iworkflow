package inbound

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/workspaces/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/workspaces/presentation/state"
)

func TestWorkspaceRowLinksItsBranchesTab(t *testing.T) {
	row := workspaceRow(dto.WorkspaceDTO{ID: "w1"}, state.MemoryChip{State: state.MemoryChipHidden})

	assert.Equal(t, "/workspaces/w1/branches", row.BranchesHref)
}
