package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func dashboardTask(id string, workspaceID string, stage string) dto.TaskDTO {
	return dto.TaskDTO{ID: id, WorkspaceID: workspaceID, Title: id, Stage: stage}
}

func TestBuildDashboardGroupsPendingAndRunningTasksPerProject(t *testing.T) {
	t.Parallel()
	workspaces := []dto.WorkspaceOptionDTO{{ID: "w1", Name: "alpha"}, {ID: "w2", Name: "beta"}, {ID: "w3", Name: "empty"}}
	tasks := []dto.TaskDTO{
		dashboardTask("a", "w1", state.StageBacklog),
		dashboardTask("b", "w1", state.StageWorking),
		dashboardTask("c", "w1", state.StageAttention),
		dashboardTask("d", "w2", state.StageBacklog),
		dashboardTask("e", "w2", state.StageReview),
		dashboardTask("f", "w2", state.StageDone),
		dashboardTask("g", "gone", state.StageWorking),
	}

	view := state.BuildDashboard(workspaces, tasks)

	assert.Len(t, view.Projects, 3)
	assert.Equal(t, "alpha", view.Projects[0].Name)
	assert.Len(t, view.Projects[0].Running, 2)
	assert.Len(t, view.Projects[0].Pending, 1)
	assert.Len(t, view.Projects[1].Running, 0)
	assert.Len(t, view.Projects[1].Pending, 1)
	assert.Empty(t, view.Projects[2].Running)
	assert.Empty(t, view.Projects[2].Pending)
	assert.Equal(t, 2, view.Running)
	assert.Equal(t, 2, view.Pending)
}

func TestBuildGraphWithoutWorkspaceIsGlobal(t *testing.T) {
	t.Parallel()
	tasks := []dto.TaskDTO{
		{ID: "a", WorkspaceID: "w1", Status: "backlog"},
		{ID: "b", WorkspaceID: "w2", Status: "backlog"},
	}
	graph := dto.TaskGraphDTO{Dependencies: []dto.TaskDependencyDTO{{PredecessorID: "a", SuccessorID: "b", Trigger: "merged"}}}

	view := state.BuildGraph("", tasks, graph, false, nil)

	assert.Len(t, view.Nodes, 2)
	assert.Len(t, view.Edges, 1)
	assert.Empty(t, state.LooseTasks("", tasks, graph, false, nil))
}
