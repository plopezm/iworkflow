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

// A task whose merge request is already merged on the provider only gets its worktree removed, so the action says so.
func TestTaskActionsLabelsTheMergeActionAsCleanUpWhenItOnlyCleansUp(t *testing.T) {
	t.Parallel()
	view := taskActionsView("t1", false, true)
	view.MergeCleanupOnly = true

	body := renderIn(t, i18n.En, components.TaskActions(view))

	assert.Contains(t, body, ">Clean up</button>")
	assert.NotContains(t, body, ">Merge</button>")
	assert.NotContains(t, body, "Merge the branch into the base")
}

func TestTaskActionsKeepsTheMergeLabelWhenItMerges(t *testing.T) {
	t.Parallel()

	body := renderIn(t, i18n.En, components.TaskActions(taskActionsView("t1", false, true)))

	assert.Contains(t, body, ">Merge</button>")
	assert.NotContains(t, body, "Clean up")
}

// The "Open with" entry sits next to the terminal and is offered only while the task has a worktree.
func TestTaskActionsOffersOpenWithBesideTheTerminalOnlyWithAWorktree(t *testing.T) {
	t.Parallel()

	view := taskActionsView("t1", false, false)
	without := renderIn(t, i18n.En, components.TaskActions(view))
	view.Task.Actions.CanOpenTerminal = true
	with := renderIn(t, i18n.En, components.TaskActions(view))

	assert.NotContains(t, without, "Open with")
	assert.Contains(t, with, `hx-get="/open-with/tasks/t1?scope=abc123"`)
	assert.Contains(t, with, `hx-target="#task-open-with-abc123"`)
	assert.Contains(t, with, `id="task-open-with-abc123"`)
	assert.Contains(t, with, "Open with")
}

// A task stuck in finishing offers the finish again and tells the user what pressing it does; any other status shows
// no such hint.
func TestTaskActionsExplainsTheRetryOfAStuckFinish(t *testing.T) {
	t.Parallel()

	view := taskActionsView("t1", false, false)
	view.Task.Status = "finishing"
	view.Task.Actions.CanFinish = true

	body := renderIn(t, i18n.Es, components.TaskActions(view))

	assert.Contains(t, body, "La finalización no se completó")
	assert.Contains(t, body, ">Finalizar</button>", "the finish stays offered so the user can retry")
}

func TestTaskActionsShowsNoFinishHintOutsideFinishing(t *testing.T) {
	t.Parallel()

	view := taskActionsView("t1", false, false)
	view.Task.Status = "idle"
	view.Task.Actions.CanFinish = true

	body := renderIn(t, i18n.Es, components.TaskActions(view))

	assert.NotContains(t, body, "La finalización no se completó")
}
