//! Modelo de vista puro del tablero de grafo: nodos, aristas, estado de espera, layout
//! automático, plan de aristas para «siguiente»/«paralela» y elección del workspace.

use std::collections::{HashMap, HashSet};

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskStageDto, TaskStatusDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_graph_dto::{
    NodePositionDto, TaskDependencyDto, TaskGraphDto,
};
use crate::modules::tasks::presentation::state::graph_geometry::{
    NODE_HEIGHT, NODE_WIDTH, Point, Rect,
};

/// Separación horizontal entre columnas del layout automático.
const COLUMN_GAP: f64 = 96.0;
/// Separación vertical entre filas del layout automático.
const ROW_GAP: f64 = 40.0;
/// Origen del layout automático.
const LAYOUT_ORIGIN: Point = Point::new(48.0, 48.0);

/// Estado visual de una arista según la tarea predecesora.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum EdgeState {
    /// La predecesora ya está integrada: la arista se ha cumplido.
    Done,
    /// La predecesora está en marcha: la arista «fluye».
    Working,
    /// La predecesora aún no ha empezado (o fue descartada).
    Pending,
}

/// Estado de una arista a partir del estado de su predecesora.
pub fn edge_state(predecessor: TaskStatusDto) -> EdgeState {
    match predecessor {
        TaskStatusDto::Merged => EdgeState::Done,
        TaskStatusDto::Backlog | TaskStatusDto::Discarded => EdgeState::Pending,
        _ => EdgeState::Working,
    }
}

/// Arista lista para pintar.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct EdgeView {
    pub predecessor_id: String,
    pub successor_id: String,
    pub state: EdgeState,
    /// Evento de la predecesora que dispara la arista (por ahora siempre `merged`).
    pub trigger: String,
    /// Etiqueta visible en el centro de la arista. Reservada para las condiciones futuras:
    /// hoy siempre es `None`.
    pub label: Option<String>,
}

impl EdgeView {
    /// Clave única de la arista.
    pub fn key(&self) -> String {
        edge_key(&self.predecessor_id, &self.successor_id)
    }
}

/// Clave única de una arista entre dos tareas.
pub fn edge_key(predecessor_id: &str, successor_id: &str) -> String {
    format!("{predecessor_id}->{successor_id}")
}

/// Nodo listo para pintar: la tarea y cuántas predecesoras le faltan por integrarse.
#[derive(Debug, Clone, PartialEq)]
pub struct NodeView {
    pub task: TaskDto,
    /// Predecesoras aún no integradas; solo cuenta para tareas en `Backlog`.
    pub waiting: usize,
}

/// Grafo visible de un workspace.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct GraphView {
    pub nodes: Vec<NodeView>,
    pub edges: Vec<EdgeView>,
}

/// Predecesoras de `task` que aún no están integradas. Una predecesora cuyo estado se
/// desconoce (p. ej. una finalizada antigua que no está cargada) no se cuenta.
pub fn waiting_count(
    task: &TaskDto,
    dependencies: &[TaskDependencyDto],
    statuses: &HashMap<&str, TaskStatusDto>,
) -> usize {
    if task.status != TaskStatusDto::Backlog {
        return 0;
    }
    dependencies
        .iter()
        .filter(|d: &&TaskDependencyDto| d.successor_id == task.id)
        .filter(|d: &&TaskDependencyDto| {
            statuses
                .get(d.predecessor_id.as_str())
                .is_some_and(|status: &TaskStatusDto| *status != TaskStatusDto::Merged)
        })
        .count()
}

/// Subpestaña del tablero en pantallas estrechas, donde los dos paneles no caben a la vez.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BoardTab {
    Loose,
    Graph,
}

impl BoardTab {
    /// Valor estable para el control segmentado y el atributo `data-tab`.
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Loose => "loose",
            Self::Graph => "graph",
        }
    }

    pub fn from_value(value: &str) -> Option<Self> {
        match value {
            "loose" => Some(Self::Loose),
            "graph" => Some(Self::Graph),
            _ => None,
        }
    }
}

/// Subpestaña por defecto: el grafo, salvo que no tenga nodos y haya tareas sueltas.
pub fn default_board_tab(graph_nodes: usize, loose: usize) -> BoardTab {
    if graph_nodes == 0 && loose > 0 {
        BoardTab::Loose
    } else {
        BoardTab::Graph
    }
}

/// Tareas de `workspace_id` que se muestran (las finalizadas solo con `show_done`), de la más
/// antigua a la más reciente.
fn visible_tasks<'a>(
    workspace_id: &str,
    tasks: &'a [TaskDto],
    show_done: bool,
) -> Vec<&'a TaskDto> {
    let mut visible: Vec<&TaskDto> = tasks
        .iter()
        .filter(|t: &&TaskDto| t.workspace_id == workspace_id)
        .filter(|t: &&TaskDto| show_done || t.stage != TaskStageDto::Done)
        .collect();
    visible.sort_by(|a: &&TaskDto, b: &&TaskDto| {
        a.created_at
            .cmp(&b.created_at)
            .then_with(|| a.id.cmp(&b.id))
    });
    visible
}

/// Reparto de las tareas visibles de un workspace entre el lienzo y el panel de sueltas.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct BoardPartition<'a> {
    /// Tareas que se pintan como nodos del grafo, de la más antigua a la más reciente.
    pub nodes: Vec<&'a TaskDto>,
    /// Tareas sueltas del panel izquierdo, de la más antigua a la más reciente.
    pub loose: Vec<&'a TaskDto>,
}

/// Única definición de qué es un nodo y qué una tarea suelta: cada tarea visible de
/// `workspace_id` cae en exactamente uno de los dos grupos. Es nodo si tiene una arista hacia
/// otra tarea visible, una posición guardada o una posición recién fijada (`placed`: ids que
/// el servidor aún no ha devuelto); si no, está suelta. Las aristas cuyo otro extremo no se
/// ve (finalizada oculta, otro workspace) no cuentan.
pub fn partition_tasks<'a>(
    workspace_id: &str,
    tasks: &'a [TaskDto],
    graph: &TaskGraphDto,
    show_done: bool,
    placed: &HashSet<String>,
) -> BoardPartition<'a> {
    let visible: Vec<&TaskDto> = visible_tasks(workspace_id, tasks, show_done);
    let ids: HashSet<&str> = visible.iter().map(|t: &&TaskDto| t.id.as_str()).collect();
    let linked: HashSet<&str> = graph
        .dependencies
        .iter()
        .filter(|d: &&TaskDependencyDto| {
            ids.contains(d.predecessor_id.as_str()) && ids.contains(d.successor_id.as_str())
        })
        .flat_map(|d: &TaskDependencyDto| [d.predecessor_id.as_str(), d.successor_id.as_str()])
        .collect();
    let positioned: HashSet<&str> = graph
        .positions
        .iter()
        .map(|p: &NodePositionDto| p.task_id.as_str())
        .collect();
    let (nodes, loose): (Vec<&TaskDto>, Vec<&TaskDto>) =
        visible.into_iter().partition(|t: &&TaskDto| {
            let id: &str = t.id.as_str();
            linked.contains(id) || positioned.contains(id) || placed.contains(id)
        });
    BoardPartition { nodes, loose }
}

/// Tareas sueltas visibles de `workspace_id`: las que muestra el panel izquierdo y que no se
/// pintan en el lienzo.
pub fn loose_tasks(
    workspace_id: &str,
    tasks: &[TaskDto],
    graph: &TaskGraphDto,
    show_done: bool,
    placed: &HashSet<String>,
) -> Vec<TaskDto> {
    partition_tasks(workspace_id, tasks, graph, show_done, placed)
        .loose
        .into_iter()
        .cloned()
        .collect()
}

/// Construye el grafo visible de `workspace_id` con las tareas dadas (activas y finalizadas).
/// Los nodos van por antigüedad; las tareas sueltas (`partition_tasks`) no son nodos; las finalizadas se ocultan
/// con `show_done = false` junto con sus aristas; las aristas con un extremo no visible se
/// descartan.
pub fn build_graph(
    workspace_id: &str,
    tasks: &[TaskDto],
    graph: &TaskGraphDto,
    show_done: bool,
    placed: &HashSet<String>,
) -> GraphView {
    let visible: Vec<&TaskDto> =
        partition_tasks(workspace_id, tasks, graph, show_done, placed).nodes;
    let statuses: HashMap<&str, TaskStatusDto> = tasks
        .iter()
        .map(|t: &TaskDto| (t.id.as_str(), t.status))
        .collect();
    let ids: HashSet<&str> = visible.iter().map(|t: &&TaskDto| t.id.as_str()).collect();

    let edges: Vec<EdgeView> = graph
        .dependencies
        .iter()
        .filter(|d: &&TaskDependencyDto| {
            ids.contains(d.predecessor_id.as_str()) && ids.contains(d.successor_id.as_str())
        })
        .filter_map(|d: &TaskDependencyDto| {
            let status: TaskStatusDto = *statuses.get(d.predecessor_id.as_str())?;
            Some(EdgeView {
                predecessor_id: d.predecessor_id.clone(),
                successor_id: d.successor_id.clone(),
                state: edge_state(status),
                trigger: d.trigger.clone(),
                label: None,
            })
        })
        .collect();
    let nodes: Vec<NodeView> = visible
        .into_iter()
        .map(|task: &TaskDto| NodeView {
            waiting: waiting_count(task, &graph.dependencies, &statuses),
            task: task.clone(),
        })
        .collect();
    GraphView { nodes, edges }
}

/// Clave de un nodo para el `<For>`: cambia cuando cambia cualquier dato que pinta.
pub fn node_key(node: &NodeView) -> String {
    format!(
        "{}|{}|{}|{}|{:?}|{:?}|{:?}|{}|{}|{:?}|{}|{}",
        node.task.id,
        node.task.updated_at,
        node.task.title,
        node.task.prompt,
        node.task.stage,
        node.task.status,
        node.task.priority,
        node.task.needs_attention,
        node.task.pending_dialogs,
        node.task.error,
        node.waiting,
        node.task.resume_at.is_some(),
    )
}

/// Profundidad de cada nodo: longitud del camino más largo desde una raíz (sin predecesoras).
/// Si las aristas formaran un ciclo (el servidor lo impide) los nodos implicados se quedan
/// con la profundidad alcanzada sin recorrerlo indefinidamente.
pub fn depths(ids: &[String], edges: &[(String, String)]) -> HashMap<String, usize> {
    let mut depth: HashMap<String, usize> = ids.iter().map(|id: &String| (id.clone(), 0)).collect();
    for _ in 0..ids.len() {
        let mut changed: bool = false;
        for (predecessor, successor) in edges {
            let Some(from) = depth.get(predecessor).copied() else {
                continue;
            };
            if let Some(to) = depth.get_mut(successor)
                && *to < from + 1
            {
                *to = from + 1;
                changed = true;
            }
        }
        if !changed {
            break;
        }
    }
    depth
}

/// Posición efectiva de cada nodo: la guardada si existe y, para los demás, un layout por
/// capas (profundidad → columna, orden → fila) que esquiva los nodos ya situados. Los nodos
/// con posición guardada nunca se mueven.
pub fn resolve_positions(
    ids: &[String],
    edges: &[(String, String)],
    saved: &HashMap<String, Point>,
) -> HashMap<String, Point> {
    let depth: HashMap<String, usize> = depths(ids, edges);
    let mut placed: HashMap<String, Point> = HashMap::new();
    let mut occupied: Vec<Rect> = Vec::new();
    for id in ids {
        if let Some(position) = saved.get(id) {
            placed.insert(id.clone(), *position);
            occupied.push(Rect::of_node(*position));
        }
    }
    let mut rows_in_column: HashMap<usize, usize> = HashMap::new();
    for id in ids {
        if placed.contains_key(id) {
            continue;
        }
        let column: usize = depth.get(id).copied().unwrap_or(0);
        let row: &mut usize = rows_in_column.entry(column).or_insert(0);
        let mut position: Point = Point::new(
            LAYOUT_ORIGIN.x + column as f64 * (NODE_WIDTH + COLUMN_GAP),
            LAYOUT_ORIGIN.y + *row as f64 * (NODE_HEIGHT + ROW_GAP),
        );
        *row += 1;
        // Como mucho hay `ids.len()` obstáculos, así que el bucle termina.
        for _ in 0..=ids.len() {
            let candidate: Rect = Rect::of_node(position);
            if !occupied.iter().any(|r: &Rect| r.intersects(&candidate)) {
                break;
            }
            position.y += NODE_HEIGHT + ROW_GAP;
        }
        occupied.push(Rect::of_node(position));
        placed.insert(id.clone(), position);
    }
    placed
}

/// Posiciones guardadas en el servidor, como mapa id → punto.
pub fn saved_positions(graph: &TaskGraphDto) -> HashMap<String, Point> {
    graph
        .positions
        .iter()
        .map(|p| (p.task_id.clone(), Point::new(p.x, p.y)))
        .collect()
}

/// Relación de una tarea nueva con una existente, para «Añadir siguiente/paralela».
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum NewTaskLink {
    /// La nueva depende de la tarea indicada.
    Next(String),
    /// La nueva tiene las mismas predecesoras que la tarea indicada.
    Parallel(String),
}

/// Predecesoras que debe tener la tarea nueva según el vínculo pedido.
pub fn link_predecessors(link: &NewTaskLink, dependencies: &[TaskDependencyDto]) -> Vec<String> {
    match link {
        NewTaskLink::Next(anchor) => vec![anchor.clone()],
        NewTaskLink::Parallel(anchor) => dependencies
            .iter()
            .filter(|d: &&TaskDependencyDto| d.successor_id == *anchor)
            .map(|d: &TaskDependencyDto| d.predecessor_id.clone())
            .collect(),
    }
}

/// Aristas (predecesora, sucesora) que hay que crear para la tarea nueva `new_id`.
pub fn edges_for_new_task(new_id: &str, predecessors: &[String]) -> Vec<(String, String)> {
    predecessors
        .iter()
        .map(|p: &String| (p.clone(), new_id.to_string()))
        .collect()
}

/// Workspace que muestra una pestaña de tablero.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BoardWorkspace {
    /// Aún no se conocen los workspaces y la ruta no indica ninguno.
    Loading,
    /// No hay ningún workspace registrado.
    Empty,
    /// La ruta indica un workspace que no existe.
    Missing,
    Selected(String),
}

/// Workspace por defecto: el primero por nombre (sin distinguir mayúsculas; el id desempata).
pub fn default_workspace(workspaces: &[WorkspaceOptionDto]) -> Option<String> {
    workspaces
        .iter()
        .min_by(|a: &&WorkspaceOptionDto, b: &&WorkspaceOptionDto| {
            a.name
                .to_lowercase()
                .cmp(&b.name.to_lowercase())
                .then_with(|| a.id.cmp(&b.id))
        })
        .map(|w: &WorkspaceOptionDto| w.id.clone())
}

/// Resuelve el workspace del tablero: el de la ruta si la hay (se usa ya, aunque la lista
/// aún no haya llegado) o, en `/`, el primero por nombre.
pub fn resolve_board_workspace(
    route_workspace: Option<&str>,
    workspaces: Option<&[WorkspaceOptionDto]>,
) -> BoardWorkspace {
    match (route_workspace, workspaces) {
        (Some(id), Some(all)) if !all.iter().any(|w: &WorkspaceOptionDto| w.id == id) => {
            BoardWorkspace::Missing
        }
        (Some(id), _) => BoardWorkspace::Selected(id.to_string()),
        (None, None) => BoardWorkspace::Loading,
        (None, Some(all)) => default_workspace(all)
            .map(BoardWorkspace::Selected)
            .unwrap_or(BoardWorkspace::Empty),
    }
}

/// Ruta del tablero de un workspace.
pub fn board_path(workspace_id: &str) -> String {
    format!("/board/{workspace_id}")
}

/// Destino de «volver al tablero» desde una tarea: el tablero de su workspace o, si no se
/// conoce, el tablero por defecto (`/`).
pub fn back_to_board_path(workspace_id: Option<&str>) -> String {
    workspace_id.map_or_else(|| "/".to_string(), board_path)
}

/// Destino de «volver al tablero» a partir de la tarea cargada (si ya llegó y no falló):
/// punto único para las pantallas de detalle y edición.
pub fn board_return_path<E>(task: Option<Result<TaskDto, E>>) -> String {
    let workspace_id: Option<String> = task.and_then(Result::ok).map(|t: TaskDto| t.workspace_id);
    back_to_board_path(workspace_id.as_deref())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_graph_dto::NodePositionDto;

    fn task(id: &str, workspace_id: &str, status: TaskStatusDto, created_at: &str) -> TaskDto {
        let stage: TaskStageDto = match status {
            TaskStatusDto::Backlog => TaskStageDto::Backlog,
            TaskStatusDto::Merged | TaskStatusDto::Discarded => TaskStageDto::Done,
            _ => TaskStageDto::Working,
        };
        TaskDto {
            id: id.to_string(),
            workspace_id: workspace_id.to_string(),
            title: format!("tarea {id}"),
            prompt: "prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status,
            priority: Default::default(),
            memory_limit_mb: None,
            cpu_limit_percent: None,
            error: None,
            resume_at: None,
            actions: Default::default(),
            stage,
            needs_attention: false,
            pending_dialogs: 0,
            created_at: created_at.to_string(),
            updated_at: created_at.to_string(),
        }
    }

    fn dep(predecessor: &str, successor: &str) -> TaskDependencyDto {
        TaskDependencyDto {
            predecessor_id: predecessor.to_string(),
            successor_id: successor.to_string(),
            trigger: "merged".to_string(),
        }
    }

    fn workspace(id: &str, name: &str) -> WorkspaceOptionDto {
        WorkspaceOptionDto {
            id: id.to_string(),
            name: name.to_string(),
            base_branch: "main".to_string(),
        }
    }

    fn ids(list: &[&str]) -> Vec<String> {
        list.iter().map(|s: &&str| (*s).to_string()).collect()
    }

    fn pairs(list: &[(&str, &str)]) -> Vec<(String, String)> {
        list.iter()
            .map(|(a, b): &(&str, &str)| ((*a).to_string(), (*b).to_string()))
            .collect()
    }

    #[test]
    fn edge_state_follows_the_predecessor() {
        assert_eq!(edge_state(TaskStatusDto::Merged), EdgeState::Done);
        assert_eq!(edge_state(TaskStatusDto::Running), EdgeState::Working);
        assert_eq!(
            edge_state(TaskStatusDto::AwaitingReview),
            EdgeState::Working
        );
        assert_eq!(edge_state(TaskStatusDto::Backlog), EdgeState::Pending);
        assert_eq!(edge_state(TaskStatusDto::Discarded), EdgeState::Pending);
    }

    #[test]
    fn waiting_counts_unmerged_predecessors_of_backlog_tasks_only() {
        let tasks: Vec<TaskDto> = vec![
            task("a", "w", TaskStatusDto::Merged, "1"),
            task("b", "w", TaskStatusDto::Running, "2"),
            task("c", "w", TaskStatusDto::Backlog, "3"),
            task("d", "w", TaskStatusDto::Running, "4"),
        ];
        let deps: Vec<TaskDependencyDto> = vec![dep("a", "c"), dep("b", "c"), dep("a", "d")];
        let statuses: HashMap<&str, TaskStatusDto> = tasks
            .iter()
            .map(|t: &TaskDto| (t.id.as_str(), t.status))
            .collect();

        assert_eq!(waiting_count(&tasks[2], &deps, &statuses), 1);
        // Una tarea que ya arrancó no espera a nadie.
        assert_eq!(waiting_count(&tasks[3], &deps, &statuses), 0);
        assert_eq!(waiting_count(&tasks[0], &deps, &statuses), 0);
    }

    #[test]
    fn waiting_ignores_predecessors_whose_state_is_unknown() {
        let backlog: TaskDto = task("c", "w", TaskStatusDto::Backlog, "3");
        let statuses: HashMap<&str, TaskStatusDto> = HashMap::new();
        assert_eq!(waiting_count(&backlog, &[dep("zz", "c")], &statuses), 0);
    }

    #[test]
    fn build_graph_keeps_one_workspace_and_orders_by_age() {
        let tasks: Vec<TaskDto> = vec![
            task("late", "w", TaskStatusDto::Backlog, "3"),
            task("early", "w", TaskStatusDto::Running, "1"),
            task("other", "x", TaskStatusDto::Backlog, "2"),
        ];
        let graph: TaskGraphDto = TaskGraphDto {
            dependencies: vec![dep("early", "late"), dep("early", "other")],
            positions: Vec::new(),
        };
        let view: GraphView = build_graph("w", &tasks, &graph, true, &HashSet::new());
        let order: Vec<&str> = view
            .nodes
            .iter()
            .map(|n: &NodeView| n.task.id.as_str())
            .collect();
        assert_eq!(order, ["early", "late"]);
        // La arista hacia otro workspace no se pinta.
        assert_eq!(view.edges.len(), 1);
        assert_eq!(view.edges[0].state, EdgeState::Working);
        assert_eq!(view.edges[0].label, None);
        assert_eq!(view.nodes[1].waiting, 1);
    }

    #[test]
    fn hiding_completed_tasks_hides_their_edges() {
        let tasks: Vec<TaskDto> = vec![
            task("a", "w", TaskStatusDto::Merged, "1"),
            task("b", "w", TaskStatusDto::Backlog, "2"),
        ];
        let graph: TaskGraphDto = TaskGraphDto {
            dependencies: vec![dep("a", "b")],
            positions: Vec::new(),
        };
        let shown: GraphView = build_graph("w", &tasks, &graph, true, &HashSet::new());
        assert_eq!((shown.nodes.len(), shown.edges.len()), (2, 1));
        assert_eq!(shown.edges[0].state, EdgeState::Done);
        let hidden: GraphView = build_graph("w", &tasks, &graph, false, &HashSet::new());
        // `b` solo estaba enlazada a la finalizada oculta: pasa a las sueltas.
        assert_eq!((hidden.nodes.len(), hidden.edges.len()), (0, 0));
    }

    fn positioned(id: &str) -> NodePositionDto {
        NodePositionDto {
            task_id: id.to_string(),
            x: 1.0,
            y: 2.0,
        }
    }

    #[test]
    fn default_tab_is_the_graph_unless_only_loose_tasks_exist() {
        assert_eq!(default_board_tab(3, 2), BoardTab::Graph);
        assert_eq!(default_board_tab(0, 0), BoardTab::Graph);
        assert_eq!(default_board_tab(0, 1), BoardTab::Loose);
        assert_eq!(
            BoardTab::from_value(BoardTab::Loose.as_str()),
            Some(BoardTab::Loose)
        );
        assert_eq!(BoardTab::from_value("graph"), Some(BoardTab::Graph));
        assert_eq!(BoardTab::from_value("x"), None);
    }

    #[test]
    fn loose_tasks_leave_the_graph_and_respect_the_workspace_and_completed_filter() {
        let tasks: Vec<TaskDto> = vec![
            task("loose-late", "w", TaskStatusDto::Backlog, "3"),
            task("loose-early", "w", TaskStatusDto::Backlog, "1"),
            task("loose-done", "w", TaskStatusDto::Merged, "2"),
            task("chained", "w", TaskStatusDto::Backlog, "4"),
            task("head", "w", TaskStatusDto::Running, "5"),
            task("placed", "w", TaskStatusDto::Backlog, "6"),
            task("elsewhere", "x", TaskStatusDto::Backlog, "7"),
        ];
        let graph: TaskGraphDto = TaskGraphDto {
            dependencies: vec![dep("head", "chained")],
            positions: vec![positioned("placed")],
        };
        let none: HashSet<String> = HashSet::new();
        let ids_of = |list: Vec<TaskDto>| -> Vec<String> {
            list.into_iter().map(|t: TaskDto| t.id).collect()
        };

        assert_eq!(
            ids_of(loose_tasks("w", &tasks, &graph, true, &none)),
            ["loose-early", "loose-done", "loose-late"]
        );
        assert_eq!(
            ids_of(loose_tasks("w", &tasks, &graph, false, &none)),
            ["loose-early", "loose-late"]
        );
        let view: GraphView = build_graph("w", &tasks, &graph, true, &none);
        let nodes: Vec<&str> = view
            .nodes
            .iter()
            .map(|n: &NodeView| n.task.id.as_str())
            .collect();
        assert_eq!(nodes, ["chained", "head", "placed"]);
    }

    fn partition_ids(
        tasks: &[TaskDto],
        graph: &TaskGraphDto,
        show_done: bool,
        placed: &HashSet<String>,
    ) -> (Vec<String>, Vec<String>) {
        let part: BoardPartition = partition_tasks("w", tasks, graph, show_done, placed);
        let ids = |list: Vec<&TaskDto>| -> Vec<String> {
            list.into_iter().map(|t: &TaskDto| t.id.clone()).collect()
        };
        (ids(part.nodes), ids(part.loose))
    }

    #[test]
    fn a_task_linked_only_to_a_hidden_completed_one_is_loose() {
        let tasks: Vec<TaskDto> = vec![
            task("a", "w", TaskStatusDto::Merged, "1"),
            task("b", "w", TaskStatusDto::Backlog, "2"),
        ];
        let graph: TaskGraphDto = TaskGraphDto {
            dependencies: vec![dep("a", "b")],
            positions: Vec::new(),
        };
        let none: HashSet<String> = HashSet::new();
        let (nodes, loose): (Vec<String>, Vec<String>) =
            partition_ids(&tasks, &graph, false, &none);
        assert!(nodes.is_empty());
        assert_eq!(loose, ["b"]);
        // Con las finalizadas visibles la arista existe y ambas son nodos.
        let (nodes, loose): (Vec<String>, Vec<String>) = partition_ids(&tasks, &graph, true, &none);
        assert_eq!(nodes, ["a", "b"]);
        assert!(loose.is_empty());
    }

    #[test]
    fn a_task_that_lost_its_last_edge_returns_to_the_loose_panel() {
        let tasks: Vec<TaskDto> = vec![
            task("a", "w", TaskStatusDto::Running, "1"),
            task("b", "w", TaskStatusDto::Backlog, "2"),
        ];
        let none: HashSet<String> = HashSet::new();
        let linked: TaskGraphDto = TaskGraphDto {
            dependencies: vec![dep("a", "b")],
            positions: Vec::new(),
        };
        assert_eq!(partition_ids(&tasks, &linked, true, &none).0, ["a", "b"]);
        let unlinked: TaskGraphDto = TaskGraphDto::default();
        let (nodes, loose): (Vec<String>, Vec<String>) =
            partition_ids(&tasks, &unlinked, true, &none);
        assert!(nodes.is_empty());
        assert_eq!(loose, ["a", "b"]);
    }

    #[test]
    fn every_visible_task_is_in_exactly_one_group() {
        let tasks: Vec<TaskDto> = vec![
            task("done", "w", TaskStatusDto::Merged, "1"),
            task("tail", "w", TaskStatusDto::Backlog, "2"),
            task("head", "w", TaskStatusDto::Running, "3"),
            task("mid", "w", TaskStatusDto::Backlog, "4"),
            task("saved", "w", TaskStatusDto::Backlog, "5"),
            task("fresh", "w", TaskStatusDto::Backlog, "6"),
            task("free", "w", TaskStatusDto::Backlog, "7"),
            task("other", "x", TaskStatusDto::Backlog, "8"),
        ];
        let graphs: Vec<TaskGraphDto> = vec![
            TaskGraphDto::default(),
            TaskGraphDto {
                dependencies: vec![
                    dep("done", "tail"),
                    dep("head", "mid"),
                    dep("head", "other"),
                ],
                positions: vec![positioned("saved"), positioned("ghost")],
            },
            TaskGraphDto {
                dependencies: vec![dep("done", "tail")],
                positions: vec![positioned("tail")],
            },
        ];
        let placed_sets: Vec<HashSet<String>> = vec![
            HashSet::new(),
            HashSet::from(["fresh".to_string()]),
            HashSet::from(["free".to_string(), "other".to_string()]),
        ];
        for graph in &graphs {
            for placed in &placed_sets {
                for show_done in [true, false] {
                    let (nodes, loose): (Vec<String>, Vec<String>) =
                        partition_ids(&tasks, graph, show_done, placed);
                    let mut all: Vec<String> = nodes.iter().chain(loose.iter()).cloned().collect();
                    all.sort();
                    let mut expected: Vec<String> = tasks
                        .iter()
                        .filter(|t: &&TaskDto| t.workspace_id == "w")
                        .filter(|t: &&TaskDto| show_done || t.stage != TaskStageDto::Done)
                        .map(|t: &TaskDto| t.id.clone())
                        .collect();
                    expected.sort();
                    assert_eq!(all, expected, "show_done={show_done} placed={placed:?}");
                    // Y el grafo pinta exactamente los nodos.
                    let view: GraphView = build_graph("w", &tasks, graph, show_done, placed);
                    let drawn: Vec<String> = view
                        .nodes
                        .iter()
                        .map(|n: &NodeView| n.task.id.clone())
                        .collect();
                    assert_eq!(drawn, nodes);
                }
            }
        }
    }

    #[test]
    fn node_key_changes_with_waiting_and_status() {
        let mut node: NodeView = NodeView {
            task: task("a", "w", TaskStatusDto::Backlog, "1"),
            waiting: 1,
        };
        let before: String = node_key(&node);
        node.waiting = 0;
        assert_ne!(before, node_key(&node));
        let after_waiting: String = node_key(&node);
        node.task.status = TaskStatusDto::Running;
        assert_ne!(after_waiting, node_key(&node));
    }

    #[test]
    fn depth_is_the_longest_path_from_a_root() {
        let nodes: Vec<String> = ids(&["a", "b", "c", "d"]);
        // a -> b -> c, a -> c, d aislado.
        let edges: Vec<(String, String)> = pairs(&[("a", "b"), ("b", "c"), ("a", "c")]);
        let depth: HashMap<String, usize> = depths(&nodes, &edges);
        assert_eq!(depth["a"], 0);
        assert_eq!(depth["b"], 1);
        assert_eq!(depth["c"], 2);
        assert_eq!(depth["d"], 0);
    }

    #[test]
    fn depth_terminates_on_cycles() {
        let nodes: Vec<String> = ids(&["a", "b"]);
        let edges: Vec<(String, String)> = pairs(&[("a", "b"), ("b", "a")]);
        let depth: HashMap<String, usize> = depths(&nodes, &edges);
        assert_eq!(depth.len(), 2);
    }

    #[test]
    fn auto_layout_puts_depth_in_columns_and_order_in_rows() {
        let nodes: Vec<String> = ids(&["a", "b", "c"]);
        let edges: Vec<(String, String)> = pairs(&[("a", "c")]);
        let placed: HashMap<String, Point> = resolve_positions(&nodes, &edges, &HashMap::new());
        assert_eq!(placed["a"], Point::new(48.0, 48.0));
        assert_eq!(placed["b"], Point::new(48.0, 48.0 + NODE_HEIGHT + ROW_GAP));
        assert_eq!(
            placed["c"],
            Point::new(48.0 + NODE_WIDTH + COLUMN_GAP, 48.0)
        );
    }

    #[test]
    fn auto_layout_keeps_saved_positions_and_dodges_them() {
        let nodes: Vec<String> = ids(&["a", "b"]);
        let mut saved: HashMap<String, Point> = HashMap::new();
        saved.insert("a".to_string(), Point::new(500.0, 500.0));
        // `b` caería justo donde está `a`: se desplaza hacia abajo.
        saved.insert("b".to_string(), Point::new(48.0, 48.0));
        let placed: HashMap<String, Point> = resolve_positions(&nodes, &[], &saved);
        assert_eq!(placed["a"], Point::new(500.0, 500.0));
        assert_eq!(placed["b"], Point::new(48.0, 48.0));

        let with_new: Vec<String> = ids(&["a", "b", "n"]);
        let placed_new: HashMap<String, Point> = resolve_positions(&with_new, &[], &saved);
        assert_eq!(placed_new["a"], Point::new(500.0, 500.0));
        assert_eq!(placed_new["b"], Point::new(48.0, 48.0));
        let fresh: Rect = Rect::of_node(placed_new["n"]);
        assert!(!fresh.intersects(&Rect::of_node(Point::new(48.0, 48.0))));
        assert!(!fresh.intersects(&Rect::of_node(Point::new(500.0, 500.0))));
    }

    #[test]
    fn saved_positions_map_by_task() {
        let graph: TaskGraphDto = TaskGraphDto {
            dependencies: Vec::new(),
            positions: vec![NodePositionDto {
                task_id: "a".to_string(),
                x: 1.0,
                y: 2.0,
            }],
        };
        assert_eq!(saved_positions(&graph)["a"], Point::new(1.0, 2.0));
    }

    #[test]
    fn next_task_depends_on_the_anchor() {
        let deps: Vec<TaskDependencyDto> = vec![dep("p", "a")];
        let preds: Vec<String> = link_predecessors(&NewTaskLink::Next("a".to_string()), &deps);
        assert_eq!(preds, ids(&["a"]));
        assert_eq!(edges_for_new_task("n", &preds), pairs(&[("a", "n")]));
    }

    #[test]
    fn parallel_task_copies_the_anchor_predecessors() {
        let deps: Vec<TaskDependencyDto> = vec![dep("p", "a"), dep("q", "a"), dep("a", "z")];
        let preds: Vec<String> = link_predecessors(&NewTaskLink::Parallel("a".to_string()), &deps);
        assert_eq!(preds, ids(&["p", "q"]));
        assert_eq!(
            edges_for_new_task("n", &preds),
            pairs(&[("p", "n"), ("q", "n")])
        );
        // Sin predecesoras: la tarea paralela es una raíz más.
        assert!(link_predecessors(&NewTaskLink::Parallel("p".to_string()), &deps).is_empty());
    }

    #[test]
    fn default_workspace_is_first_by_name_ignoring_case() {
        let all: Vec<WorkspaceOptionDto> = vec![
            workspace("w3", "zeta"),
            workspace("w2", "Beta"),
            workspace("w1", "alpha"),
        ];
        assert_eq!(default_workspace(&all).as_deref(), Some("w1"));
        assert_eq!(default_workspace(&[]), None);
        let tie: Vec<WorkspaceOptionDto> = vec![workspace("b", "same"), workspace("a", "same")];
        assert_eq!(default_workspace(&tie).as_deref(), Some("a"));
    }

    #[test]
    fn board_workspace_resolution() {
        let all: Vec<WorkspaceOptionDto> = vec![workspace("w2", "beta"), workspace("w1", "alpha")];
        assert_eq!(
            resolve_board_workspace(None, Some(&all)),
            BoardWorkspace::Selected("w1".to_string())
        );
        assert_eq!(
            resolve_board_workspace(Some("w2"), Some(&all)),
            BoardWorkspace::Selected("w2".to_string())
        );
        assert_eq!(
            resolve_board_workspace(Some("nope"), Some(&all)),
            BoardWorkspace::Missing
        );
        // La ruta manda aunque la lista de workspaces aún no haya llegado.
        assert_eq!(
            resolve_board_workspace(Some("w2"), None),
            BoardWorkspace::Selected("w2".to_string())
        );
        assert_eq!(resolve_board_workspace(None, None), BoardWorkspace::Loading);
        assert_eq!(
            resolve_board_workspace(None, Some(&[])),
            BoardWorkspace::Empty
        );
    }

    #[test]
    fn board_path_is_workspace_scoped() {
        assert_eq!(board_path("w1"), "/board/w1");
        assert_eq!(back_to_board_path(Some("w1")), "/board/w1");
        assert_eq!(back_to_board_path(None), "/");
    }

    #[test]
    fn the_return_path_comes_from_the_loaded_task_or_falls_back_to_the_default_board() {
        let loaded: TaskDto = task("a", "w9", TaskStatusDto::Running, "1");
        assert_eq!(board_return_path::<String>(Some(Ok(loaded))), "/board/w9");
        assert_eq!(board_return_path::<String>(Some(Err("x".to_string()))), "/");
        assert_eq!(board_return_path::<String>(None), "/");
        assert_eq!(edge_key("a", "b"), "a->b");
    }
}
