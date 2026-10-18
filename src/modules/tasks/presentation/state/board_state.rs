use std::collections::{HashMap, HashSet};
use std::future::Future;

use leptos::prelude::*;

use crate::core::components::form::parse_positive_number;
use crate::core::components::toast::{ToastMessage, push_toast};
use crate::core::dock::context::use_tab_params;
use crate::core::i18n::Msg;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::ImageAttachmentDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    BranchOptionDto, CreateTaskDto, CreatedTaskDto, TaskDto, TaskPriorityDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_graph_dto::{
    TaskDependencyDto, TaskGraphDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    AddTaskDependency, DeleteTask, FinishTask, MergeTask, RemoveTaskDependency, SetTaskPosition,
    StartTask, add_task_dependency, create_task, get_task_graph, list_done_tasks,
    list_workspace_branches,
};
use crate::modules::tasks::presentation::state::graph_geometry::{
    Point, Rect, Viewport, ZOOM_STEP, bounds_of_nodes, fit_viewport, wheel_zoom_factor,
};
use crate::modules::tasks::presentation::state::graph_interaction::{
    DragEffect, DragMode, DragOutcome,
};
use crate::modules::tasks::presentation::state::graph_model::{
    BoardTab, BoardWorkspace, GraphView, NewTaskLink, build_graph, default_board_tab,
    edges_for_new_task, link_predecessors, loose_tasks, resolve_board_workspace, resolve_positions,
    saved_positions,
};
use crate::modules::tasks::presentation::state::image_attachments::ImageAttachments;
use crate::modules::tasks::presentation::state::loose_drag::{
    LooseDrag, centre_position, drop_position,
};
use crate::modules::tasks::presentation::state::placement::{
    can_place, place, roll_back, unconfirmed,
};
use crate::modules::tasks::presentation::state::task_store::{TaskStore, use_task_store};

const START_FAILED_AFTER_CREATE: Msg = Msg {
    en: "Task created in To do, but it could not be started: {error}",
    es: "Tarea creada en Por hacer, pero no se pudo iniciar: {error}",
    zh: "任务已创建在“待办”中，但无法启动：{error}",
    de: "Aufgabe in „Zu erledigen“ erstellt, konnte aber nicht gestartet werden: {error}",
    fr: "Tâche créée dans « À faire », mais impossible de la démarrer : {error}",
};

/// Tamaño de página de las tareas finalizadas.
pub const DONE_PAGE_SIZE: usize = 50;

/// Desplazamiento de la página `page` (empieza en 0) de tareas finalizadas.
pub fn done_page_offset(page: usize) -> i64 {
    i64::try_from(page * DONE_PAGE_SIZE).unwrap_or(i64::MAX)
}

/// Puede haber más tareas finalizadas si la última página recibida del servidor vino completa.
/// `None` significa que no se ha cargado ninguna página; no depende de los parches de SSE.
pub fn done_has_more(last_page_len: Option<usize>) -> bool {
    last_page_len.is_some_and(|len: usize| len >= DONE_PAGE_SIZE)
}

/// Resultado de cargar las páginas de finalizadas.
#[derive(Debug, Clone, PartialEq, Eq, Default, serde::Serialize, serde::Deserialize)]
pub struct DonePages {
    pub tasks: Vec<TaskDto>,
    /// Filas de la última página pedida (antes de descartar duplicados); `None` si no hubo ninguna.
    pub last_page_len: Option<usize>,
}

/// Une las tareas activas con las finalizadas ya cargadas (función pura). Si una tarea aparece
/// en ambas listas (cambió de estado entre dos cargas) prevalece la activa; los duplicados
/// dentro de las finalizadas se descartan.
pub fn combine_tasks(active: &[TaskDto], done: &[TaskDto]) -> Vec<TaskDto> {
    let mut seen: HashSet<&str> = HashSet::new();
    active
        .iter()
        .chain(done.iter())
        .filter(|task: &&TaskDto| seen.insert(task.id.as_str()))
        .cloned()
        .collect()
}

/// Recorre las primeras `pages` páginas de finalizadas con `fetch(página)`, parando en la
/// primera incompleta. El desplazamiento puede repetir filas entre páginas si el conjunto cambia
/// mientras se carga, por eso se descartan los ids ya vistos.
async fn collect_done_pages<F, Fut>(pages: usize, mut fetch: F) -> Result<DonePages, ServerFnError>
where
    F: FnMut(usize) -> Fut,
    Fut: Future<Output = Result<Vec<TaskDto>, ServerFnError>>,
{
    let mut done: Vec<TaskDto> = Vec::new();
    let mut seen: HashSet<String> = HashSet::new();
    let mut last_page_len: Option<usize> = None;
    for page in 0..pages {
        let batch: Vec<TaskDto> = fetch(page).await?;
        let complete: bool = batch.len() >= DONE_PAGE_SIZE;
        last_page_len = Some(batch.len());
        done.extend(
            batch
                .into_iter()
                .filter(|task: &TaskDto| seen.insert(task.id.clone())),
        );
        if !complete {
            break;
        }
    }
    Ok(DonePages {
        tasks: done,
        last_page_len,
    })
}

/// Carga las primeras `pages` páginas de tareas finalizadas.
async fn fetch_done_pages(pages: usize) -> Result<DonePages, ServerFnError> {
    collect_done_pages(pages, |page: usize| {
        list_done_tasks(None, DONE_PAGE_SIZE as i64, done_page_offset(page))
    })
    .await
}
/// Texto del aviso cuando la tarea se crea pero no puede iniciarse.
pub fn start_failed_after_create_text(error: &str) -> String {
    START_FAILED_AFTER_CREATE.fmt(&[("error", error)])
}

/// Construye el DTO de alta con las imágenes adjuntas (función pura). `limits` es el texto de
/// los campos (RAM MiB, CPU %).
// Un argumento por campo del formulario de alta; agruparlos solo movería el problema.
#[allow(clippy::too_many_arguments)]
pub fn build_create_task_dto(
    workspace_id: String,
    title: String,
    prompt: String,
    priority: &str,
    (memory_limit, cpu_limit): (&str, &str),
    start_now: bool,
    base_branch: &str,
    images: Vec<ImageAttachmentDto>,
) -> CreateTaskDto {
    CreateTaskDto {
        workspace_id,
        title,
        prompt,
        priority: TaskPriorityDto::from_value(priority).unwrap_or_default(),
        memory_limit_mb: parse_positive_number(memory_limit),
        cpu_limit_percent: parse_positive_number(cpu_limit),
        start_now,
        base_branch: Some(base_branch.trim().to_string()).filter(|b: &String| !b.is_empty()),
        images,
    }
}

/// Límites (RAM, CPU) que se envían al crear: con la sección avanzada oculta los campos no
/// aplican y se usan los valores por defecto de ajustes (texto vacío).
pub fn effective_limits(show_advanced: bool, memory: &str, cpu: &str) -> (String, String) {
    if show_advanced {
        (memory.to_string(), cpu.to_string())
    } else {
        (String::new(), String::new())
    }
}

/// Nombre de la rama actual (HEAD) entre las ramas listadas, si hay alguna.
pub fn current_branch(branches: &[BranchOptionDto]) -> Option<String> {
    branches
        .iter()
        .find(|b: &&BranchOptionDto| b.current)
        .map(|b: &BranchOptionDto| b.name.clone())
}

const LINK_FAILED_AFTER_CREATE: Msg = Msg {
    en: "Task created, but its dependency could not be added: {error}",
    es: "Tarea creada, pero no se pudo añadir su dependencia: {error}",
    zh: "任务已创建，但无法添加其依赖：{error}",
    de: "Aufgabe erstellt, aber die Abhängigkeit konnte nicht hinzugefügt werden: {error}",
    fr: "Tâche créée, mais sa dépendance n'a pas pu être ajoutée : {error}",
};

/// Texto del aviso cuando la tarea se crea pero alguna de sus aristas falla.
pub fn link_failed_after_create_text(error: &str) -> String {
    LINK_FAILED_AFTER_CREATE.fmt(&[("error", error)])
}

/// Petición de alta desde el tablero: la tarea y las predecesoras de las que depende.
#[derive(Debug, Clone, PartialEq)]
pub struct LinkedCreate {
    pub input: CreateTaskDto,
    pub predecessors: Vec<String>,
}

/// Resultado del alta: la tarea creada y los errores de las aristas que no se pudieron crear.
#[derive(Debug, Clone, PartialEq)]
pub struct LinkedCreated {
    pub created: CreatedTaskDto,
    pub link_errors: Vec<String>,
}

/// Crea la tarea y, solo si se creó, una arista por cada predecesora. Una arista rechazada no
/// deshace la tarea: se acumula su error para avisar.
async fn create_with_links<C, CFut, L, LFut>(
    request: LinkedCreate,
    create: C,
    mut link: L,
) -> Result<LinkedCreated, ServerFnError>
where
    C: FnOnce(CreateTaskDto) -> CFut,
    CFut: Future<Output = Result<CreatedTaskDto, ServerFnError>>,
    L: FnMut(String, String) -> LFut,
    LFut: Future<Output = Result<(), ServerFnError>>,
{
    let created: CreatedTaskDto = create(request.input).await?;
    let mut link_errors: Vec<String> = Vec::new();
    for (predecessor, successor) in edges_for_new_task(&created.task.id, &request.predecessors) {
        if let Err(error) = link(predecessor, successor).await {
            link_errors.push(error.to_string());
        }
    }
    Ok(LinkedCreated {
        created,
        link_errors,
    })
}

/// Estado de la pantalla del tablero de un workspace: grafo, lienzo (vista, gestos,
/// selección), acciones sobre tareas y aristas, y el modal de alta de tarea.
#[derive(Clone, Copy)]
pub struct BoardState {
    /// Almacén compartido (shell): tareas activas, finalizadas visibles y workspaces.
    pub store: TaskStore,
    /// Workspace que muestra la pestaña (el de la ruta o, en `/`, el primero por nombre).
    pub workspace: Memo<BoardWorkspace>,
    /// Id del workspace mostrado, si ya se conoce.
    pub workspace_id: Memo<Option<String>>,
    /// Lista de workspaces (conserva la última carga mientras se recarga).
    pub workspace_options: Memo<Vec<WorkspaceOptionDto>>,
    /// Carga de las finalizadas; su resultado se vuelca en `store.done`, que los eventos de
    /// SSE parchean sin recargar páginas.
    pub done_tasks: Resource<Result<DonePages, ServerFnError>>,
    /// Si la última página de finalizadas pedida al servidor vino completa.
    pub done_has_more: RwSignal<bool>,
    /// Páginas de finalizadas que se piden al servidor ("Cargar más" la incrementa).
    pub done_pages: RwSignal<usize>,
    /// Mostrar las tareas finalizadas en el lienzo.
    pub show_completed: RwSignal<bool>,
    /// Grafo guardado del workspace; se recarga tras cada acción de arista/posición y cuando
    /// cambia el estado de alguna de sus tareas.
    pub graph: Resource<Result<TaskGraphDto, ServerFnError>>,
    /// Último grafo cargado con éxito (se conserva mientras se recarga).
    pub graph_data: RwSignal<TaskGraphDto>,
    pub graph_loaded: RwSignal<bool>,
    /// Nodos y aristas visibles.
    pub view: Memo<GraphView>,
    /// Tareas sueltas (sin aristas ni posición) que muestra el panel izquierdo.
    pub loose: Memo<Vec<TaskDto>>,
    /// Subpestaña elegida en pantallas estrechas; `None` = la que corresponda por defecto.
    pub board_tab: RwSignal<Option<BoardTab>>,
    /// Subpestaña visible en pantallas estrechas.
    pub active_tab: Memo<BoardTab>,
    /// Panel de tareas sueltas plegado.
    pub panel_collapsed: RwSignal<bool>,
    /// Tarea suelta que se está arrastrando hacia el lienzo.
    pub loose_drag: RwSignal<Option<LooseDrag>>,
    /// Posición efectiva de cada nodo (guardada, arrastrada o calculada).
    pub positions: Memo<HashMap<String, Point>>,
    /// Posiciones fijadas en esta sesión (mientras se arrastra y hasta que el servidor las confirma).
    overrides: RwSignal<HashMap<String, Point>>,
    pub viewport: RwSignal<Viewport>,
    pub drag: RwSignal<DragMode>,
    pub selected_node: RwSignal<Option<String>>,
    /// Arista seleccionada: (predecesora, sucesora).
    pub selected_edge: RwSignal<Option<(String, String)>>,
    pub add_edge: ServerAction<AddTaskDependency>,
    pub remove_edge: ServerAction<RemoveTaskDependency>,
    pub set_position: ServerAction<SetTaskPosition>,
    /// Tarea de la última posición enviada. `Action::input()` vuelve a `None` al terminar la
    /// acción, así que el deshacer de un guardado fallido no puede leerla de ahí. Un `Action`
    /// solo conserva el resultado del último envío, que es el que corresponde a este id.
    pub position_target: RwSignal<Option<String>>,
    pub create: Action<LinkedCreate, Result<LinkedCreated, ServerFnError>>,
    pub delete: ServerAction<DeleteTask>,
    pub start: ServerAction<StartTask>,
    pub finish: ServerAction<FinishTask>,
    pub merge: ServerAction<MergeTask>,
    pub modal_open: RwSignal<bool>,
    pub form_workspace_id: RwSignal<String>,
    pub form_title: RwSignal<String>,
    pub form_prompt: RwSignal<String>,
    /// Valor serializado de `TaskPriorityDto` (alimenta un `SelectField`).
    pub form_priority: RwSignal<String>,
    pub form_memory_limit: RwSignal<String>,
    pub form_cpu_limit: RwSignal<String>,
    /// Muestra las opciones avanzadas (límites de RAM y CPU) en el alta.
    pub form_show_advanced: RwSignal<bool>,
    pub form_start_now: RwSignal<bool>,
    /// Si la tarea nueva se enlaza a otra («siguiente»/«paralela»): fuerza `start_now = false`
    /// y el workspace del tablero.
    pub form_link: RwSignal<Option<NewTaskLink>>,
    /// Rama de la que parte el worktree; vacío = base del workspace.
    pub form_base_branch: RwSignal<String>,
    /// Ramas del workspace elegido en el formulario (vacías mientras no haya ninguno).
    pub branches: Resource<Result<Vec<BranchOptionDto>, ServerFnError>>,
    /// Imágenes adjuntas al prompt inicial.
    pub images: ImageAttachments,
    pub toasts: RwSignal<Vec<ToastMessage>>,
    toast_seq: RwSignal<u32>,
    pub start_target: RwSignal<Option<String>>,
    pub delete_target: RwSignal<Option<String>>,
    pub finish_target: RwSignal<Option<String>>,
    pub merge_target: RwSignal<Option<String>>,
}

/// Firma de los estados de las tareas de `workspace_id`: cambia cuando alguna tarea del
/// workspace aparece, desaparece o cambia de estado, y es la clave de recarga del grafo.
pub fn workspace_status_signature(
    workspace_id: &str,
    active: &[TaskDto],
    done: &[TaskDto],
) -> String {
    let mut entries: Vec<String> = combine_tasks(active, done)
        .iter()
        .filter(|t: &&TaskDto| t.workspace_id == workspace_id)
        .map(|t: &TaskDto| format!("{}:{:?}", t.id, t.status))
        .collect();
    entries.sort();
    entries.join(",")
}

impl BoardState {
    pub fn new() -> Self {
        let add_edge: ServerAction<AddTaskDependency> = ServerAction::new();
        let remove_edge: ServerAction<RemoveTaskDependency> = ServerAction::new();
        let set_position: ServerAction<SetTaskPosition> = ServerAction::new();
        let create: Action<LinkedCreate, Result<LinkedCreated, ServerFnError>> =
            Action::new(|request: &LinkedCreate| {
                let request: LinkedCreate = request.clone();
                async move {
                    create_with_links(
                        request,
                        create_task,
                        |predecessor: String, successor: String| async move {
                            add_task_dependency(predecessor, successor)
                                .await
                                .map(|_: TaskDependencyDto| ())
                        },
                    )
                    .await
                }
            });
        let delete: ServerAction<DeleteTask> = ServerAction::new();
        let start: ServerAction<StartTask> = ServerAction::new();
        let finish: ServerAction<FinishTask> = ServerAction::new();
        let merge: ServerAction<MergeTask> = ServerAction::new();
        let store: TaskStore = use_task_store();
        let params = use_tab_params();

        // El tablero necesita también las finalizadas (una predecesora integrada es un nodo
        // más del grafo) y, al salir, deja de parchearlas.
        store.show_done.set(true);
        on_cleanup(move || store.show_done.set(false));
        let done_pages: RwSignal<usize> = RwSignal::new(1);
        // Crear una tarea en "Por hacer" no emite evento SSE (start/finish/merge/delete sí),
        // así que el alta recarga el almacén. Solo mueve un contador (no llama al backend);
        // la primera ejecución, al montar, se omite.
        Effect::new(move |initialized: Option<bool>| {
            create.version().track();
            if initialized == Some(true) {
                store.reload_all();
            }
            true
        });
        let done_tasks: Resource<Result<DonePages, ServerFnError>> = Resource::new(
            move || {
                (
                    store.show_done.get(),
                    done_pages.get(),
                    store.reload_count(),
                )
            },
            |(show, pages, _)| async move {
                if show {
                    fetch_done_pages(pages).await
                } else {
                    Ok(DonePages::default())
                }
            },
        );
        // Vuelca la carga al almacén (solo cliente); no coordina llamadas al backend.
        let has_more: RwSignal<bool> = RwSignal::new(false);
        Effect::new(move |_| {
            if let Some(Ok(loaded)) = done_tasks.get() {
                has_more.set(done_has_more(loaded.last_page_len));
                store.done.set(loaded.tasks);
            }
        });

        let workspace_list: Memo<Option<Vec<WorkspaceOptionDto>>> =
            Memo::new(move |previous: Option<&Option<Vec<WorkspaceOptionDto>>>| {
                match store.workspaces.get() {
                    Some(Ok(list)) => Some(list),
                    _ => previous.cloned().flatten(),
                }
            });
        let workspace_options: Memo<Vec<WorkspaceOptionDto>> =
            Memo::new(move |_| workspace_list.get().unwrap_or_default());
        let route_workspace: Memo<Option<String>> =
            Memo::new(move |_| params.with(|p| p.get("workspace_id")));
        let workspace: Memo<BoardWorkspace> = Memo::new(move |_| {
            workspace_list.with(|list: &Option<Vec<WorkspaceOptionDto>>| {
                resolve_board_workspace(route_workspace.get().as_deref(), list.as_deref())
            })
        });
        let workspace_id: Memo<Option<String>> = Memo::new(move |_| match workspace.get() {
            BoardWorkspace::Selected(id) => Some(id),
            _ => None,
        });

        let status_signature: Memo<String> = Memo::new(move |_| {
            let Some(id): Option<String> = workspace_id.get() else {
                return String::new();
            };
            store.active.with(|active: &Vec<TaskDto>| {
                store
                    .done
                    .with(|done: &Vec<TaskDto>| workspace_status_signature(&id, active, done))
            })
        });
        // Cualquier acción de arista/posición y cualquier cambio de estado de una tarea del
        // workspace (p. ej. una sucesora que el servidor arranca sola) recargan el grafo.
        let graph: Resource<Result<TaskGraphDto, ServerFnError>> = Resource::new(
            move || {
                (
                    workspace_id.get(),
                    add_edge.version().get(),
                    remove_edge.version().get(),
                    set_position.version().get(),
                    // El alta crea sus aristas después de la tarea: se recarga al terminar.
                    create.version().get(),
                    status_signature.get(),
                )
            },
            |(workspace_id, ..)| async move {
                match workspace_id {
                    Some(id) => get_task_graph(id).await,
                    None => Ok(TaskGraphDto::default()),
                }
            },
        );
        let graph_data: RwSignal<TaskGraphDto> = RwSignal::new(TaskGraphDto::default());
        let graph_loaded: RwSignal<bool> = RwSignal::new(false);
        let overrides: RwSignal<HashMap<String, Point>> = RwSignal::new(HashMap::new());
        // Al cambiar de workspace no sobreviven el grafo ni las posiciones del anterior.
        Effect::new(move |previous: Option<Option<String>>| {
            let current: Option<String> = workspace_id.get();
            if previous.is_some_and(|before: Option<String>| before != current) {
                graph_data.set(TaskGraphDto::default());
                graph_loaded.set(false);
                overrides.set(HashMap::new());
            }
            current
        });
        // Copia el resultado a una señal propia (solo cliente) para conservar el grafo anterior
        // mientras se recarga; no coordina llamadas al backend. Las posiciones que el servidor
        // ya devuelve dejan de ser provisionales.
        Effect::new(move |_| {
            if let Some(Ok(loaded)) = graph.get() {
                let saved: HashMap<String, Point> = saved_positions(&loaded);
                let pending: HashMap<String, Point> =
                    overrides.with_untracked(|current: &HashMap<String, Point>| {
                        unconfirmed(current, &saved)
                    });
                if overrides.with_untracked(|current: &HashMap<String, Point>| {
                    current.len() != pending.len()
                }) {
                    overrides.set(pending);
                }
                if graph_data.with_untracked(|current: &TaskGraphDto| *current != loaded) {
                    graph_data.set(loaded);
                }
                graph_loaded.set(true);
            }
        });

        let show_completed: RwSignal<bool> = RwSignal::new(true);
        // Tareas con posición recién fijada que el servidor aún no ha devuelto: ya no están
        // sueltas. Memo aparte para no reconstruir el grafo en cada movimiento de un nodo.
        let placed: Memo<HashSet<String>> = Memo::new(move |_| {
            overrides.with(|m: &HashMap<String, Point>| m.keys().cloned().collect())
        });
        let workspace_tasks: Memo<Vec<TaskDto>> = Memo::new(move |_| {
            store.active.with(|active: &Vec<TaskDto>| {
                store
                    .done
                    .with(|done: &Vec<TaskDto>| combine_tasks(active, done))
            })
        });
        let loose: Memo<Vec<TaskDto>> = Memo::new(move |_| {
            let Some(id): Option<String> = workspace_id.get() else {
                return Vec::new();
            };
            workspace_tasks.with(|tasks: &Vec<TaskDto>| {
                graph_data.with(|graph: &TaskGraphDto| {
                    placed.with(|placed: &HashSet<String>| {
                        loose_tasks(&id, tasks, graph, show_completed.get(), placed)
                    })
                })
            })
        });
        let board_tab: RwSignal<Option<BoardTab>> = RwSignal::new(None);
        let view: Memo<GraphView> = Memo::new(move |_| {
            let Some(id): Option<String> = workspace_id.get() else {
                return GraphView::default();
            };
            workspace_tasks.with(|tasks: &Vec<TaskDto>| {
                graph_data.with(|graph: &TaskGraphDto| {
                    placed.with(|placed: &HashSet<String>| {
                        build_graph(&id, tasks, graph, show_completed.get(), placed)
                    })
                })
            })
        });
        let positions: Memo<HashMap<String, Point>> = Memo::new(move |_| {
            view.with(|graph: &GraphView| {
                let ids: Vec<String> = graph.nodes.iter().map(|n| n.task.id.clone()).collect();
                let edges: Vec<(String, String)> = graph
                    .edges
                    .iter()
                    .map(|e| (e.predecessor_id.clone(), e.successor_id.clone()))
                    .collect();
                let mut saved: HashMap<String, Point> = graph_data.with(saved_positions);
                saved.extend(overrides.get());
                resolve_positions(&ids, &edges, &saved)
            })
        });

        let active_tab: Memo<BoardTab> = Memo::new(move |_| {
            board_tab.get().unwrap_or_else(|| {
                default_board_tab(
                    view.with(|g: &GraphView| g.nodes.len()),
                    loose.with(Vec::len),
                )
            })
        });

        let form_workspace_id: RwSignal<String> = RwSignal::new(String::new());
        let form_base_branch: RwSignal<String> = RwSignal::new(String::new());
        let branches: Resource<Result<Vec<BranchOptionDto>, ServerFnError>> = Resource::new(
            move || form_workspace_id.get(),
            |workspace_id: String| async move {
                if workspace_id.is_empty() {
                    Ok(Vec::new())
                } else {
                    list_workspace_branches(workspace_id).await
                }
            },
        );
        // Una rama elegida para otro workspace ya no vale.
        Effect::new(move |_| {
            form_workspace_id.track();
            form_base_branch.set(String::new());
        });
        // Por defecto, la rama actual del repositorio del workspace elegido.
        Effect::new(move |_| {
            if let Some(Ok(loaded)) = branches.get() {
                form_base_branch.set(current_branch(&loaded).unwrap_or_default());
            }
        });

        Self {
            store,
            workspace,
            workspace_id,
            workspace_options,
            done_tasks,
            done_has_more: has_more,
            done_pages,
            show_completed,
            graph,
            graph_data,
            graph_loaded,
            view,
            loose,
            board_tab,
            active_tab,
            panel_collapsed: RwSignal::new(false),
            loose_drag: RwSignal::new(None),
            positions,
            overrides,
            viewport: RwSignal::new(Viewport::default()),
            drag: RwSignal::new(DragMode::Idle),
            selected_node: RwSignal::new(None),
            selected_edge: RwSignal::new(None),
            add_edge,
            remove_edge,
            set_position,
            position_target: RwSignal::new(None),
            create,
            delete,
            start,
            finish,
            merge,
            modal_open: RwSignal::new(false),
            form_workspace_id,
            form_title: RwSignal::new(String::new()),
            form_prompt: RwSignal::new(String::new()),
            form_priority: RwSignal::new(TaskPriorityDto::default().as_str().to_string()),
            form_memory_limit: RwSignal::new(String::new()),
            form_cpu_limit: RwSignal::new(String::new()),
            form_show_advanced: RwSignal::new(false),
            form_start_now: RwSignal::new(false),
            form_link: RwSignal::new(None),
            form_base_branch,
            branches,
            images: ImageAttachments::new(),
            toasts: RwSignal::new(Vec::new()),
            toast_seq: RwSignal::new(0),
            start_target: RwSignal::new(None),
            delete_target: RwSignal::new(None),
            finish_target: RwSignal::new(None),
            merge_target: RwSignal::new(None),
        }
    }

    /// Fija la posición de `id` en el lienzo y la guarda: la tarea pasa a ser un nodo del grafo.
    fn place_task(&self, id: String, position: Point) {
        self.overrides
            .update(|m: &mut HashMap<String, Point>| *m = place(m, &id, position));
        self.save_position(id, position);
    }

    /// Envía la posición al servidor recordando la tarea afectada para poder deshacer el cambio.
    fn save_position(&self, id: String, position: Point) {
        self.position_target.set(Some(id.clone()));
        self.set_position.dispatch(SetTaskPosition {
            id,
            x: position.x,
            y: position.y,
        });
    }

    // ── Panel de tareas sueltas ──────────────────────────────────────────────────────────

    /// Empieza a arrastrar la tarea suelta `id` (`client`: coordenadas de ventana).
    pub fn loose_pointer_down(&self, id: String, client: Point) {
        self.loose_drag.set(Some(LooseDrag {
            task_id: id,
            pointer: client,
        }));
    }

    pub fn loose_pointer_move(&self, client: Point) {
        self.loose_drag.update(|drag: &mut Option<LooseDrag>| {
            if let Some(drag) = drag {
                drag.pointer = client;
            }
        });
    }

    /// Suelta la tarea arrastrada: si el puntero está sobre el lienzo (`canvas`, rectángulo en
    /// coordenadas de ventana) pasa a ser un nodo en esa posición.
    pub fn loose_pointer_up(&self, client: Point, canvas: Option<Rect>) {
        let Some(drag) = self.loose_drag.get_untracked() else {
            return;
        };
        self.loose_drag.set(None);
        let target: Option<Point> = canvas
            .and_then(|rect: Rect| drop_position(client, &rect, &self.viewport.get_untracked()));
        if let Some(position) = target
            && self.is_still_loose(&drag.task_id)
        {
            self.place_task(drag.task_id, position);
        }
    }

    /// Gesto cancelado por el navegador: la tarea sigue suelta.
    pub fn loose_pointer_cancel(&self) {
        self.loose_drag.set(None);
    }

    /// Añade la tarea suelta `id` al grafo, en el centro visible del lienzo de `size`.
    pub fn add_loose_to_graph(&self, id: String, size: (f64, f64)) {
        if !self.is_still_loose(&id) {
            return;
        }
        let position: Point = centre_position(size, &self.viewport.get_untracked());
        self.place_task(id, position);
        // En pantallas estrechas el lienzo no se ve: se enseña el resultado.
        self.board_tab.set(Some(BoardTab::Graph));
    }

    /// La tarea sigue en el panel de sueltas (no se ha colocado ni borrado mientras tanto).
    fn is_still_loose(&self, id: &str) -> bool {
        self.loose
            .with_untracked(|loose: &Vec<TaskDto>| can_place(id, loose))
    }

    /// Elige la subpestaña visible en pantallas estrechas.
    pub fn select_tab(&self, tab: BoardTab) {
        self.board_tab.set(Some(tab));
    }

    /// Nodos con su posición, en orden de pintado (para localizar el nodo bajo el puntero).
    fn node_points(&self) -> Vec<(String, Point)> {
        let positions: HashMap<String, Point> = self.positions.get_untracked();
        self.view.with_untracked(|graph: &GraphView| {
            graph
                .nodes
                .iter()
                .filter_map(|n| {
                    positions
                        .get(&n.task.id)
                        .map(|p: &Point| (n.task.id.clone(), *p))
                })
                .collect()
        })
    }

    // ── Gestos del lienzo ────────────────────────────────────────────────────────────────

    /// Empieza a arrastrar el fondo (`screen`: punto relativo al lienzo).
    pub fn pointer_down_background(&self, screen: Point) {
        self.drag.set(DragMode::pan(screen));
    }

    /// Empieza a arrastrar el nodo `id`.
    pub fn pointer_down_node(&self, id: String, screen: Point) {
        let start: Option<Point> = self
            .positions
            .with_untracked(|p: &HashMap<String, Point>| p.get(&id).copied());
        if let Some(start) = start {
            self.drag.set(DragMode::node(
                id,
                screen,
                start,
                &self.viewport.get_untracked(),
            ));
        }
    }

    /// Empieza una arista desde el puerto de salida del nodo `id`.
    pub fn pointer_down_port(&self, id: String, screen: Point) {
        self.drag.set(DragMode::connect(
            id,
            screen,
            &self.viewport.get_untracked(),
        ));
    }

    pub fn pointer_move(&self, screen: Point) {
        let mode: DragMode = self.drag.get_untracked();
        if mode.is_idle() {
            return;
        }
        let (next, effect): (DragMode, DragEffect) =
            mode.moved(screen, &self.viewport.get_untracked(), &self.node_points());
        self.drag.set(next);
        match effect {
            DragEffect::None => {}
            DragEffect::SetViewport(viewport) => self.viewport.set(viewport),
            DragEffect::MoveNode(id, position) => {
                self.overrides.update(|m: &mut HashMap<String, Point>| {
                    m.insert(id, position);
                });
            }
        }
    }

    pub fn pointer_up(&self, screen: Point) {
        let mode: DragMode = self.drag.get_untracked();
        self.drag.set(DragMode::Idle);
        let positions: HashMap<String, Point> = self.positions.get_untracked();
        let outcome: DragOutcome = mode.finished(
            screen,
            &self.viewport.get_untracked(),
            &self.node_points(),
            |id: &str| positions.get(id).copied(),
        );
        match outcome {
            DragOutcome::None => {}
            DragOutcome::ClearSelection => self.clear_selection(),
            DragOutcome::SelectNode(id) => {
                self.selected_edge.set(None);
                self.selected_node.set(Some(id));
            }
            DragOutcome::CommitNode(id, position) => self.save_position(id, position),
            DragOutcome::Connect { from, to } => {
                self.add_edge.dispatch(AddTaskDependency {
                    predecessor_id: from,
                    successor_id: to,
                });
            }
        }
    }

    /// Gesto cancelado por el navegador: se abandona sin guardar nada.
    pub fn pointer_cancel(&self) {
        self.drag.set(DragMode::Idle);
    }

    /// Zoom con la rueda, centrado en el cursor.
    pub fn wheel(&self, screen: Point, delta_y: f64) {
        let factor: f64 = wheel_zoom_factor(delta_y);
        self.viewport
            .update(|v: &mut Viewport| *v = v.zoomed_at(screen, factor));
    }

    /// Acerca o aleja un paso, centrado en `center`.
    pub fn zoom_step(&self, zoom_in: bool, center: Point) {
        let factor: f64 = if zoom_in { ZOOM_STEP } else { 1.0 / ZOOM_STEP };
        self.viewport
            .update(|v: &mut Viewport| *v = v.zoomed_at(center, factor));
    }

    /// Encuadra todos los nodos en un lienzo de `size` píxeles.
    pub fn fit(&self, size: (f64, f64)) {
        let points: Vec<Point> = self
            .positions
            .with_untracked(|p: &HashMap<String, Point>| p.values().copied().collect());
        self.viewport
            .set(fit_viewport(bounds_of_nodes(&points), size));
    }

    pub fn select_edge(&self, predecessor_id: String, successor_id: String) {
        self.selected_node.set(None);
        self.selected_edge.set(Some((predecessor_id, successor_id)));
    }

    pub fn clear_selection(&self) {
        self.selected_node.set(None);
        self.selected_edge.set(None);
    }

    /// Borra la arista seleccionada.
    pub fn delete_selected_edge(&self) {
        if let Some((predecessor_id, successor_id)) = self.selected_edge.get_untracked() {
            self.remove_edge.dispatch(RemoveTaskDependency {
                predecessor_id,
                successor_id,
            });
            self.selected_edge.set(None);
        }
    }

    // ── Alta de tareas ───────────────────────────────────────────────────────────────────

    /// Abre el modal de alta con el formulario vacío y el workspace del tablero preseleccionado.
    pub fn open_create(&self) {
        self.reset_form();
        self.form_link.set(None);
        self.modal_open.set(true);
    }

    /// Abre el modal de alta de una tarea enlazada a otra («siguiente» o «paralela»): no puede
    /// iniciarse al crearla porque espera a sus predecesoras.
    pub fn open_create_linked(&self, link: NewTaskLink) {
        self.reset_form();
        self.form_link.set(Some(link));
        self.modal_open.set(true);
    }

    fn reset_form(&self) {
        self.create.clear();
        self.form_workspace_id
            .set(self.workspace_id.get_untracked().unwrap_or_default());
        self.form_title.set(String::new());
        self.form_prompt.set(String::new());
        self.form_priority
            .set(TaskPriorityDto::default().as_str().to_string());
        self.form_memory_limit.set(String::new());
        self.form_cpu_limit.set(String::new());
        self.form_show_advanced.set(false);
        self.form_start_now.set(false);
        self.form_base_branch.set(String::new());
        self.images.clear();
    }

    pub fn close_modal(&self) {
        self.modal_open.set(false);
        self.images.clear();
    }

    /// Envía el formulario de alta.
    pub fn submit(&self) {
        // Con la sección avanzada oculta los límites no aplican: se usan los de ajustes.
        let (memory_limit, cpu_limit): (String, String) = effective_limits(
            self.form_show_advanced.get_untracked(),
            &self.form_memory_limit.get_untracked(),
            &self.form_cpu_limit.get_untracked(),
        );
        let link: Option<NewTaskLink> = self.form_link.get_untracked();
        let predecessors: Vec<String> = link
            .as_ref()
            .map(|link: &NewTaskLink| {
                self.graph_data
                    .with_untracked(|g: &TaskGraphDto| link_predecessors(link, &g.dependencies))
            })
            .unwrap_or_default();
        self.create.dispatch(LinkedCreate {
            input: build_create_task_dto(
                self.form_workspace_id.get_untracked(),
                self.form_title.get_untracked(),
                self.form_prompt.get_untracked(),
                &self.form_priority.get_untracked(),
                (&memory_limit, &cpu_limit),
                start_now_for(link.as_ref(), self.form_start_now.get_untracked()),
                &self.form_base_branch.get_untracked(),
                self.images.images.get_untracked(),
            ),
            predecessors,
        });
    }

    /// Pide la siguiente página de tareas finalizadas.
    pub fn load_more_done(&self) {
        self.done_pages.update(|pages: &mut usize| *pages += 1);
    }

    pub fn start_task(&self, id: String) {
        self.start_target.set(Some(id.clone()));
        self.start.dispatch(StartTask { id });
    }

    pub fn delete_task(&self, id: String) {
        self.delete_target.set(Some(id.clone()));
        self.delete.dispatch(DeleteTask { id });
    }

    pub fn finish_task(&self, id: String) {
        self.finish_target.set(Some(id.clone()));
        self.finish.dispatch(FinishTask { id });
    }

    pub fn merge_task(&self, id: String) {
        self.merge_target.set(Some(id.clone()));
        self.merge.dispatch(MergeTask { id });
    }

    // ── Notificaciones ───────────────────────────────────────────────────────────────────

    /// Convierte cada error de una acción en una notificación descartable y avisa de los
    /// avisos del alta. Solo toca estado de UI; no llama al backend.
    pub fn watch_action_errors(&self) {
        let state: Self = *self;
        Effect::new(move |_| {
            if let Some(Ok(done)) = state.create.value().get() {
                if let Some(message) = done.created.start_error {
                    state.notify_start_failed_after_create(&message);
                }
                for error in done.link_errors {
                    state.push(link_failed_after_create_text(&error));
                }
            }
        });
        Effect::new(move |_| {
            if let Some(Err(e)) = state.start.value().get() {
                state.notify_error(state.start_target.get_untracked(), e.to_string());
            }
        });
        Effect::new(move |_| {
            if let Some(Err(e)) = state.delete.value().get() {
                state.notify_error(state.delete_target.get_untracked(), e.to_string());
            }
        });
        Effect::new(move |_| {
            if let Some(Err(e)) = state.finish.value().get() {
                state.notify_error(state.finish_target.get_untracked(), e.to_string());
            }
        });
        Effect::new(move |_| {
            if let Some(Err(e)) = state.merge.value().get() {
                state.notify_error(state.merge_target.get_untracked(), e.to_string());
            }
        });
        // Una arista rechazada (ciclo, duplicada, otro workspace…) no cambia el grafo.
        Effect::new(move |_| {
            if let Some(Err(e)) = state.add_edge.value().get() {
                state.push(e.to_string());
            }
        });
        Effect::new(move |_| {
            if let Some(Err(e)) = state.remove_edge.value().get() {
                state.push(e.to_string());
            }
        });
        // Si no se pudo guardar la posición, el nodo vuelve a la que tenía en el servidor. La
        // tarea afectada es la del último envío (`position_target`), que se corresponde con este
        // resultado porque la acción solo conserva el del último envío.
        Effect::new(move |_| {
            if let Some(Err(e)) = state.set_position.value().get() {
                if let Some(id) = state.position_target.get_untracked() {
                    state
                        .overrides
                        .update(|m: &mut HashMap<String, Point>| *m = roll_back(m, &id));
                }
                state.push(e.to_string());
            }
        });
    }

    fn push(&self, text: String) {
        push_toast(self.toasts, self.toast_seq, text);
    }

    /// Muestra un error de una acción del tablero como notificación, incluyendo el título
    /// de la tarea afectada cuando se conoce su id.
    pub fn notify_error(&self, task_id: Option<String>, message: String) {
        let title: Option<String> = task_id.as_deref().and_then(|id: &str| {
            self.store.active.with_untracked(|tasks: &Vec<TaskDto>| {
                tasks
                    .iter()
                    .find(|t: &&TaskDto| t.id == id)
                    .map(|t: &TaskDto| t.title.clone())
            })
        });
        let text: String = match title.or(task_id) {
            Some(label) => format!("{label}: {message}"),
            None => message,
        };
        self.push(text);
    }

    /// Avisa de que la tarea se creó en "Por hacer" pero su arranque falló.
    pub fn notify_start_failed_after_create(&self, error: &str) {
        self.push(start_failed_after_create_text(error));
    }

    pub fn dismiss_toast(&self, id: u32) {
        self.toasts
            .update(|list: &mut Vec<ToastMessage>| list.retain(|t: &ToastMessage| t.id != id));
    }
}

/// Una tarea enlazada a otras espera a sus predecesoras: nunca se inicia al crearla.
pub fn start_now_for(link: Option<&NewTaskLink>, requested: bool) -> bool {
    requested && link.is_none()
}

impl Default for BoardState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
        TaskStageDto, TaskStatusDto,
    };

    #[test]
    fn advanced_limits_are_sent_when_section_is_shown() {
        let limits: (String, String) = effective_limits(true, "2048", "50");
        assert_eq!(limits, ("2048".to_string(), "50".to_string()));
    }

    #[test]
    fn advanced_limits_are_dropped_when_section_is_hidden() {
        let limits: (String, String) = effective_limits(false, "2048", "50");
        assert_eq!(limits, (String::new(), String::new()));
    }

    #[test]
    fn create_dto_carries_images_and_priority() {
        let image: ImageAttachmentDto = ImageAttachmentDto {
            name: "a.png".to_string(),
            mime_type: "image/png".to_string(),
            data: "aGVsbG8=".to_string(),
        };
        let dto: CreateTaskDto = build_create_task_dto(
            "ws".to_string(),
            String::new(),
            "haz algo".to_string(),
            "high",
            ("2048", "0"),
            true,
            "origin/develop",
            vec![image.clone()],
        );
        assert_eq!(dto.memory_limit_mb, Some(2048));
        assert_eq!(dto.cpu_limit_percent, None);
        assert_eq!(dto.images, vec![image]);
        assert_eq!(dto.base_branch.as_deref(), Some("origin/develop"));
        assert_eq!(dto.priority, TaskPriorityDto::High);
        assert!(dto.start_now);
    }

    #[test]
    fn create_dto_leaves_the_base_branch_unset_when_blank() {
        let dto: CreateTaskDto = build_create_task_dto(
            "ws".to_string(),
            String::new(),
            "haz algo".to_string(),
            "normal",
            ("", ""),
            false,
            "  ",
            Vec::new(),
        );
        assert_eq!(dto.base_branch, None);
    }

    #[test]
    fn current_branch_is_the_one_flagged_as_head() {
        let branch = |name: &str, current: bool| BranchOptionDto {
            name: name.to_string(),
            remote: false,
            current,
        };
        let listed: Vec<BranchOptionDto> = vec![branch("main", false), branch("feat", true)];

        assert_eq!(current_branch(&listed), Some("feat".to_string()));
        assert_eq!(current_branch(&[branch("main", false)]), None);
    }

    #[test]
    fn done_paging_helpers() {
        assert_eq!(done_page_offset(0), 0);
        assert_eq!(done_page_offset(2), 100);
    }

    #[test]
    fn done_has_more_depends_only_on_last_fetched_page() {
        assert!(!done_has_more(None));
        assert!(!done_has_more(Some(0)));
        assert!(!done_has_more(Some(DONE_PAGE_SIZE - 1)));
        assert!(done_has_more(Some(DONE_PAGE_SIZE)));
    }

    #[test]
    fn combine_tasks_drops_ids_repeated_across_lists() {
        let active: Vec<TaskDto> = vec![task(
            "a",
            "ws-a",
            TaskStageDto::Working,
            "2026-01-01T00:00:00Z",
        )];
        let done: Vec<TaskDto> = vec![
            task("a", "ws-a", TaskStageDto::Done, "2026-01-01T00:00:00Z"),
            task("d", "ws-a", TaskStageDto::Done, "2026-01-01T00:00:00Z"),
            task("d", "ws-a", TaskStageDto::Done, "2026-01-01T00:00:00Z"),
        ];

        let combined: Vec<TaskDto> = combine_tasks(&active, &done);

        let ids: Vec<&str> = combined.iter().map(|t: &TaskDto| t.id.as_str()).collect();
        assert_eq!(ids, vec!["a", "d"]);
        assert_eq!(combined[0].stage, TaskStageDto::Working);
    }

    #[test]
    fn combine_tasks_appends_done_after_active() {
        let active: Vec<TaskDto> = vec![task(
            "a",
            "ws-a",
            TaskStageDto::Working,
            "2026-01-01T00:00:00Z",
        )];
        let done: Vec<TaskDto> = vec![task(
            "d",
            "ws-a",
            TaskStageDto::Done,
            "2026-01-01T00:00:00Z",
        )];

        let combined: Vec<TaskDto> = combine_tasks(&active, &done);

        let ids: Vec<&str> = combined.iter().map(|t: &TaskDto| t.id.as_str()).collect();
        assert_eq!(ids, vec!["a", "d"]);
    }

    #[test]
    fn status_signature_changes_with_the_workspace_task_states_only() {
        let a: TaskDto = task("a", "ws", TaskStageDto::Working, "1");
        let other: TaskDto = task("o", "other", TaskStageDto::Working, "1");
        let before: String = workspace_status_signature("ws", &[a.clone(), other.clone()], &[]);

        let mut changed_other: TaskDto = other.clone();
        changed_other.status = TaskStatusDto::Merged;
        assert_eq!(
            before,
            workspace_status_signature("ws", &[a.clone(), changed_other], &[])
        );

        let mut changed: TaskDto = a.clone();
        changed.status = TaskStatusDto::Merged;
        assert_ne!(before, workspace_status_signature("ws", &[changed], &[]));
        // El orden de las listas no importa.
        let b: TaskDto = task("b", "ws", TaskStageDto::Backlog, "2");
        assert_eq!(
            workspace_status_signature("ws", &[a.clone(), b.clone()], &[]),
            workspace_status_signature("ws", &[b], &[a])
        );
    }

    #[test]
    fn linked_tasks_are_never_started_on_creation() {
        let link: NewTaskLink = NewTaskLink::Next("a".to_string());
        assert!(!start_now_for(Some(&link), true));
        assert!(start_now_for(None, true));
        assert!(!start_now_for(None, false));
    }

    #[test]
    fn link_failure_text_carries_the_error() {
        assert_eq!(
            link_failed_after_create_text("cycle"),
            "Task created, but its dependency could not be added: cycle"
        );
    }

    fn created(id: &str) -> CreatedTaskDto {
        CreatedTaskDto {
            task: task(id, "ws", TaskStageDto::Backlog, "1"),
            start_error: None,
        }
    }

    fn request(predecessors: &[&str]) -> LinkedCreate {
        LinkedCreate {
            input: CreateTaskDto::default(),
            predecessors: predecessors
                .iter()
                .map(|p: &&str| (*p).to_string())
                .collect(),
        }
    }

    #[tokio::test]
    async fn create_with_links_adds_one_edge_per_predecessor_after_creating() {
        let linked: std::sync::Mutex<Vec<(String, String)>> = std::sync::Mutex::new(Vec::new());
        let result: Result<LinkedCreated, ServerFnError> = create_with_links(
            request(&["p", "q"]),
            |_input: CreateTaskDto| async { Ok(created("n")) },
            |p: String, s: String| {
                linked.lock().expect("lock").push((p, s));
                async { Ok(()) }
            },
        )
        .await;

        let done: LinkedCreated = result.expect("created");
        assert!(done.link_errors.is_empty());
        assert_eq!(done.created.task.id, "n");
        assert_eq!(
            *linked.lock().expect("lock"),
            vec![
                ("p".to_string(), "n".to_string()),
                ("q".to_string(), "n".to_string())
            ]
        );
    }

    #[tokio::test]
    async fn create_with_links_keeps_the_task_and_reports_rejected_edges() {
        let result: Result<LinkedCreated, ServerFnError> = create_with_links(
            request(&["p", "q"]),
            |_input: CreateTaskDto| async { Ok(created("n")) },
            |p: String, _s: String| async move {
                if p == "p" {
                    Err(ServerFnError::new("cycle"))
                } else {
                    Ok(())
                }
            },
        )
        .await;

        let done: LinkedCreated = result.expect("created");
        assert_eq!(done.link_errors.len(), 1);
        assert!(done.link_errors[0].contains("cycle"));
    }

    #[tokio::test]
    async fn create_with_links_creates_no_edges_when_creation_fails() {
        let calls: std::sync::atomic::AtomicUsize = std::sync::atomic::AtomicUsize::new(0);
        let result: Result<LinkedCreated, ServerFnError> = create_with_links(
            request(&["p"]),
            |_input: CreateTaskDto| async { Err(ServerFnError::new("boom")) },
            |_p: String, _s: String| {
                calls.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                async { Ok(()) }
            },
        )
        .await;

        assert!(result.is_err());
        assert_eq!(calls.load(std::sync::atomic::Ordering::SeqCst), 0);
    }

    fn full_page(prefix: &str) -> Vec<TaskDto> {
        (0..DONE_PAGE_SIZE)
            .map(|i: usize| {
                task(
                    &format!("{prefix}{i}"),
                    "ws-a",
                    TaskStageDto::Done,
                    "2026-01-01T00:00:00Z",
                )
            })
            .collect()
    }

    #[tokio::test]
    async fn collect_done_pages_stops_at_first_short_page() {
        let mut calls: Vec<usize> = Vec::new();
        let result: DonePages = collect_done_pages(5, |page: usize| {
            calls.push(page);
            let batch: Vec<TaskDto> = match page {
                0 => full_page("p0-"),
                _ => vec![task(
                    "last",
                    "ws-a",
                    TaskStageDto::Done,
                    "2026-01-01T00:00:00Z",
                )],
            };
            async move { Ok(batch) }
        })
        .await
        .expect("páginas");

        assert_eq!(calls, vec![0, 1]);
        assert_eq!(result.tasks.len(), DONE_PAGE_SIZE + 1);
        assert_eq!(result.last_page_len, Some(1));
        assert!(!done_has_more(result.last_page_len));
    }

    #[tokio::test]
    async fn collect_done_pages_dedupes_rows_repeated_between_pages() {
        let result: DonePages = collect_done_pages(2, |page: usize| {
            let mut batch: Vec<TaskDto> = full_page("p0-");
            if page == 1 {
                batch = vec![task(
                    "p0-0",
                    "ws-a",
                    TaskStageDto::Done,
                    "2026-01-01T00:00:00Z",
                )];
            }
            async move { Ok(batch) }
        })
        .await
        .expect("páginas");

        assert_eq!(result.tasks.len(), DONE_PAGE_SIZE);
    }

    #[tokio::test]
    async fn collect_done_pages_full_page_reports_more_and_empty_extra_page_does_not() {
        let full: DonePages = collect_done_pages(1, |_page: usize| async { Ok(full_page("p-")) })
            .await
            .expect("páginas");
        assert!(done_has_more(full.last_page_len));

        let extra_empty: DonePages = collect_done_pages(2, |page: usize| {
            let batch: Vec<TaskDto> = if page == 0 {
                full_page("p-")
            } else {
                Vec::new()
            };
            async move { Ok(batch) }
        })
        .await
        .expect("páginas");
        assert_eq!(extra_empty.tasks.len(), DONE_PAGE_SIZE);
        assert!(!done_has_more(extra_empty.last_page_len));

        let none: DonePages = collect_done_pages(0, |_page: usize| async { Ok(Vec::new()) })
            .await
            .expect("páginas");
        assert!(!done_has_more(none.last_page_len));
    }

    #[tokio::test]
    async fn collect_done_pages_propagates_errors() {
        let result: Result<DonePages, ServerFnError> =
            collect_done_pages(2, |_page: usize| async { Err(ServerFnError::new("boom")) }).await;

        assert!(result.is_err());
    }

    #[test]
    fn start_failure_after_create_text_mentions_backlog_and_error() {
        assert_eq!(
            start_failed_after_create_text("no branch"),
            "Task created in To do, but it could not be started: no branch"
        );
    }

    fn task(id: &str, workspace_id: &str, stage: TaskStageDto, updated_at: &str) -> TaskDto {
        TaskDto {
            id: id.to_string(),
            workspace_id: workspace_id.to_string(),
            title: format!("tarea {id}"),
            prompt: "prompt".to_string(),
            branch: "pi/x".to_string(),
            base_branch: "main".to_string(),
            worktree_path: "/wt".to_string(),
            status: crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStatusDto::Idle,
            priority: Default::default(),
            memory_limit_mb: None,
            cpu_limit_percent: None,
            error: None,
            resume_at: None,
            actions: Default::default(),
            stage,
            needs_attention: matches!(stage, TaskStageDto::Attention | TaskStageDto::Review),
            pending_dialogs: 0,
            created_at: updated_at.to_string(),
            updated_at: updated_at.to_string(),
        }
    }
}
