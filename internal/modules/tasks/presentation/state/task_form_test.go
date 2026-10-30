package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestBranchChoicesKeepTheSelectedBranchWhenTheListingNoLongerHasIt(t *testing.T) {
	t.Parallel()
	listed := []dto.BranchOptionDTO{{Name: "main"}, {Name: "origin/dev", Remote: true}}

	choices := state.BranchChoices(listed, "feature-gone")

	assert.Equal(t, []state.BranchChoice{
		{Name: "main"},
		{Name: "origin/dev", Remote: true},
		{Name: "feature-gone"},
	}, choices)
}

func TestBranchChoicesAddNothingWhenTheSelectionIsListedOrEmpty(t *testing.T) {
	t.Parallel()
	listed := []dto.BranchOptionDTO{{Name: "main"}}

	assert.Len(t, state.BranchChoices(listed, "main"), 1)
	assert.Len(t, state.BranchChoices(listed, ""), 1)
}

func TestHasAdvancedLimitsOpensTheOptionsOnlyWhenALimitIsSet(t *testing.T) {
	t.Parallel()

	assert.False(t, state.HasAdvancedLimits("", ""))
	assert.True(t, state.HasAdvancedLimits("512", ""))
	assert.True(t, state.HasAdvancedLimits("", "150"))
}

func TestRelatedProjectChoicesListEveryWorkspaceButTheOwnAndMarkTheChosenOnes(t *testing.T) {
	t.Parallel()
	workspaces := []dto.WorkspaceOptionDTO{
		{ID: "own", Name: "app", Path: "/repos/app"},
		{ID: "api", Name: "api", Path: "/repos/api"},
		{ID: "web", Name: "web", Path: "/repos/web"},
	}

	choices := state.RelatedProjectChoices(workspaces, "own", []string{"web"})

	assert.Equal(t, []state.RelatedProjectChoice{
		{ID: "api", Name: "api", Path: "/repos/api"},
		{ID: "web", Name: "web", Path: "/repos/web", Selected: true},
	}, choices)
}

func TestRelatedProjectChoicesListEveryWorkspaceWhenTheOwnOneIsNotChosenYet(t *testing.T) {
	t.Parallel()
	workspaces := []dto.WorkspaceOptionDTO{{ID: "api", Name: "api"}, {ID: "web", Name: "web"}}

	choices := state.RelatedProjectChoices(workspaces, "", nil)

	assert.Len(t, choices, 2)
	assert.False(t, choices[0].Selected)
}

func TestRelatedProjectChoicesNeverMarkTheOwnWorkspaceAndListNothingWhenItIsTheOnlyOne(t *testing.T) {
	t.Parallel()
	workspaces := []dto.WorkspaceOptionDTO{{ID: "own", Name: "app"}}

	assert.Empty(t, state.RelatedProjectChoices(workspaces, "own", []string{"own"}))
}
