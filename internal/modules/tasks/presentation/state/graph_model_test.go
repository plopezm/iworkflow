package state_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func taskModel(id, workspaceID, status, createdAt string) dto.TaskDTO {
	stage := "working"
	switch status {
	case "backlog":
		stage = "backlog"
	case "merged", "discarded":
		stage = "done"
	}
	return dto.TaskDTO{
		ID:          id,
		WorkspaceID: workspaceID,
		Title:       "tarea " + id,
		Prompt:      "prompt",
		Branch:      "pi/x",
		BaseBranch:  "main",
		Status:      status,
		Stage:       stage,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}
}

func dep(predecessor, successor string) dto.TaskDependencyDTO {
	return dto.TaskDependencyDTO{
		PredecessorID: predecessor,
		SuccessorID:   successor,
		Trigger:       "merged",
	}
}

func positioned(id string) dto.NodePositionDTO {
	return dto.NodePositionDTO{
		TaskID: id,
		X:      1.0,
		Y:      2.0,
	}
}

func TestEdgeStateFollowsThePredecessor(t *testing.T) {
	assert.Equal(t, state.EdgeStateDone, state.EdgeStateFor("merged"))
	assert.Equal(t, state.EdgeStatePending, state.EdgeStateFor("backlog"))
	assert.Equal(t, state.EdgeStatePending, state.EdgeStateFor("discarded"))
	assert.Equal(t, state.EdgeStateWorking, state.EdgeStateFor("running"))
	assert.Equal(t, state.EdgeStateWorking, state.EdgeStateFor("idle"))
}

func TestWaitingCountsUnmergedPredecessorsOfBacklogTasks(t *testing.T) {
	tasks := []dto.TaskDTO{
		taskModel("a", "w", "merged", "1"),
		taskModel("b", "w", "running", "2"),
		taskModel("c", "w", "backlog", "3"),
		taskModel("d", "w", "running", "4"),
	}
	deps := []dto.TaskDependencyDTO{dep("a", "c"), dep("b", "c"), dep("a", "d")}
	statuses := map[string]string{
		"a": "merged",
		"b": "running",
		"c": "backlog",
		"d": "running",
	}

	assert.Equal(t, 1, state.WaitingCount(tasks[2], deps, statuses))
	// Una tarea que ya arrancó no espera a nadie
	assert.Equal(t, 0, state.WaitingCount(tasks[3], deps, statuses))
	assert.Equal(t, 0, state.WaitingCount(tasks[0], deps, statuses))
}

func TestWaitingIgnoresPredecessorsWhoseStateIsUnknown(t *testing.T) {
	backlog := taskModel("c", "w", "backlog", "3")
	statuses := map[string]string{}
	assert.Equal(t, 0, state.WaitingCount(backlog, []dto.TaskDependencyDTO{dep("zz", "c")}, statuses))
}

func TestBuildGraphKeepsOneWorkspaceAndOrdersByAge(t *testing.T) {
	tasks := []dto.TaskDTO{
		taskModel("late", "w", "backlog", "3"),
		taskModel("early", "w", "running", "1"),
		taskModel("other", "x", "backlog", "2"),
	}
	graph := dto.TaskGraphDTO{
		Dependencies: []dto.TaskDependencyDTO{dep("early", "late"), dep("early", "other")},
		Positions:    nil,
	}
	view := state.BuildGraph("w", tasks, graph, true, nil)
	order := make([]string, len(view.Nodes))
	for i, n := range view.Nodes {
		order[i] = n.Task.ID
	}
	assert.Equal(t, []string{"early", "late"}, order)
	assert.False(t, view.Nodes[0].HasPredecessors)
	assert.False(t, view.Nodes[0].CanAddParallel())
	assert.True(t, view.Nodes[1].HasPredecessors)
	assert.True(t, view.Nodes[1].CanAddParallel())
	// La arista hacia otro workspace no se pinta
	assert.Len(t, view.Edges, 1)
	assert.Equal(t, "early", view.Edges[0].PredecessorID)
	assert.Equal(t, "late", view.Edges[0].SuccessorID)
}

func TestBuildGraphSkipsEdgesToHiddenCompletedTasks(t *testing.T) {
	tasks := []dto.TaskDTO{
		taskModel("a", "w", "merged", "1"),
		taskModel("b", "w", "backlog", "2"),
	}
	graph := dto.TaskGraphDTO{
		Dependencies: []dto.TaskDependencyDTO{dep("a", "b")},
		Positions:    nil,
	}
	shown := state.BuildGraph("w", tasks, graph, true, nil)
	assert.Len(t, shown.Nodes, 2)
	assert.Len(t, shown.Edges, 1)
	assert.Equal(t, state.EdgeStateDone, shown.Edges[0].State)

	hidden := state.BuildGraph("w", tasks, graph, false, nil)
	// 'b' solo estaba enlazada a la finalizada oculta: pasa a las sueltas
	assert.Empty(t, hidden.Nodes)
	assert.Empty(t, hidden.Edges)
}

func TestLooseTasksExcludesNodesAndOtherWorkspaces(t *testing.T) {
	tasks := []dto.TaskDTO{
		taskModel("loose-early", "w", "running", "1"),
		taskModel("chained", "w", "backlog", "2"),
		taskModel("head", "w", "running", "3"),
		taskModel("loose-done", "w", "merged", "4"),
		taskModel("loose-late", "w", "backlog", "5"),
		taskModel("placed", "w", "backlog", "6"),
		taskModel("elsewhere", "x", "backlog", "7"),
	}
	graph := dto.TaskGraphDTO{
		Dependencies: []dto.TaskDependencyDTO{dep("head", "chained")},
		Positions:    []dto.NodePositionDTO{positioned("placed")},
	}
	idsOf := func(list []dto.TaskDTO) []string {
		res := make([]string, len(list))
		for i, t := range list {
			res[i] = t.ID
		}
		return res
	}

	assert.Equal(t, []string{"loose-early", "loose-done", "loose-late"}, idsOf(state.LooseTasks("w", tasks, graph, true, nil)))
	assert.Equal(t, []string{"loose-early", "loose-late"}, idsOf(state.LooseTasks("w", tasks, graph, false, nil)))

	view := state.BuildGraph("w", tasks, graph, true, nil)
	nodes := make([]string, len(view.Nodes))
	for i, n := range view.Nodes {
		nodes[i] = n.Task.ID
	}
	assert.Equal(t, []string{"chained", "head", "placed"}, nodes)
}

func TestATaskLinkedOnlyToAHiddenCompletedOneIsLoose(t *testing.T) {
	tasks := []dto.TaskDTO{
		taskModel("a", "w", "merged", "1"),
		taskModel("b", "w", "backlog", "2"),
	}
	graph := dto.TaskGraphDTO{
		Dependencies: []dto.TaskDependencyDTO{dep("a", "b")},
		Positions:    nil,
	}
	partHidden := state.PartitionTasks("w", tasks, graph, false, nil)
	assert.Empty(t, partHidden.Nodes)
	assert.Len(t, partHidden.Loose, 1)
	assert.Equal(t, "b", partHidden.Loose[0].ID)

	// Con las finalizadas visibles la arista existe y ambas son nodos
	partShown := state.PartitionTasks("w", tasks, graph, true, nil)
	assert.Len(t, partShown.Nodes, 2)
	assert.Equal(t, "a", partShown.Nodes[0].ID)
	assert.Equal(t, "b", partShown.Nodes[1].ID)
	assert.Empty(t, partShown.Loose)
}

func TestADetachedTaskWithoutEdgesPositionOrOverrideIsLoose(t *testing.T) {
	tasks := []dto.TaskDTO{taskModel("a", "w", "backlog", "1")}
	part := state.PartitionTasks("w", tasks, dto.TaskGraphDTO{}, true, nil)
	assert.Empty(t, part.Nodes)
	assert.Len(t, part.Loose, 1)
	assert.Equal(t, "a", part.Loose[0].ID)
}

func TestATaskThatLostItsLastEdgeReturnsToTheLoosePanel(t *testing.T) {
	tasks := []dto.TaskDTO{
		taskModel("a", "w", "running", "1"),
		taskModel("b", "w", "backlog", "2"),
	}
	linked := dto.TaskGraphDTO{
		Dependencies: []dto.TaskDependencyDTO{dep("a", "b")},
	}
	partLinked := state.PartitionTasks("w", tasks, linked, true, nil)
	assert.Len(t, partLinked.Nodes, 2)

	unlinked := dto.TaskGraphDTO{}
	partUnlinked := state.PartitionTasks("w", tasks, unlinked, true, nil)
	assert.Empty(t, partUnlinked.Nodes)
	assert.Len(t, partUnlinked.Loose, 2)
}

func TestDepthsTraverseTheEdges(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	edges := [][2]string{
		{"a", "b"},
		{"b", "c"},
		{"a", "d"},
	}
	depth := state.Depths(ids, edges)
	assert.Equal(t, 0, depth["a"])
	assert.Equal(t, 1, depth["b"])
	assert.Equal(t, 2, depth["c"])
	assert.Equal(t, 1, depth["d"])
}

func TestAutoLayoutPutsDepthInColumnsAndOrderInRows(t *testing.T) {
	nodes := []string{"a", "b", "c"}
	edges := [][2]string{{"a", "c"}}
	placed := state.ResolvePositions(nodes, edges, nil)
	assert.Equal(t, state.Point{X: 48.0, Y: 48.0}, placed["a"])
	assert.Equal(t, state.Point{X: 48.0, Y: 48.0 + state.NodeHeight + state.RowGap}, placed["b"])
	assert.Equal(t, state.Point{X: 48.0 + state.NodeWidth + state.ColumnGap, Y: 48.0}, placed["c"])
}

func TestAutoLayoutKeepsSavedPositionsAndDodgesThem(t *testing.T) {
	nodes := []string{"a", "b"}
	saved := map[string]state.Point{
		"a": {X: 500.0, Y: 500.0},
		"b": {X: 48.0, Y: 48.0},
	}
	placed := state.ResolvePositions(nodes, nil, saved)
	assert.Equal(t, state.Point{X: 500.0, Y: 500.0}, placed["a"])
	assert.Equal(t, state.Point{X: 48.0, Y: 48.0}, placed["b"])

	withNew := []string{"a", "b", "n"}
	placedNew := state.ResolvePositions(withNew, nil, saved)
	assert.Equal(t, state.Point{X: 500.0, Y: 500.0}, placedNew["a"])
	assert.Equal(t, state.Point{X: 48.0, Y: 48.0}, placedNew["b"])
	fresh := state.RectOfNode(placedNew["n"])
	assert.False(t, fresh.Intersects(state.RectOfNode(state.Point{X: 48.0, Y: 48.0})))
	assert.False(t, fresh.Intersects(state.RectOfNode(state.Point{X: 500.0, Y: 500.0})))
}

func TestSavedPositionsMapByTask(t *testing.T) {
	graph := dto.TaskGraphDTO{
		Positions: []dto.NodePositionDTO{
			{TaskID: "a", X: 1.0, Y: 2.0},
		},
	}
	saved := state.SavedPositions(graph)
	assert.Equal(t, state.Point{X: 1.0, Y: 2.0}, saved["a"])
}

func TestLinkPredecessorsAndEdgesForNewTask(t *testing.T) {
	deps := []dto.TaskDependencyDTO{
		dep("p1", "anchor"),
		dep("p2", "anchor"),
		dep("other", "someone_else"),
	}
	nextLink := state.NewTaskLink{
		Kind:   state.NewTaskLinkNext,
		Anchor: "anchor",
	}
	assert.Equal(t, []string{"anchor"}, state.LinkPredecessors(nextLink, deps))

	parallelLink := state.NewTaskLink{
		Kind:   state.NewTaskLinkParallel,
		Anchor: "anchor",
	}
	assert.Equal(t, []string{"p1", "p2"}, state.LinkPredecessors(parallelLink, deps))

	edges := state.EdgesForNewTask("new-id", []string{"p1", "p2"})
	assert.Equal(t, [][2]string{{"p1", "new-id"}, {"p2", "new-id"}}, edges)
}

func TestDefaultBoardTabPrefersLooseOnlyWhenTheGraphIsEmpty(t *testing.T) {
	assert.Equal(t, state.BoardTabLoose, state.DefaultBoardTab(0, 5))
	assert.Equal(t, state.BoardTabGraph, state.DefaultBoardTab(1, 5))
	assert.Equal(t, state.BoardTabGraph, state.DefaultBoardTab(0, 0))
	assert.Equal(t, state.BoardTabGraph, state.DefaultBoardTab(2, 0))
}
