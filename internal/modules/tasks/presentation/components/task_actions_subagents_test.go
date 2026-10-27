package components_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

// The subagents entry of the actions tab opens the panel of the task, and counts the subagents still working.
func TestTaskActionsOffersTheSubagentsWithTheActiveCount(t *testing.T) {
	t.Parallel()
	view := state.TaskDetailView{
		Scope:      "abc123",
		Task:       dto.TaskDTO{ID: "t1"},
		Transcript: state.Transcript{SubagentsActive: 2},
	}

	body := renderIn(t, i18n.En, components.TaskActions(view))

	assert.Contains(t, body, `hx-get="/tasks/t1/subagents?scope=abc123"`, "the entry asks for the panel of this task")
	assert.Contains(t, body, ">Subagents", "the entry has its label")
	assert.Contains(t, body, `badge-info">2<`, "two subagents are still working")
}

func TestTaskActionsHidesTheSubagentsCountWhenNoneIsWorking(t *testing.T) {
	t.Parallel()
	view := state.TaskDetailView{Scope: "abc123", Task: dto.TaskDTO{ID: "t1"}}

	body := renderIn(t, i18n.En, components.TaskActions(view))

	assert.Contains(t, body, `hx-get="/tasks/t1/subagents?scope=abc123"`)
	assert.NotContains(t, body, "badge-info", "no badge without active subagents")
}
