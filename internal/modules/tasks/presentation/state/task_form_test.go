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
