use std::collections::HashSet;
use std::future::Future;

use leptos::prelude::*;

use crate::core::components::form::parse_positive_number;

use crate::core::components::toast::{ToastMessage, push_toast};
use crate::modules::tasks::infrastructure::adapters::inbound::dto::image_attachment_dto::ImageAttachmentDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    CreateTaskDto, TaskDto, TaskPriorityDto, TaskStageDto, WorkspaceOptionDto,
};
use crate::modules::tasks::infrastructure::adapters::inbound::server_fns::{
    CreateTask, DeleteTask, FinishTask, MergeTask, StartTask, list_done_tasks,
};
use crate::modules::tasks::presentation::state::image_attachments::ImageAttachments;
use crate::modules::tasks::presentation::state::task_store::{TaskStore, use_task_store};

/// Compara dos instantes RFC 3339 por su valor temporal; si alguno no se puede interpretar
/// se recurre a la comparación textual, que es cronológica para el formato que emite el servidor.
fn compare_instants(a: &str, b: &str) -> std::cmp::Ordering {
    let parse = |value: &str| -> Option<chrono::DateTime<chrono::FixedOffset>> {
        chrono::DateTime::parse_from_rfc3339(value).ok()
    };
    match (parse(a), parse(b)) {
        (Some(left), Some(right)) => left.cmp(&right),
        _ => a.cmp(b),
    }
}

/// Orden de las tarjetas dentro de una columna: primero las abiertas (atención, prioridad y
/// antigüedad) y al final las finalizadas, la última cerrada primero.
fn compare_lane_tasks(a: &TaskDto, b: &TaskDto) -> std::cmp::Ordering {
    let a_done: bool = a.stage == TaskStageDto::Done;
    let b_done: bool = b.stage == TaskStageDto::Done;
    a_done.cmp(&b_done).then_with(|| {
        if a_done {
            return compare_instants(&b.updated_at, &a.updated_at);
        }
        b.needs_attention.cmp(&a.needs_attention).then_with(|| {
            b.priority
                .rank()
                .cmp(&a.priority.rank())
                .then_with(|| compare_instants(&a.created_at, &b.created_at))
        })
    })
}

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

/// El chip "Necesita atención" usa `needs_attention` (Attention + Review) para coincidir con
/// los contadores del carril y del título; el resto de chips filtran por etapa exacta.
fn matches_stage_filter(task: &TaskDto, stage_filter: Option<TaskStageDto>) -> bool {
    match stage_filter {
        None => true,
        Some(TaskStageDto::Attention) => task.needs_attention,
        Some(stage) => task.stage == stage,
    }
}

/// Agrupa, filtra y ordena las tareas en carriles por workspace (función pura, sin I/O).
///
/// Reglas: un workspace solo aparece si tiene tareas visibles tras aplicar `stage_filter`
/// y `show_done`; dentro de un carril las tareas se ordenan por prioridad descendente y, a igual
/// prioridad, por `created_at` ascendente (la más antigua primero); los carriles con alguna tarea en atención van primero,
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
            let matches_filter: bool = matches_stage_filter(t, stage_filter);
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
        // `sort_by` is stable: attention-first keeps the priority/age order within each group.
        lane.tasks.sort_by(compare_lane_tasks);
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

/// Etiqueta del contador de tareas de un carril ("1 tarea" / "3 tareas").
pub fn lane_total_label(total: usize) -> String {
    if total == 1 {
        "1 tarea".to_string()
    } else {
        format!("{total} tareas")
    }
}

/// Etiqueta del contador de atención de un carril; `None` cuando no hay ninguna.
pub fn lane_attention_label(attention: usize) -> Option<String> {
    (attention > 0).then(|| format!("{attention} atención"))
}

/// Clave de una tarjeta para el `<For>` del tablero: identifica la tarea y cambia cuando
/// cambia cualquier dato visible, de modo que solo se reconstruyen las tarjetas modificadas.
pub fn card_key(task: &TaskDto) -> String {
    format!(
        "{}|{}|{}|{:?}|{:?}|{:?}|{}|{}|{:?}|{}",
        task.id,
        task.updated_at,
        task.title,
        task.stage,
        task.status,
        task.priority,
        task.needs_attention,
        task.pending_dialogs,
        task.error,
        task.prompt,
    )
}

/// Ids de los carriles en orden de pintado (claves del `<For>` de carriles).
pub fn lane_ids(lanes: &[LaneView]) -> Vec<String> {
    lanes
        .iter()
        .map(|l: &LaneView| l.workspace_id.clone())
        .collect()
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

/// Estado de la pantalla del tablero: tareas agrupadas por carril, filtros y modal de alta de tarea.
#[derive(Clone, Copy)]
pub struct BoardState {
    /// Almacén compartido (shell): tareas activas, finalizadas visibles y workspaces.
    pub store: TaskStore,
    /// Carga de las finalizadas (vacío mientras `show_done` esté desactivado); su resultado
    /// se vuelca en `store.done`, que los eventos de SSE parchean sin recargar páginas.
    pub done_tasks: Resource<Result<DonePages, ServerFnError>>,
    /// Si la última página de finalizadas pedida al servidor vino completa; solo cambia al
    /// cargar, no con los parches de SSE sobre `store.done`.
    pub done_has_more: RwSignal<bool>,
    /// Páginas de finalizadas que se piden al servidor ("Cargar más" la incrementa).
    pub done_pages: RwSignal<usize>,
    pub workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>>,
    pub stage_filter: RwSignal<Option<TaskStageDto>>,
    pub show_done: RwSignal<bool>,
    pub create: ServerAction<CreateTask>,
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
    pub form_start_now: RwSignal<bool>,
    /// Imágenes adjuntas al prompt inicial.
    pub images: ImageAttachments,
    pub toasts: RwSignal<Vec<ToastMessage>>,
    toast_seq: RwSignal<u32>,
    pub start_target: RwSignal<Option<String>>,
    pub delete_target: RwSignal<Option<String>>,
    pub finish_target: RwSignal<Option<String>>,
    pub merge_target: RwSignal<Option<String>>,
}

/// Texto del aviso cuando la tarea se crea pero no puede iniciarse.
pub fn start_failed_after_create_text(error: &str) -> String {
    format!("Tarea creada en Por hacer, pero no se pudo iniciar: {error}")
}

/// Construye el DTO de alta con las imágenes adjuntas (función pura).
pub fn build_create_task_dto(
    workspace_id: String,
    title: String,
    prompt: String,
    priority: &str,
    memory_limit: &str,
    cpu_limit: &str,
    start_now: bool,
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
        images,
    }
}

impl BoardState {
    pub fn new() -> Self {
        let create: ServerAction<CreateTask> = ServerAction::new();
        let delete: ServerAction<DeleteTask> = ServerAction::new();
        let start: ServerAction<StartTask> = ServerAction::new();
        let finish: ServerAction<FinishTask> = ServerAction::new();
        let merge: ServerAction<MergeTask> = ServerAction::new();
        let store: TaskStore = use_task_store();
        let show_done: RwSignal<bool> = store.show_done;
        // El tablero arranca sin finalizadas y, al salir, deja de parchearlas.
        show_done.set(false);
        on_cleanup(move || show_done.set(false));
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
        // Solo se recarga al (des)activar `show_done`, pedir más páginas o en una recarga
        // completa del almacén; los `changed` sueltos parchean `store.done`.
        let done_tasks: Resource<Result<DonePages, ServerFnError>> = Resource::new(
            move || (show_done.get(), done_pages.get(), store.reload_count()),
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
        let workspaces: Resource<Result<Vec<WorkspaceOptionDto>, ServerFnError>> = store.workspaces;

        Self {
            store,
            done_tasks,
            done_has_more: has_more,
            done_pages,
            workspaces,
            stage_filter: RwSignal::new(None),
            show_done,
            create,
            delete,
            start,
            finish,
            merge,
            modal_open: RwSignal::new(false),
            form_workspace_id: RwSignal::new(String::new()),
            form_title: RwSignal::new(String::new()),
            form_prompt: RwSignal::new(String::new()),
            form_priority: RwSignal::new(TaskPriorityDto::default().as_str().to_string()),
            form_memory_limit: RwSignal::new(String::new()),
            form_cpu_limit: RwSignal::new(String::new()),
            form_start_now: RwSignal::new(false),
            images: ImageAttachments::new(),
            toasts: RwSignal::new(Vec::new()),
            toast_seq: RwSignal::new(0),
            start_target: RwSignal::new(None),
            delete_target: RwSignal::new(None),
            finish_target: RwSignal::new(None),
            merge_target: RwSignal::new(None),
        }
    }

    /// Abre el modal de alta con el formulario vacío, descartando errores previos.
    pub fn open_create(&self) {
        self.create.clear();
        self.form_workspace_id.set(String::new());
        self.form_title.set(String::new());
        self.form_prompt.set(String::new());
        self.form_priority
            .set(TaskPriorityDto::default().as_str().to_string());
        self.form_memory_limit.set(String::new());
        self.form_cpu_limit.set(String::new());
        self.form_start_now.set(false);
        self.images.clear();
        self.modal_open.set(true);
    }

    pub fn close_modal(&self) {
        self.modal_open.set(false);
        self.images.clear();
    }

    /// Envía el formulario de alta.
    pub fn submit(&self) {
        self.create.dispatch(CreateTask {
            input: build_create_task_dto(
                self.form_workspace_id.get_untracked(),
                self.form_title.get_untracked(),
                self.form_prompt.get_untracked(),
                &self.form_priority.get_untracked(),
                &self.form_memory_limit.get_untracked(),
                &self.form_cpu_limit.get_untracked(),
                self.form_start_now.get_untracked(),
                self.images.images.get_untracked(),
            ),
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
            "2048",
            "0",
            true,
            vec![image.clone()],
        );
        assert_eq!(dto.memory_limit_mb, Some(2048));
        assert_eq!(dto.cpu_limit_percent, None);
        assert_eq!(dto.images, vec![image]);
        assert_eq!(dto.priority, TaskPriorityDto::High);
        assert!(dto.start_now);
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
    fn combine_tasks_appends_done_after_active_and_feeds_build_lanes() {
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
        let lanes: Vec<LaneView> = build_lanes(&combined, &[workspace("ws-a", "A")], None, true);
        assert_eq!(lanes[0].tasks.len(), 2);
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
            priority: Default::default(),
            memory_limit_mb: None,
            cpu_limit_percent: None,
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
    fn card_key_changes_only_when_visible_data_changes() {
        let base: TaskDto = task("a", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z");
        let same: TaskDto = base.clone();
        let mut moved: TaskDto = base.clone();
        moved.stage = TaskStageDto::Review;
        let mut retitled: TaskDto = base.clone();
        retitled.title = "otro".to_string();

        assert_eq!(card_key(&base), card_key(&same));
        assert_ne!(card_key(&base), card_key(&moved));
        assert_ne!(card_key(&base), card_key(&retitled));
    }

    #[test]
    fn lane_ids_follow_lane_order() {
        let tasks: Vec<TaskDto> = vec![
            task("a", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z"),
            task("b", "ws-b", TaskStageDto::Attention, "2026-01-01T00:00:00Z"),
        ];
        let workspaces: Vec<WorkspaceOptionDto> =
            vec![workspace("ws-a", "A"), workspace("ws-b", "B")];
        let lanes: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, false);

        assert_eq!(
            lane_ids(&lanes),
            vec!["ws-b".to_string(), "ws-a".to_string()]
        );
    }

    fn lane_ids_of(tasks: &[TaskDto]) -> Vec<String> {
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("ws-a", "A")];
        let lanes: Vec<LaneView> = build_lanes(tasks, &workspaces, None, false);
        assert_eq!(lanes.len(), 1);
        lanes[0]
            .tasks
            .iter()
            .map(|t: &TaskDto| t.id.clone())
            .collect()
    }

    fn prioritized(
        id: &str,
        stage: TaskStageDto,
        priority: TaskPriorityDto,
        created_at: &str,
    ) -> TaskDto {
        let mut dto: TaskDto = task(id, "ws-a", stage, "2030-01-01T00:00:00Z");
        dto.priority = priority;
        dto.created_at = created_at.to_string();
        dto
    }

    #[test]
    fn equal_priority_is_listed_oldest_first_regardless_of_non_attention_stage_or_updates() {
        let tasks: Vec<TaskDto> = vec![
            prioritized(
                "newest",
                TaskStageDto::Working,
                TaskPriorityDto::Normal,
                "2026-01-03T00:00:00Z",
            ),
            prioritized(
                "oldest",
                TaskStageDto::Backlog,
                TaskPriorityDto::Normal,
                "2026-01-01T00:00:00Z",
            ),
            prioritized(
                "middle",
                TaskStageDto::Backlog,
                TaskPriorityDto::Normal,
                "2026-01-02T00:00:00Z",
            ),
        ];

        assert_eq!(lane_ids_of(&tasks), vec!["oldest", "middle", "newest"]);
    }

    #[test]
    fn higher_priority_beats_age() {
        let tasks: Vec<TaskDto> = vec![
            prioritized(
                "old-normal",
                TaskStageDto::Working,
                TaskPriorityDto::Normal,
                "2026-01-01T00:00:00Z",
            ),
            prioritized(
                "new-urgent",
                TaskStageDto::Working,
                TaskPriorityDto::Urgent,
                "2026-01-05T00:00:00Z",
            ),
            prioritized(
                "old-low",
                TaskStageDto::Working,
                TaskPriorityDto::Low,
                "2025-12-01T00:00:00Z",
            ),
            prioritized(
                "mid-high",
                TaskStageDto::Working,
                TaskPriorityDto::High,
                "2026-01-03T00:00:00Z",
            ),
            prioritized(
                "old-high",
                TaskStageDto::Working,
                TaskPriorityDto::High,
                "2026-01-02T00:00:00Z",
            ),
        ];

        assert_eq!(
            lane_ids_of(&tasks),
            vec![
                "new-urgent",
                "old-high",
                "mid-high",
                "old-normal",
                "old-low"
            ]
        );
    }

    #[test]
    fn attention_tasks_come_first_keeping_order_within_groups() {
        let tasks: Vec<TaskDto> = vec![
            prioritized(
                "w-old",
                TaskStageDto::Working,
                TaskPriorityDto::Normal,
                "2026-01-01T00:00:00Z",
            ),
            prioritized(
                "a-new",
                TaskStageDto::Attention,
                TaskPriorityDto::Normal,
                "2026-01-03T00:00:00Z",
            ),
            prioritized(
                "w-new",
                TaskStageDto::Working,
                TaskPriorityDto::Normal,
                "2026-01-02T00:00:00Z",
            ),
            prioritized(
                "r-old",
                TaskStageDto::Review,
                TaskPriorityDto::Normal,
                "2026-01-02T12:00:00Z",
            ),
        ];

        assert_eq!(
            lane_ids_of(&tasks),
            vec!["r-old", "a-new", "w-old", "w-new"]
        );
    }

    #[test]
    fn lane_labels_are_singular_and_plural() {
        assert_eq!(lane_total_label(1), "1 tarea");
        assert_eq!(lane_total_label(3), "3 tareas");
        assert_eq!(lane_attention_label(0), None);
        assert_eq!(lane_attention_label(2), Some("2 atención".to_string()));
    }

    #[test]
    fn card_key_changes_when_priority_changes() {
        let base: TaskDto = task("a", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z");
        let mut urgent: TaskDto = base.clone();
        urgent.priority = TaskPriorityDto::Urgent;

        assert_ne!(card_key(&base), card_key(&urgent));
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
    fn attention_filter_matches_attention_counters() {
        let tasks: Vec<TaskDto> = vec![
            task("1", "ws-a", TaskStageDto::Attention, "2026-01-01T00:00:00Z"),
            task("2", "ws-a", TaskStageDto::Review, "2026-01-01T00:00:00Z"),
            task("3", "ws-a", TaskStageDto::Working, "2026-01-01T00:00:00Z"),
        ];
        let workspaces: Vec<WorkspaceOptionDto> = vec![workspace("ws-a", "A")];

        let unfiltered: Vec<LaneView> = build_lanes(&tasks, &workspaces, None, false);
        let filtered: Vec<LaneView> =
            build_lanes(&tasks, &workspaces, Some(TaskStageDto::Attention), false);

        assert_eq!(filtered[0].tasks.len(), unfiltered[0].attention_count);
        assert_eq!(filtered[0].tasks.len(), 2);
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

    #[test]
    fn done_tasks_follow_active_ones_ordered_by_latest_completion_first() {
        let mut old_urgent: TaskDto = task(
            "done-old",
            "ws-a",
            TaskStageDto::Done,
            "2026-01-01T00:00:00Z",
        );
        old_urgent.priority = TaskPriorityDto::Urgent;
        old_urgent.created_at = "2025-12-01T00:00:00Z".to_string();
        let mut last: TaskDto = task(
            "done-last",
            "ws-a",
            TaskStageDto::Done,
            "2026-01-05T00:00:00Z",
        );
        last.priority = TaskPriorityDto::Low;
        last.created_at = "2025-12-31T00:00:00Z".to_string();
        let mut middle: TaskDto = task(
            "done-mid",
            "ws-a",
            TaskStageDto::Done,
            "2026-01-03T00:00:00Z",
        );
        middle.created_at = "2025-12-15T00:00:00Z".to_string();
        let working: TaskDto = task(
            "working",
            "ws-a",
            TaskStageDto::Working,
            "2026-01-02T00:00:00Z",
        );

        let lanes: Vec<LaneView> = build_lanes(
            &[old_urgent, working, last, middle],
            &[workspace("ws-a", "A")],
            None,
            true,
        );

        let ids: Vec<&str> = lanes[0]
            .tasks
            .iter()
            .map(|t: &TaskDto| t.id.as_str())
            .collect();
        assert_eq!(ids, vec!["working", "done-last", "done-mid", "done-old"]);
    }
}
