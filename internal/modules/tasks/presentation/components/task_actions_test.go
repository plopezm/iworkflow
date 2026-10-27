package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// taskActionsView builds the actions tab of one task with the flags the caller sets, so the entry points the tab
// offers are the ones the task allows.
func taskActionsView(id string, canViewDiff, canMerge bool) state.TaskDetailView {
	return state.TaskDetailView{
		Scope: "abc123",
		Task: dto.TaskDTO{
			ID: id,
			Actions: dto.TaskActionsDTO{
				CanViewDiff: canViewDiff,
				CanMerge:    canMerge,
			},
		},
	}
}

// The diff of a task is opened in its own dock tab; the entry points at it and is offered only when the task can
// be viewed.
func TestTaskActionsShowsTheDiffEntryWhenCanViewDiff(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskActions(taskActionsView("t1", true, false)))

	assert.Contains(t, body, `href="/tasks/t1/diff"`, "the entry opens the dock tab of the diff of the task")
	assert.Contains(t, body, `data-dock-path="/tasks/t1/diff"`, "the entry opens the dock tab of the diff of the task")
	assert.Contains(t, body, ">View diff</a>", "the entry carries the label of the diff")
}

func TestTaskActionsHidesTheDiffEntryWhenCanViewDiffIsFalse(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskActions(taskActionsView("t1", false, false)))

	assert.NotContains(t, body, "/tasks/t1/diff", "no task without a diff to view offers it")
	assert.NotContains(t, body, "View diff", "the diff entry is not offered without the flag")
}

func TestTaskActionsShowsTheReviewSectionWhenOnlyTheTaskCanMerge(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskActions(taskActionsView("t1", false, true)))

	assert.Contains(t, body, "Review", "the review section shows when the task can be merged")
	assert.Contains(t, body, "Merge", "the merge action shows when the task can be merged")
	assert.NotContains(t, body, "/tasks/t1/diff", "the diff entry stays hidden without the flag")
}
