use leptos::prelude::*;

use crate::core::components::toast::{ToastMessage, push_toast};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    CreateTaskDto, TaskDto, TaskStageDto, UpdateTaskDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    CreateTask, DeleteTask, FinishTask, MergeTask, StartTask, UpdateTask, list_tasks,
    list_workspace_options,
};

/// Sondeo lento de respaldo: el tablero se actualiza sobre todo vía SSE.
#[cfg(feature = "hydrate")]
const FALLBACK_POLL_INTERVAL: std::time::Duration = std::time::Duration::from_secs(60);

/// Espera antes de reabrir el stream SSE cuando el navegador lo cierra por completo.
#[cfg(feature = "hydrate")]
const SSE_RECONNECT_DELAY: std::time::Duration = std::time::Duration::from_secs(3);

/// Una tarjeta más su orden relativo dentro del carril (menor = más prioritario).
fn stage_rank(stage: TaskStageDto) -> u8 {
    match stage {
        TaskStageDto::Attention => 0,
        TaskStageDto::Review => 1,
        TaskStageDto::Working => 2,
        TaskStageDto::Backlog => 3,
        TaskStageDto::Done => 4,
    }
}

/// Compara timestamps RFC 3339; ante un valor inválido lo trata como el más antiguo posible.
fn parse_updated_at(value: &str) -> chrono::DateTime<chrono::Utc> {
    chrono::DateTime::parse_from_rfc3339(value)
        .map(|dt: chrono::DateTime<chrono::FixedOffset>| dt.with_timezone(&chrono::Utc))
        .unwrap_or(chrono::DateTime::<chrono::Utc>::MIN_UTC)
}

/// Carril del tablero: todas las tareas visibles de un mismo workspace.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LaneView {
    pub workspace_id: String,
    pub workspace_name: String,
    pub base_branch: String,
    pub tasks: Vec<TaskDto>,
    pub total: usize,
    pub attention_count: usize,
}

/// Agrupa, filtra y ordena las tareas en carriles por workspace (función pura, sin I/O).
///
/// Reglas: un workspace solo aparece si tiene tareas visibles tras aplicar `stage_filter`
/// y `show_done`; dentro de un carril las tareas se ordenan por prioridad de etapa y luego
/// por `updated_at` descendente; los carriles con alguna tarea en atención van primero,
/// y el resto se ordena por nombre de workspace.
pub fn build_lanes(
    tasks: &[TaskDto],
    workspaces: &[WorkspaceOptionDto],
    stage_filter: Option<TaskStageDto>,
    show_done: bool,
) -> Vec<LaneView> {
    let visible: Vec<TaskDto> = tasks
        .iter()
        .filter(|t: &&TaskDto| {
            let matches_filter: bool = stage_filter.is_none_or(|s: TaskStageDto| t.stage == s);
            let matches_done: bool = t.stage != TaskStageDto::Done || show_done;
            matches_filter && matches_done
        })
        .cloned()
        .collect();

    let mut lanes: Vec<LaneView> = Vec::new();
    for task in visible {
        let workspace_id: String = task.workspace_id.clone();
        match lanes
            .iter_mut()
            .find(|lane: &&mut LaneView| lane.workspace_id == workspace_id)
        {
            Some(lane) => lane.tasks.push(task),
            None => {
                let workspace: Option<&WorkspaceOptionDto> = workspaces
                    .iter()
                    .find(|w: &&WorkspaceOptionDto| w.id == workspace_id);
                lanes.push(LaneView {
                    workspace_name: workspace
                        .map(|w: &WorkspaceOptionDto| w.name.clone())
                        .unwrap_or_else(|| workspace_id.clone()),
                    base_branch: workspace
                        .map(|w: &WorkspaceOptionDto| w.base_branch.clone())
                        .unwrap_or_default(),
                    workspace_id,
                    tasks: vec![task],
                    total: 0,
                    attention_count: 0,
                });
            }
        }
    }

    for lane in &mut lanes {
        lane.tasks.sort_by(|a: &TaskDto, b: &TaskDto| {
            stage_rank(a.stage)
                .cmp(&stage_rank(b.stage))
                .then_with(|| parse_updated_at(&b.updated_at).cmp(&parse_updated_at(&a.updated_at)))
        });
        lane.total = lane.tasks.len();
        lane.attention_count = lane
            .tasks
            .iter()
            .filter(|t: &&TaskDto| t.needs_attention)
            .count();
    }

    lanes.sort_by(|a: &LaneView, b: &LaneView| {
        let a_has_attention: bool = a.attention_count > 0;
        let b_has_attention: bool = b.attention_count > 0;
        b_has_attention
            .cmp(&a_has_attention)
            .then_with(|| a.workspace_name.cmp(&b.workspace_name))
    });

    lanes
}

/// Etapas disponibles como chips de filtro (excluye `Done`, controlado por su propio toggle).
pub const FILTER_STAGES: [Option<TaskStageDto>; 5] = [
    None,
    Some(TaskStageDto::Attention),
    Some(TaskStageDto::Working),
    Some(TaskStageDto::Backlog),
    Some(TaskStageDto::Review),
];

/// Etiqueta de un chip de filtro.
pub fn filter_label(stage: Option<TaskStageDto>) -> &'static str {
    match stage {
        None => "Todas",
        Some(s) => s.label(),
    }
}

/// Estado de la pantalla del tablero: tareas agrupadas por carril, filtros y modal de tarea.
#[derive(Clone, Copy)]
pub struct BoardState {
    pub tasks: Resource<Result<Vec<TaskDto>, ServerFnError>>,
    pub workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>>,
    pub stage_filter: RwSignal<Option<TaskStageDto>>,
    pub show_done: RwSignal<bool>,
    pub create: ServerAction<CreateTask>,
    pub update: ServerAction<UpdateTask>,
    pub delete: ServerAction<DeleteTask>,
    pub start: ServerAction<StartTask>,
    pub finish: ServerAction<FinishTask>,
    pub merge: ServerAction<MergeTask>,
    pub modal_open: RwSignal<bool>,
    /// `None` = crear; `Some(id)` = editar la tarea con ese id.
    pub edit_task_id: RwSignal<Option<String>>,
    pub form_workspace_id: RwSignal<String>,
    pub form_title: RwSignal<String>,
    pub form_prompt: RwSignal<String>,
    pub form_start_now: RwSignal<bool>,
    pub toasts: RwSignal<Vec<ToastMessage>>,
    toast_seq: RwSignal<u32>,
    pub start_target: RwSignal<Option<String>>,
    pub delete_target: RwSignal<Option<String>>,
    pub finish_target: RwSignal<Option<String>>,
    pub merge_target: RwSignal<Option<String>>,
    #[cfg_attr(not(feature = "hydrate"), allow(dead_code))]
    tick: RwSignal<u32>,
}

/// Texto del aviso cuando la tarea se crea pero no puede iniciarse.
pub fn start_failed_after_create_text(error: &str) -> String {
    format!("Tarea creada en Por hacer, pero no se pudo iniciar: {error}")
}

impl BoardState {
    pub fn new() -> Self {
        let create: ServerAction<CreateTask> = ServerAction::new();
        let update: ServerAction<UpdateTask> = ServerAction::new();
        let delete: ServerAction<DeleteTask> = ServerAction::new();
        let start: ServerAction<StartTask> = ServerAction::new();
        let finish: ServerAction<FinishTask> = ServerAction::new();
        let merge: ServerAction<MergeTask> = ServerAction::new();
        let tick: RwSignal<u32> = RwSignal::new(0);
        let tasks: Resource<Result<Vec<TaskDto>, ServerFnError>> = Resource::new(
            move || {
                (
                    tick.get(),
                    create.version().get(),
                    update.version().get(),
                    delete.version().get(),
                    start.version().get(),
                    finish.version().get(),
                    merge.version().get(),
                )
            },
            |_| list_tasks(None),
        );
        let workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>> =
            Resource::new(|| (), |_| list_workspace_options());

        let state: Self = Self {
            tasks,
            workspaces,
            stage_filter: RwSignal::new(None),
            show_done: RwSignal::new(false),
            create,
            update,
            delete,
            start,
            finish,
            merge,
            modal_open: RwSignal::new(false),
            edit_task_id: RwSignal::new(None),
            form_workspace_id: RwSignal::new(String::new()),
            form_title: RwSignal::new(String::new()),
            form_prompt: RwSignal::new(String::new()),
            form_start_now: RwSignal::new(false),
            toasts: RwSignal::new(Vec::new()),
            toast_seq: RwSignal::new(0),
            start_target: RwSignal::new(None),
            delete_target: RwSignal::new(None),
            finish_target: RwSignal::new(None),
            merge_target: RwSignal::new(None),
            tick,
        };
        state.connect_events();
        state
    }

    /// Abre el modal en modo creación con el formulario vacío, descartando errores previos.
    pub fn open_create(&self) {
        self.create.clear();
        self.update.clear();
        self.edit_task_id.set(None);
        self.form_workspace_id.set(String::new());
        self.form_title.set(String::new());
        self.form_prompt.set(String::new());
        self.form_start_now.set(false);
        self.modal_open.set(true);
    }

    /// Abre el modal en modo edición precargado con los datos de `task`, descartando errores previos.
    pub fn open_edit(&self, task: &TaskDto) {
        self.create.clear();
        self.update.clear();
        self.edit_task_id.set(Some(task.id.clone()));
        self.form_workspace_id.set(task.workspace_id.clone());
        self.form_title.set(task.title.clone());
        self.form_prompt.set(task.prompt.clone());
        self.form_start_now.set(false);
        self.modal_open.set(true);
    }

    pub fn close_modal(&self) {
        self.modal_open.set(false);
    }

    /// Envía el formulario: crea o actualiza según `edit_task_id`.
    pub fn submit(&self) {
        match self.edit_task_id.get_untracked() {
            None => {
                self.create.dispatch(CreateTask {
                    input: CreateTaskDto {
                        workspace_id: self.form_workspace_id.get_untracked(),
                        title: self.form_title.get_untracked(),
                        prompt: self.form_prompt.get_untracked(),
                        start_now: self.form_start_now.get_untracked(),
                    },
                });
            }
            Some(id) => {
                self.update.dispatch(UpdateTask {
                    id,
                    input: UpdateTaskDto {
                        title: self.form_title.get_untracked(),
                        prompt: self.form_prompt.get_untracked(),
                    },
                });
            }
        }
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

    /// Muestra un error de una acción del tablero como notificación, incluyendo el título
    /// de la tarea afectada cuando se conoce su id.
    pub fn notify_error(&self, task_id: Option<String>, message: String) {
        let title: Option<String> = task_id.as_deref().and_then(|id: &str| {
            self.tasks
                .get_untracked()
                .and_then(|r: Result<Vec<TaskDto>, ServerFnError>| r.ok())
                .and_then(|tasks: Vec<TaskDto>| {
                    tasks
                        .into_iter()
                        .find(|t: &TaskDto| t.id == id)
                        .map(|t: TaskDto| t.title)
                })
        });
        let text: String = match title.or(task_id) {
            Some(label) => format!("{label}: {message}"),
            None => message,
        };
        push_toast(self.toasts, self.toast_seq, text);
    }

    /// Avisa de que la tarea se creó en "Por hacer" pero su arranque falló.
    pub fn notify_start_failed_after_create(&self, error: &str) {
        push_toast(
            self.toasts,
            self.toast_seq,
            start_failed_after_create_text(error),
        );
    }

    pub fn dismiss_toast(&self, id: u32) {
        self.toasts
            .update(|list: &mut Vec<ToastMessage>| list.retain(|t: &ToastMessage| t.id != id));
    }

    #[cfg(not(feature = "hydrate"))]
    fn connect_events(&self) {}

    /// Abre el stream SSE del tablero: cualquier `reset`/`changed`/`lagged` refresca el
    /// listado. Si el navegador cierra la conexión por completo (`readyState == CLOSED`),
    /// la reabre tras `SSE_RECONNECT_DELAY` (los cortes recuperables ya los reintenta el
    /// propio `EventSource`). Además mantiene un sondeo lento de respaldo por si el stream
    /// se corta sin disparar `onerror` (p.ej. proxies que cierran la conexión en silencio).
    #[cfg(feature = "hydrate")]
    fn connect_events(&self) {
        use wasm_bindgen::JsCast;
        use wasm_bindgen::closure::Closure;
        use web_sys::{Event, EventSource};

        type Handler = Closure<dyn FnMut(Event)>;

        struct Connection {
            source: EventSource,
            _handlers: Vec<Handler>,
        }

        #[derive(Default)]
        struct SseState {
            connection: Option<Connection>,
            reconnect: Option<TimeoutHandle>,
        }

        fn open(tick: RwSignal<u32>, slot: StoredValue<SseState, LocalStorage>) {
            let Ok(source) = EventSource::new("/api/tasks/events") else {
                return;
            };
            let on_bump: Handler =
                Closure::new(move |_: Event| tick.update(|n: &mut u32| *n = n.wrapping_add(1)));
            for name in ["reset", "changed", "lagged"] {
                if source
                    .add_event_listener_with_callback(name, on_bump.as_ref().unchecked_ref())
                    .is_err()
                {
                    leptos::logging::error!(
                        "no se pudo suscribir al evento SSE '{name}' del tablero"
                    );
                }
            }
            let watched_source: EventSource = source.clone();
            let on_error: Handler = Closure::new(move |_: Event| {
                if watched_source.ready_state() != EventSource::CLOSED {
                    return;
                }
                let handle: Option<TimeoutHandle> =
                    set_timeout_with_handle(move || open(tick, slot), SSE_RECONNECT_DELAY).ok();
                slot.update_value(|state: &mut SseState| state.reconnect = handle);
            });
            if source
                .add_event_listener_with_callback("error", on_error.as_ref().unchecked_ref())
                .is_err()
            {
                leptos::logging::error!("no se pudo suscribir al evento de error SSE del tablero");
            }
            slot.update_value(|state: &mut SseState| {
                state.connection = Some(Connection {
                    source,
                    _handlers: vec![on_bump, on_error],
                });
            });
        }

        let tick: RwSignal<u32> = self.tick;
        let slot: StoredValue<SseState, LocalStorage> = StoredValue::new_local(SseState::default());
        open(tick, slot);

        on_cleanup(move || {
            slot.update_value(|state: &mut SseState| {
                if let Some(connection) = state.connection.take() {
                    connection.source.close();
                }
                if let Some(handle) = state.reconnect.take() {
                    handle.clear();
                }
            });
        });

        if let Ok(handle) = set_interval_with_handle(
            move || tick.update(|n: &mut u32| *n = n.wrapping_add(1)),
            FALLBACK_POLL_INTERVAL,
        ) {
            on_cleanup(move || handle.clear());
        }
    }
}

impl Default for BoardState {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn start_failure_after_create_text_mentions_backlog_and_error() {
        assert_eq!(
            start_failed_after_create_text("sin rama"),
            "Tarea creada en Por hacer, pero no se pudo iniciar: sin rama"
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
            error: None,
            actions: Default::default(),
            stage,
            needs_attention: matches!(stage, TaskStageDto::Attention | TaskStageDto::Review),
            pending_dialogs: 0,
            created_at: updated_at.to_string(),
            updated_at: updated_at.to_string(),
        }
    }

    fn workspace(id: &str, name: &str) -> WorkspaceOptionDto {
        WorkspaceOptionDto {
            id: id.to_string(),
            name: name.to_string(),
            base_branch: "main".to_string(),
        }
    }

    #[test]
    fn workspaces_without_visible_tasks_have_no_lane() {
        let tasks: Vec<TaskDto> = vec![task(
            "1",
            "ws-a",
            TaskStageDto::Done,
            "2026-01-01T00:00:00Z",
        )];
        let workspaces: Vec<WorkspaceOptionDto> =
            vec![workspace("ws-a", "A"), workspace("ws-b", "B")];

        let lanes: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, false);

        assert!(lanes.is_empty());
    }

    #[test]
    fn tasks_are_ordered_by_stage_priority_then_recency() {
        let tasks: Vec<TaskDto> = vec![
            task(
                "backlog",
                "ws-a",
                TaskStageDto::Backlog,
                "2026-01-01T00:00:00Z",
            ),
            task(
                "old-review",
                "ws-a",
                TaskStageDto::Review,
                "2026-01-01T00:00:00Z",
            ),
            task(
                "new-review",
                "ws-a",
                TaskStageDto::Review,
                "2026-01-02T00:00:00Z",
            ),
            task(
                "attention",
                "ws-a",
                TaskStageDto::Attention,
                "2026-01-01T00:00:00Z",
            ),
            task(
                "working",
                "ws-a",
                TaskStageDto::Working,
                "2026-01-01T00:00:00Z",
            ),
        ];
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("ws-a", "A")];

        let lanes: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, false);

        assert_eq!(lanes.len(), 1);
        let ids: Vec<&str> = lanes[0]
            .tasks
            .iter()
            .map(|t: &TaskDto| t.id.as_str())
            .collect();
        assert_eq!(
            ids,
            vec![
                "attention",
                "new-review",
                "old-review",
                "working",
                "backlog"
            ]
        );
    }

    #[test]
    fn lanes_with_attention_come_first_then_alphabetical() {
        let tasks: Vec<TaskDto> = vec![
            task("1", "ws-z", TaskStageDto::Working, "2026-01-01T00:00:00Z"),
            task("2", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z"),
            task("3", "ws-m", TaskStageDto::Attention, "2026-01-01T00:00:00Z"),
        ];
        let workspaces: Vec<WorkspaceOptionDto> = vec![
            workspace("ws-z", "Zeta"),
            workspace("ws-a", "Alpha"),
            workspace("ws-m", "Medio"),
        ];

        let lanes: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, false);

        let names: Vec<&str> = lanes
            .iter()
            .map(|l: &LaneView| l.workspace_name.as_str())
            .collect();
        assert_eq!(names, vec!["Medio", "Alpha", "Zeta"]);
    }

    #[test]
    fn done_tasks_are_hidden_unless_show_done_is_set() {
        let tasks: Vec<TaskDto> = vec![
            task("1", "ws-a", TaskStageDto::Done, "2026-01-01T00:00:00Z"),
            task("2", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z"),
        ];
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("ws-a", "A")];

        let hidden: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, false);
        assert_eq!(hidden[0].tasks.len(), 1);

        let shown: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, true);
        assert_eq!(shown[0].tasks.len(), 2);
    }

    #[test]
    fn stage_filter_narrows_visible_tasks() {
        let tasks: Vec<TaskDto> = vec![
            task("1", "ws-a", TaskStageDto::Backlog, "2026-01-01T00:00:00Z"),
            task("2", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z"),
        ];
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("ws-a", "A")];

        let lanes: Vec<LaneView> =
            build_lanes(&tasks, &workspaces, Some(TaskStageDto::Working), false);

        assert_eq!(lanes.len(), 1);
        assert_eq!(lanes[0].tasks.len(), 1);
        assert_eq!(lanes[0].tasks[0].id, "2");
    }

    #[test]
    fn lane_counts_reflect_visible_tasks() {
        let tasks: Vec<TaskDto> = vec![
            task("1", "ws-a", TaskStageDto::Attention, "2026-01-01T00:00:00Z"),
            task("2", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z"),
        ];
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("ws-a", "A")];

        let lanes: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, false);

        assert_eq!(lanes[0].total, 2);
        assert_eq!(lanes[0].attention_count, 1);
    }

    #[test]
    fn unknown_workspace_falls_back_to_its_id() {
        let tasks: Vec<TaskDto> = vec![task(
            "1",
            "missing-ws",
            TaskStageDto::Working,
            "2026-01-01T00:00:00Z",
        )];

        let lanes: Vec<LaneView> = build_lanes(&tasks, &[], None, false);

        assert_eq!(lanes[0].workspace_name, "missing-ws");
    }
}
