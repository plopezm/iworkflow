package components_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/a-h/templ"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"iworkflow/internal/core/i18n"
	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
	"iworkflow/internal/modules/tasks/presentation/components"
	"iworkflow/internal/modules/tasks/presentation/state"
)

func renderComponent(t *testing.T, locale i18n.Locale, component templ.Component) string {
	t.Helper()
	ctx := i18n.WithState(context.Background(), i18n.State{Locale: locale, Chosen: true})
	var out bytes.Buffer
	require.NoError(t, component.Render(ctx, &out))
	return out.String()
}

func TestEdgeClassAndArrowMarkerOutput(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "graph-edge graph-edge-done", components.EdgeClass(state.EdgeStateDone, false))
	assert.Equal(t, "graph-edge graph-edge-done graph-edge-selected", components.EdgeClass(state.EdgeStateDone, true))
	assert.Equal(t, "graph-edge graph-edge-working", components.EdgeClass(state.EdgeStateWorking, false))
	assert.Equal(t, "graph-edge graph-edge-working graph-edge-selected", components.EdgeClass(state.EdgeStateWorking, true))
	assert.Equal(t, "graph-edge graph-edge-pending", components.EdgeClass(state.EdgeStatePending, false))
	assert.Equal(t, "graph-edge graph-edge-pending graph-edge-selected", components.EdgeClass(state.EdgeStatePending, true))
	// Unknown/fallback state yields pending
	assert.Equal(t, "graph-edge graph-edge-pending", components.EdgeClass(state.EdgeState("unknown"), false))

	assert.Equal(t, "url(#iwf-arrow-done-board)", components.ArrowMarker(state.EdgeStateDone, false, "board"))
	assert.Equal(t, "url(#iwf-arrow-working-board)", components.ArrowMarker(state.EdgeStateWorking, false, "board"))
	assert.Equal(t, "url(#iwf-arrow-pending-board)", components.ArrowMarker(state.EdgeStatePending, false, "board"))
	assert.Equal(t, "url(#iwf-arrow-active-board)", components.ArrowMarker(state.EdgeStateWorking, true, "board"))
	assert.Equal(t, "url(#iwf-arrow-active-tab_40_1)", components.ArrowMarker(state.EdgeStateDone, true, "tab_40_1"))
}

func TestMarkerScopeAndMarkerIDDeterminism(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "board", components.MarkerScope(""))
	assert.Equal(t, "board_2f_123", components.MarkerScope("board/123"))
	assert.Equal(t, "workspace_2d_1", components.MarkerScope("workspace-1"))
	assert.Equal(t, "my_2e_tab_3a_alpha", components.MarkerScope("my.tab:alpha"))

	// Repeated calls return identical output
	assert.Equal(t, components.MarkerScope("complex/path-1:b"), components.MarkerScope("complex/path-1:b"))

	assert.Equal(t, "iwf-arrow-done-board_2f_123", components.MarkerID("board_2f_123", "iwf-arrow-done"))
	assert.Equal(t, "iwf-arrow-active-scope-x", components.MarkerID("scope-x", "iwf-arrow-active"))
}

func TestWaitingLabelFormatting(t *testing.T) {
	t.Parallel()

	ctxEn := i18n.WithState(context.Background(), i18n.State{Locale: i18n.En, Chosen: true})
	ctxEs := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Es, Chosen: true})
	ctxZh := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Zh, Chosen: true})
	ctxDe := i18n.WithState(context.Background(), i18n.State{Locale: i18n.De, Chosen: true})
	ctxFr := i18n.WithState(context.Background(), i18n.State{Locale: i18n.Fr, Chosen: true})

	// 0 -> nil
	assert.Nil(t, components.WaitingLabel(ctxEn, 0))
	assert.Nil(t, components.WaitingLabel(ctxEs, 0))
	assert.Nil(t, components.WaitingLabel(ctxEn, -2))

	// 1 -> contains "1"
	label1En := components.WaitingLabel(ctxEn, 1)
	require.NotNil(t, label1En)
	assert.Equal(t, "Waiting for 1 predecessor", *label1En)

	label1Es := components.WaitingLabel(ctxEs, 1)
	require.NotNil(t, label1Es)
	assert.Equal(t, "Esperando 1 predecesora", *label1Es)

	label1Zh := components.WaitingLabel(ctxZh, 1)
	require.NotNil(t, label1Zh)
	assert.Equal(t, "等待 1 个前置任务", *label1Zh)

	label1De := components.WaitingLabel(ctxDe, 1)
	require.NotNil(t, label1De)
	assert.Equal(t, "Wartet auf 1 Vorgänger", *label1De)

	label1Fr := components.WaitingLabel(ctxFr, 1)
	require.NotNil(t, label1Fr)
	assert.Equal(t, "En attente de 1 prédécesseur", *label1Fr)

	// N -> contains "N"
	label3En := components.WaitingLabel(ctxEn, 3)
	require.NotNil(t, label3En)
	assert.Equal(t, "Waiting for 3 predecessors", *label3En)

	label3Es := components.WaitingLabel(ctxEs, 3)
	require.NotNil(t, label3Es)
	assert.Equal(t, "Esperando 3 predecesoras", *label3Es)

	label5Zh := components.WaitingLabel(ctxZh, 5)
	require.NotNil(t, label5Zh)
	assert.Equal(t, "等待 5 个前置任务", *label5Zh)
}

func TestRenderingGraphArrowDefs(t *testing.T) {
	t.Parallel()

	html := renderComponent(t, i18n.En, components.GraphArrowDefs("scope_abc"))

	assert.Contains(t, html, "<defs>")
	assert.Contains(t, html, `id="iwf-arrow-done-scope_abc"`)
	assert.Contains(t, html, `class="graph-arrow-done"`)
	assert.Contains(t, html, `id="iwf-arrow-working-scope_abc"`)
	assert.Contains(t, html, `class="graph-arrow-working"`)
	assert.Contains(t, html, `id="iwf-arrow-pending-scope_abc"`)
	assert.Contains(t, html, `class="graph-arrow-pending"`)
	assert.Contains(t, html, `id="iwf-arrow-active-scope_abc"`)
	assert.Contains(t, html, `class="graph-arrow-active"`)
	assert.Contains(t, html, `viewBox="0 0 10 10"`)
	assert.Contains(t, html, `d="M 0 1 L 10 5 L 0 9 z"`)
}

func TestRenderingGraphEdgeLine(t *testing.T) {
	t.Parallel()

	label := "optional-label"
	edge := state.EdgeView{
		PredecessorID: "task-1",
		SuccessorID:   "task-2",
		State:         state.EdgeStateWorking,
		Label:         &label,
	}
	from := state.Point{X: 100, Y: 150}
	to := state.Point{X: 400, Y: 250}

	// Unselected edge
	htmlUnselected := renderComponent(t, i18n.En, components.GraphEdgeLine(
		edge,
		"board",
		from,
		to,
		false,
		nil,
	))
	assert.Contains(t, htmlUnselected, `class="graph-edge graph-edge-working"`)
	assert.Contains(t, htmlUnselected, `data-edge-key="task-1-&gt;task-2"`)
	assert.Contains(t, htmlUnselected, `class="graph-edge-hit"`)
	assert.Contains(t, htmlUnselected, `class="graph-edge-line"`)
	assert.Contains(t, htmlUnselected, `marker-end="url(#iwf-arrow-working-board)"`)
	assert.Contains(t, htmlUnselected, `class="graph-edge-label"`)
	assert.Contains(t, htmlUnselected, "optional-label")
	assert.NotContains(t, htmlUnselected, "graph-edge-delete")

	// Selected edge with delete attributes in Spanish
	deleteAttrs := templ.Attributes{
		"hx-delete": "/tasks/dependencies?predecessor=task-1&successor=task-2",
	}
	htmlSelected := renderComponent(t, i18n.Es, components.GraphEdgeLine(
		edge,
		"board",
		from,
		to,
		true,
		deleteAttrs,
	))
	assert.Contains(t, htmlSelected, `class="graph-edge graph-edge-working graph-edge-selected"`)
	assert.Contains(t, htmlSelected, `marker-end="url(#iwf-arrow-active-board)"`)
	assert.Contains(t, htmlSelected, `class="graph-edge-delete"`)
	assert.Contains(t, htmlSelected, `aria-label="Quitar dependencia"`)
	assert.Contains(t, htmlSelected, `<title>Quitar dependencia</title>`)
	assert.Contains(t, htmlSelected, `hx-delete="/tasks/dependencies?predecessor=task-1&amp;successor=task-2"`)
	assert.Contains(t, htmlSelected, `<circle r="12.0"`)
}

func TestRenderingLooseTasksPanel(t *testing.T) {
	t.Parallel()

	// Empty panel
	htmlEmpty := renderComponent(t, i18n.Es, components.LooseTasksPanel(nil, "sc1", false))
	assert.Contains(t, htmlEmpty, "Tareas sueltas")
	assert.Contains(t, htmlEmpty, "No hay tareas sueltas")
	assert.Contains(t, htmlEmpty, "Las tareas creadas con «Nueva tarea» aparecen aquí.")
	assert.Contains(t, htmlEmpty, `class="side-panel"`)

	// Non-empty panel with tasks
	errText := "Test error"
	tasks := []dto.TaskDTO{
		{
			ID:             "t-1",
			Title:          "First loose task",
			Stage:          "backlog",
			Priority:       "high",
			Error:          &errText,
			PendingDialogs: 1,
			NeedsAttention: true,
		},
		{
			ID:       "t-2",
			Title:    "Second loose task",
			Stage:    "working",
			Priority: "normal",
		},
	}
	htmlTasks := renderComponent(t, i18n.En, components.LooseTasksPanel(tasks, "sc1", false))
	assert.Contains(t, htmlTasks, "Loose tasks")
	assert.Contains(t, htmlTasks, "First loose task")
	assert.Contains(t, htmlTasks, "Second loose task")
	assert.Contains(t, htmlTasks, "loose-card-attention")
	assert.Contains(t, htmlTasks, "Test error")
	assert.Contains(t, htmlTasks, "1 dialog")
	assert.Contains(t, htmlTasks, "Add to graph")
	assert.NotContains(t, htmlTasks, "No loose tasks")

	// Collapsed panel
	htmlCollapsed := renderComponent(t, i18n.En, components.LooseTasksPanel(tasks, "sc1", true))
	assert.Contains(t, htmlCollapsed, "side-panel-collapsed")
	assert.Contains(t, htmlCollapsed, "hidden")
}

func TestTaskNodeAndCanvasRendering(t *testing.T) {
	t.Parallel()

	node := state.NodeView{
		Task: dto.TaskDTO{
			ID:       "task-10",
			Title:    "Feature implementation",
			Stage:    "backlog",
			Priority: "urgent",
		},
		Waiting: 2,
	}
	pos := state.Point{X: 120.5, Y: 80.0}

	htmlNode := renderComponent(t, i18n.Es, components.TaskNode(node, pos, "sc-board"))
	assert.Contains(t, htmlNode, "left: 120.5px; top: 80.0px; width: 264px; height: 140px;")
	assert.Contains(t, htmlNode, `data-node-id="task-10"`)
	assert.Contains(t, htmlNode, `class="graph-port graph-port-in"`)
	assert.Contains(t, htmlNode, `title="Entrada: suelta aquí una conexión"`)
	assert.Contains(t, htmlNode, `class="graph-port graph-port-out"`)
	assert.Contains(t, htmlNode, `title="Salida: arrastra a otra tarea para que espere a esta"`)
	assert.Contains(t, htmlNode, "Feature implementation")
	assert.Contains(t, htmlNode, "Esperando 2 predecesoras")
	assert.Contains(t, htmlNode, "Urgente")
	assert.Contains(t, htmlNode, "Siguiente")
	assert.Contains(t, htmlNode, "Paralela")
	assert.Contains(t, htmlNode, "Quitar del grafo")

	// Canvas rendering
	view := state.GraphView{
		Nodes: []state.NodeView{node},
		Edges: []state.EdgeView{
			{
				PredecessorID: "task-10",
				SuccessorID:   "task-10",
				State:         state.EdgeStatePending,
			},
		},
	}
	positions := map[string]state.Point{
		"task-10": pos,
	}
	viewport := state.Viewport{X: 10.0, Y: -5.0, Zoom: 0.75}

	htmlCanvas := renderComponent(t, i18n.En, components.TaskGraphCanvas(view, positions, viewport, "sc-board", ""))
	assert.Contains(t, htmlCanvas, components.GridStyle(viewport))
	assert.Contains(t, htmlCanvas, "background-size: 18.00px 18.00px; background-position: 10.00px -5.00px;")
	assert.Contains(t, htmlCanvas, "graph-edges")
	assert.Contains(t, htmlCanvas, "graph-nodes")
	assert.Contains(t, htmlCanvas, "zoom-controls")
	assert.Contains(t, htmlCanvas, "zoom-fit-btn")
}

func TestTaskNodeOmitsParallelActionForFirstTask(t *testing.T) {
	t.Parallel()
	node := state.NodeView{
		Task: dto.TaskDTO{
			ID:    "task-first",
			Title: "First task",
			Stage: "backlog",
		},
		Waiting: 0,
	}
	pos := state.Point{X: 0, Y: 0}

	htmlNode := renderComponent(t, i18n.Es, components.TaskNode(node, pos, "sc-board"))
	assert.Contains(t, htmlNode, "Siguiente")
	assert.NotContains(t, htmlNode, "Paralela")
	assert.Contains(t, htmlNode, "Quitar del grafo")
}

func TestTaskNodeIncludesParallelActionWhenHasPredecessors(t *testing.T) {
	t.Parallel()
	node := state.NodeView{
		Task: dto.TaskDTO{
			ID:    "task-second",
			Title: "Second task",
			Stage: "backlog",
		},
		Waiting:         0,
		HasPredecessors: true,
	}
	pos := state.Point{X: 0, Y: 0}

	htmlNode := renderComponent(t, i18n.Es, components.TaskNode(node, pos, "sc-board"))
	assert.Contains(t, htmlNode, "Siguiente")
	assert.Contains(t, htmlNode, "Paralela")
	assert.Contains(t, htmlNode, "Quitar del grafo")
}

func TestNodeStyleAndClass(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "left: 10.0px; top: 20.5px; width: 264px; height: 140px;", components.NodeStyle(state.Point{X: 10.0, Y: 20.5}))

	assert.Equal(t, "graph-node group", components.NodeClass(false, false, false))
	assert.Equal(t, "graph-node group graph-node-selected", components.NodeClass(true, false, false))
	assert.Equal(t, "graph-node group graph-node-target", components.NodeClass(false, true, false))
	assert.Equal(t, "graph-node group graph-node-attention", components.NodeClass(false, false, true))
	assert.Equal(t, "graph-node group graph-node-selected graph-node-target graph-node-attention", components.NodeClass(true, true, true))
}

func TestGridStyle(t *testing.T) {
	t.Parallel()

	vp := state.Viewport{X: 10.0, Y: -4.5, Zoom: 0.5}
	assert.Equal(t, "background-size: 12.00px 12.00px; background-position: 10.00px -4.50px;", components.GridStyle(vp))
}

func TestLooseCardKeyAndGhostStyle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "left: 42px; top: 84px;", components.GhostStyle(state.Point{X: 42.1, Y: 84.4}))

	task := dto.TaskDTO{
		ID:             "t-99",
		Title:          "Task title",
		Stage:          "working",
		Status:         "running",
		Priority:       "normal",
		PendingDialogs: 2,
		NeedsAttention: false,
	}
	key := components.LooseCardKey(task)
	assert.Contains(t, key, "t-99")
	assert.Contains(t, key, "Task title")
	assert.Contains(t, key, "running")
}
