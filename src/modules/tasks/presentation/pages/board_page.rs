use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::core::components::toast::ToastStack;
use crate::core::dock::context::use_tab_nav;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    CreatedTaskDto, TaskDto, TaskStageDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::components::board_lane::BoardLane;
use crate::modules::tasks::presentation::components::board_toolbar::BoardToolbar;
use crate::modules::tasks::presentation::components::task_card::{
    TaskActionBusy, TaskCardBusy, TaskCardHandlers,
};
use crate::modules::tasks::presentation::components::task_modal::TaskModal;
use crate::modules::tasks::presentation::state::board_state::{
    BoardState, LaneView, build_lanes, combine_tasks, lane_ids,
};

use crate::core::i18n::Msg;

const LOADING_BOARD: Msg = Msg {
    en: "Loading board…",
    es: "Cargando tablero…",
    zh: "正在加载看板…",
    de: "Board wird geladen…",
    fr: "Chargement du tableau…",
};

const NO_TASKS_WITH_FILTERS: Msg = Msg {
    en: "No tasks match these filters.",
    es: "No hay tareas visibles con estos filtros.",
    zh: "没有符合这些筛选条件的任务。",
    de: "Keine Aufgaben für diese Filter sichtbar.",
    fr: "Aucune tâche visible avec ces filtres.",
};

const NO_TASKS_YET: Msg = Msg {
    en: "No tasks yet.",
    es: "Aún no hay tareas.",
    zh: "还没有任务。",
    de: "Noch keine Aufgaben.",
    fr: "Aucune tâche pour le moment.",
};

const LOAD_MORE: Msg = Msg {
    en: "Load more",
    es: "Cargar más",
    zh: "加载更多",
    de: "Mehr laden",
    fr: "Charger plus",
};

#[component]
pub fn BoardPage() -> impl IntoView {
    let state: BoardState = BoardState::new();

    let nav = use_tab_nav();

    // Cierra el modal tras crear con éxito (estado de UI, sin lógica de negocio).
    Effect::new(move |_| {
        let created: bool = state
            .create
            .value()
            .with(|r: &Option<Result<CreatedTaskDto, ServerFnError>>| matches!(r, Some(Ok(_))));
        if created {
            state.close_modal();
        }
    });

    let handlers: TaskCardHandlers = TaskCardHandlers {
        start: Callback::new(move |id: String| state.start_task(id)),
        edit: Callback::new(move |task: TaskDto| {
            nav.open(&format!("/tasks/{}/edit", task.id));
        }),
        delete: Callback::new(move |id: String| state.delete_task(id)),
        finish: Callback::new(move |id: String| state.finish_task(id)),
        merge: Callback::new(move |id: String| state.merge_task(id)),
    };
    // Cada acción bloquea solo la tarjeta a la que se aplicó, no el tablero entero.
    let busy: TaskCardBusy = TaskCardBusy {
        start: TaskActionBusy {
            target: state.start_target,
            pending: state.start.pending().into(),
        },
        delete: TaskActionBusy {
            target: state.delete_target,
            pending: state.delete.pending().into(),
        },
        finish: TaskActionBusy {
            target: state.finish_target,
            pending: state.finish.pending().into(),
        },
        merge: TaskActionBusy {
            target: state.merge_target,
            pending: state.merge.pending().into(),
        },
    };

    // Si la tarea se creó pero no pudo arrancar, avisa (la tarea queda en "Por hacer").
    Effect::new(move |_| {
        if let Some(Ok(created)) = state.create.value().get()
            && let Some(message) = created.start_error
        {
            state.notify_start_failed_after_create(&message);
        }
    });
    // Convierte cada error de acción en una notificación descartable (estado de UI).
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

    // Carriles ya calculados; viven fuera del `Suspend` y conservan el valor anterior
    // mientras se recarga, así el tablero no se reconstruye en cada refresco.
    let lanes: RwSignal<Vec<LaneView>> = RwSignal::new(Vec::new());
    let loaded: RwSignal<bool> = RwSignal::new(false);
    let has_more_done: RwSignal<bool> = RwSignal::new(false);
    // Publica los carriles desde un `Effect`: solo corre en el cliente y tras la hidratación.
    // Escribirlos durante el render del `Suspend` hacía que el cliente hidratara el tablero
    // que el HTML del servidor (emitido antes de resolver el `Suspend`) no contenía.
    Effect::new(move |_| {
        let stage_filter: Option<TaskStageDto> = state.stage_filter.get();
        let show_done: bool = state.show_done.get();
        let workspaces: Option<Result<Vec<WorkspaceOptionDto>, ServerFnError>> =
            state.workspaces.get();
        let tasks_failed: bool = state
            .store
            .tasks
            .with(|r: &Option<Result<Vec<TaskDto>, ServerFnError>>| matches!(r, Some(Err(_))));
        let active: Vec<TaskDto> = state.store.active.get();
        let done: Vec<TaskDto> = state.store.done.get();
        let more_done: bool = state.done_has_more.get();
        let tasks_loaded: bool = state.store.loaded.get();
        // Un fallo al cargar las finalizadas no debe ocultar las tareas activas: se pinta el
        // tablero sin ellas.
        match (workspaces, tasks_loaded && !tasks_failed) {
            (Some(Ok(workspaces)), true) => {
                has_more_done.set(show_done && more_done);
                let tasks: Vec<TaskDto> = combine_tasks(&active, &done);
                let next: Vec<LaneView> = build_lanes(&tasks, &workspaces, stage_filter, show_done);
                if lanes.with_untracked(|current: &Vec<LaneView>| *current != next) {
                    lanes.set(next);
                }
                loaded.set(true);
            }
            (Some(Err(_)), _) => loaded.set(false),
            _ if tasks_failed => loaded.set(false),
            // Recargando: se conserva el tablero anterior.
            _ => {}
        }
    });
    // Última lista de workspaces cargada con éxito; solo se escribe si cambia de verdad.
    let modal_workspaces: RwSignal<Vec<WorkspaceOptionDto>> = RwSignal::new(Vec::new());
    Effect::new(move |_| {
        if let Some(Ok(loaded)) = state.workspaces.get()
            && modal_workspaces
                .with_untracked(|current: &Vec<WorkspaceOptionDto>| *current != loaded)
        {
            modal_workspaces.set(loaded);
        }
    });
    let filters_active: Memo<bool> =
        Memo::new(move |_| state.stage_filter.get().is_some() || state.show_done.get());
    view! {
        <div class="flex h-full min-h-0 flex-col">
        <BoardToolbar
            stage_filter=state.stage_filter
            show_done=state.show_done
            on_new_task=Callback::new(move |_| state.open_create())
        />
        // Este `Suspend` solo espera los datos y pinta errores: el tablero se pinta fuera
        // (a partir de `lanes`) para que un refresco no lo reconstruya.
        <Transition fallback=|| view! { <p class="muted p-4">{LOADING_BOARD.get()}</p> }>
            {move || Suspend::new(async move {
                if let Err(e) = state.workspaces.await {
                    return view! { <ErrorText message=e.to_string() /> }.into_any();
                }
                if let Err(e) = state.store.tasks.await {
                    return view! { <ErrorText message=e.to_string() /> }.into_any();
                }
                match state.done_tasks.await {
                    Ok(_) => ().into_any(),
                    Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                }
            })}
        </Transition>
        <Show when=move || {
            loaded.get() && lanes.with(|l: &Vec<LaneView>| l.is_empty()) && filters_active.get()
        }>
            <p class="muted p-8 text-center">{NO_TASKS_WITH_FILTERS.get()}</p>
        </Show>
        <Show when=move || {
            loaded.get() && lanes.with(|l: &Vec<LaneView>| l.is_empty()) && !filters_active.get()
        }>
            <p class="muted p-8 text-center">{NO_TASKS_YET.get()}</p>
        </Show>
        <Show when=move || loaded.get() && lanes.with(|l: &Vec<LaneView>| !l.is_empty())>
            <div class="reveal flex min-h-0 flex-1 snap-x snap-mandatory gap-5 overflow-x-auto p-5">
                <For
                    each=move || lanes.with(|l: &Vec<LaneView>| lane_ids(l))
                    key=|id: &String| id.clone()
                    children=move |workspace_id: String| {
                        view! { <BoardLane workspace_id lanes handlers busy /> }
                    }
                />
            </div>
        </Show>
        <Show when=move || has_more_done.get()>
            <div class="flex justify-center p-4">
                <button
                    type="button"
                    class="btn-ghost"
                    on:click=move |_| state.load_more_done()
                >
                    {LOAD_MORE.get()}
                </button>
            </div>
        </Show>
        // Fuera de cualquier `Suspend`: una recarga de workspaces no debe reconstruir el modal
        // (perdería el foco y lo escrito en el formulario).
        <TaskModal state workspaces=modal_workspaces.into() />
        <ToastStack toasts=state.toasts />
        </div>
    }
}
