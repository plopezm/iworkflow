package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/presentation/state"
)

func TestFinishPreviewPathCarriesTheScopeOfThePage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/tasks/t1/finish/preview", state.FinishPreviewPath("t1", ""))
	assert.Equal(t, "/tasks/t1/finish/preview?scope=abc123", state.FinishPreviewPath("t1", "abc123"))
}

func TestFinishActionPathPostsToTheFinishOfThePage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "/tasks/t1/finish", state.FinishActionPath("t1", ""))
	assert.Equal(t, "/tasks/t1/finish?scope=abc123", state.FinishActionPath("t1", "abc123"))
}

func TestFinishSlotIDIsStablePerPageInstance(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "task-finish", state.FinishSlotID(""))
	assert.Equal(t, "task-finish-abc123", state.FinishSlotID("abc123"))
}
