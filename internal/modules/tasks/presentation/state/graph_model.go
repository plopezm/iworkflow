package state

import (
	"fmt"
	"slices"

	"iworkflow/internal/modules/tasks/infrastructure/adapters/inbound/dto"
)

// Modelo del grafo de dependencias y particionado de tareas en el tablero:
// separa las tareas en nodos del lienzo y tareas sueltas, resuelve el orden topológico
// y calcula las posiciones automáticas. Sin dependencias de UI ni DOM.

const (
	// ColumnGap es la separación horizontal entre columnas del layout automático.
	ColumnGap = 96.0
	// RowGap es la separación vertical entre filas del layout automático.
	RowGap = 40.0
)

// LayoutOrigin es el origen de coordenadas del layout automático.
var LayoutOrigin = Point{X: 48.0, Y: 48.0}

// EdgeState es el estado visual de una arista según la tarea predecesora.
type EdgeState string

const (
	// EdgeStateDone indica que la predecesora ya está integrada: la arista se ha cumplido.
	EdgeStateDone EdgeState = "done"
	// EdgeStateWorking indica que la predecesora está en marcha: la arista «fluye».
	EdgeStateWorking EdgeState = "working"
	// EdgeStatePending indica que la predecesora aún no ha empezado (o fue descartada).
	EdgeStatePending EdgeState = "pending"
)

// EdgeStateFor calcula el estado de una arista a partir del estado de su predecesora.
func EdgeStateFor(predecessorStatus string) EdgeState {
	switch predecessorStatus {
	case "merged":
		return EdgeStateDone
	case "backlog", "discarded":
		return EdgeStatePending
	default:
		return EdgeStateWorking
	}
}

// EdgeView es una arista lista para pintar en el lienzo.
type EdgeView struct {
	PredecessorID string    `json:"predecessor_id"`
	SuccessorID   string    `json:"successor_id"`
	State         EdgeState `json:"state"`
	Trigger       string    `json:"trigger"`
	Label         *string   `json:"label,omitempty"`
}

// Key devuelve la clave única de la arista.
func (e EdgeView) Key() string {
	return EdgeKey(e.PredecessorID, e.SuccessorID)
}

// EdgeKey devuelve la clave única de una arista entre dos tareas.
func EdgeKey(predecessorID, successorID string) string {
	return predecessorID + "->" + successorID
}

// NodeView es un nodo listo para pintar: la tarea y cuántas predecesoras le faltan por integrarse.
type NodeView struct {
	Task dto.TaskDTO `json:"task"`
	// Waiting es el número de predecesoras aún no integradas; solo cuenta para tareas en Backlog.
	Waiting int `json:"waiting"`
	// HasPredecessors indica si la tarea tiene al menos una predecesora en el grafo.
	HasPredecessors bool `json:"has_predecessors"`
}

// CanAddParallel indica si la tarea puede tener una tarea añadida en paralelo (no es la primera en su flujo).
func (n NodeView) CanAddParallel() bool {
	return n.HasPredecessors || n.Waiting > 0
}

// GraphView es el grafo visible de un workspace.
type GraphView struct {
	Nodes []NodeView `json:"nodes"`
	Edges []EdgeView `json:"edges"`
}

// WaitingCount calcula cuántas predecesoras de task aún no están integradas. Una predecesora cuyo
// estado se desconoce no se cuenta.
func WaitingCount(task dto.TaskDTO, dependencies []dto.TaskDependencyDTO, statuses map[string]string) int {
	if task.Status != "backlog" {
		return 0
	}
	count := 0
	for _, d := range dependencies {
		if d.SuccessorID == task.ID {
			if status, ok := statuses[d.PredecessorID]; ok && status != "merged" {
				count++
			}
		}
	}
	return count
}

// BoardTab representa la subpestaña del tablero en pantallas estrechas.
type BoardTab string

const (
	BoardTabLoose BoardTab = "loose"
	BoardTabGraph BoardTab = "graph"
)

// DefaultBoardTab resuelve la subpestaña por defecto: el grafo, salvo que no tenga nodos y haya tareas sueltas.
func DefaultBoardTab(graphNodes int, loose int) BoardTab {
	if graphNodes == 0 && loose > 0 {
		return BoardTabLoose
	}
	return BoardTabGraph
}

// visibleTasks devuelve las tareas de workspaceID ordenadas de la más antigua a la más reciente.
func visibleTasks(workspaceID string, tasks []dto.TaskDTO, showDone bool) []dto.TaskDTO {
	visible := make([]dto.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		if t.WorkspaceID != workspaceID {
			continue
		}
		if !showDone && (t.Status == "merged" || t.Status == "discarded") {
			continue
		}
		visible = append(visible, t)
	}
	slices.SortStableFunc(visible, func(a, b dto.TaskDTO) int {
		if a.CreatedAt < b.CreatedAt {
			return -1
		}
		if a.CreatedAt > b.CreatedAt {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return visible
}

// BoardPartition es el reparto de las tareas visibles de un workspace entre el lienzo y el panel de sueltas.
type BoardPartition struct {
	Nodes []dto.TaskDTO
	Loose []dto.TaskDTO
}

// PartitionTasks reparte las tareas visibles de un workspace entre nodos del grafo y tareas sueltas.
// Es nodo si tiene una arista hacia otra tarea visible, una posición guardada o una posición recién fijada (placed).
func PartitionTasks(
	workspaceID string,
	tasks []dto.TaskDTO,
	graph dto.TaskGraphDTO,
	showDone bool,
	placed map[string]struct{},
) BoardPartition {
	visible := visibleTasks(workspaceID, tasks, showDone)
	ids := make(map[string]struct{}, len(visible))
	for _, t := range visible {
		ids[t.ID] = struct{}{}
	}

	linked := make(map[string]struct{})
	for _, d := range graph.Dependencies {
		_, predOk := ids[d.PredecessorID]
		_, succOk := ids[d.SuccessorID]
		if predOk && succOk {
			linked[d.PredecessorID] = struct{}{}
			linked[d.SuccessorID] = struct{}{}
		}
	}

	positioned := make(map[string]struct{}, len(graph.Positions))
	for _, p := range graph.Positions {
		positioned[p.TaskID] = struct{}{}
	}

	nodes := make([]dto.TaskDTO, 0, len(visible))
	loose := make([]dto.TaskDTO, 0, len(visible))

	for _, t := range visible {
		_, isLinked := linked[t.ID]
		_, isPositioned := positioned[t.ID]
		_, isPlaced := placed[t.ID]
		if isLinked || isPositioned || isPlaced {
			nodes = append(nodes, t)
		} else {
			loose = append(loose, t)
		}
	}

	return BoardPartition{Nodes: nodes, Loose: loose}
}

// LooseTasks devuelve las tareas sueltas visibles de workspaceID: las que muestra el panel izquierdo
// y que no se pintan en el lienzo.
func LooseTasks(
	workspaceID string,
	tasks []dto.TaskDTO,
	graph dto.TaskGraphDTO,
	showDone bool,
	placed map[string]struct{},
) []dto.TaskDTO {
	return PartitionTasks(workspaceID, tasks, graph, showDone, placed).Loose
}

// BuildGraph construye el grafo visible para un workspace con sus nodos y aristas preparadas.
func BuildGraph(
	workspaceID string,
	tasks []dto.TaskDTO,
	graph dto.TaskGraphDTO,
	showDone bool,
	placed map[string]struct{},
) GraphView {
	partition := PartitionTasks(workspaceID, tasks, graph, showDone, placed)
	statuses := make(map[string]string, len(tasks))
	for _, t := range tasks {
		statuses[t.ID] = t.Status
	}

	visibleIDs := make(map[string]struct{}, len(partition.Nodes))
	for _, t := range partition.Nodes {
		visibleIDs[t.ID] = struct{}{}
	}

	edges := make([]EdgeView, 0, len(graph.Dependencies))
	for _, d := range graph.Dependencies {
		_, predOk := visibleIDs[d.PredecessorID]
		_, succOk := visibleIDs[d.SuccessorID]
		if predOk && succOk {
			status, hasStatus := statuses[d.PredecessorID]
			if hasStatus {
				edges = append(edges, EdgeView{
					PredecessorID: d.PredecessorID,
					SuccessorID:   d.SuccessorID,
					State:         EdgeStateFor(status),
					Trigger:       d.Trigger,
				})
			}
		}
	}

	predecessorsCount := make(map[string]int, len(partition.Nodes))
	for _, d := range graph.Dependencies {
		if _, ok := statuses[d.PredecessorID]; ok {
			predecessorsCount[d.SuccessorID]++
		}
	}

	nodes := make([]NodeView, 0, len(partition.Nodes))
	for _, t := range partition.Nodes {
		nodes = append(nodes, NodeView{
			Task:            t,
			Waiting:         WaitingCount(t, graph.Dependencies, statuses),
			HasPredecessors: predecessorsCount[t.ID] > 0,
		})
	}

	return GraphView{Nodes: nodes, Edges: edges}
}

// NodeKey genera la clave de un nodo para evitar re-renderizados innecesarios.
func NodeKey(node NodeView) string {
	return fmt.Sprintf(
		"%s|%s|%s|%s|%s|%s|%t|%d|%s|%d|%t",
		node.Task.ID,
		node.Task.Title,
		node.Task.Prompt,
		node.Task.Branch,
		node.Task.Status,
		node.Task.Priority,
		node.Task.NeedsAttention,
		node.Task.PendingDialogs,
		node.Task.UpdatedAt,
		node.Waiting,
		node.HasPredecessors,
	)
}

// Depths calcula la profundidad de cada nodo en el grafo mediante ordenación topológica por relajación.
func Depths(ids []string, edges [][2]string) map[string]int {
	depth := make(map[string]int, len(ids))
	for _, id := range ids {
		depth[id] = 0
	}
	for range len(ids) {
		changed := false
		for _, edge := range edges {
			predecessor, successor := edge[0], edge[1]
			predDepth, okP := depth[predecessor]
			succDepth, okS := depth[successor]
			if okP && okS && succDepth <= predDepth {
				depth[successor] = predDepth + 1
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return depth
}

// ResolvePositions calcula las posiciones de los nodos: conserva las posiciones guardadas y
// asigna a los demás una posición automática en rejilla según su columna de profundidad.
func ResolvePositions(
	ids []string,
	edges [][2]string,
	saved map[string]Point,
) map[string]Point {
	depth := Depths(ids, edges)
	placed := make(map[string]Point, len(ids))
	occupied := make([]Rect, 0, len(ids))

	for _, id := range ids {
		if pos, ok := saved[id]; ok {
			placed[id] = pos
			occupied = append(occupied, RectOfNode(pos))
		}
	}

	rowsInColumn := make(map[int]int)
	for _, id := range ids {
		if _, exists := placed[id]; exists {
			continue
		}
		col := depth[id]
		row := rowsInColumn[col]
		rowsInColumn[col]++

		pos := Point{
			X: LayoutOrigin.X + float64(col)*(NodeWidth+ColumnGap),
			Y: LayoutOrigin.Y + float64(row)*(NodeHeight+RowGap),
		}

		for range len(ids) + 1 {
			candidate := RectOfNode(pos)
			collides := false
			for _, r := range occupied {
				if r.Intersects(candidate) {
					collides = true
					break
				}
			}
			if !collides {
				break
			}
			pos.Y += NodeHeight + RowGap
		}

		occupied = append(occupied, RectOfNode(pos))
		placed[id] = pos
	}
	return placed
}

// SavedPositions extrae las posiciones guardadas de un TaskGraphDTO como mapa id -> Point.
func SavedPositions(graph dto.TaskGraphDTO) map[string]Point {
	saved := make(map[string]Point, len(graph.Positions))
	for _, p := range graph.Positions {
		saved[p.TaskID] = Point{X: p.X, Y: p.Y}
	}
	return saved
}

// NewTaskLinkKind es el tipo de vínculo de una nueva tarea con una existente.
type NewTaskLinkKind string

const (
	// NewTaskLinkNext indica que la nueva tarea depende de la tarea indicada.
	NewTaskLinkNext NewTaskLinkKind = "next"
	// NewTaskLinkParallel indica que la nueva tarea tiene las mismas predecesoras que la tarea indicada.
	NewTaskLinkParallel NewTaskLinkKind = "parallel"
)

// NewTaskLink representa la relación de una tarea nueva con una existente para «Añadir siguiente/paralela».
type NewTaskLink struct {
	Kind   NewTaskLinkKind
	Anchor string
}

// LinkPredecessors devuelve las predecesoras que debe tener la tarea nueva según el vínculo solicitado.
func LinkPredecessors(link NewTaskLink, dependencies []dto.TaskDependencyDTO) []string {
	switch link.Kind {
	case NewTaskLinkNext:
		return []string{link.Anchor}
	case NewTaskLinkParallel:
		predecessors := make([]string, 0)
		for _, d := range dependencies {
			if d.SuccessorID == link.Anchor {
				predecessors = append(predecessors, d.PredecessorID)
			}
		}
		return predecessors
	default:
		return nil
	}
}

// EdgesForNewTask devuelve los pares (predecesora, sucesora) a crear para una nueva tarea newID.
func EdgesForNewTask(newID string, predecessors []string) [][2]string {
	edges := make([][2]string, 0, len(predecessors))
	for _, p := range predecessors {
		edges = append(edges, [2]string{p, newID})
	}
	return edges
}
