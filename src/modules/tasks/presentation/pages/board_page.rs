use leptos::prelude::*;
use leptos_meta::Title;
use leptos_router::NavigateOptions;
use leptos_router::hooks::use_navigate;

use crate::core::components::layout::ErrorText;
use crate::core::components::toast::ToastStack;
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
    BoardState, LaneView, build_lanes, combine_tasks, done_has_more, lane_ids,
};

#[component]
pub fn BoardPage() -> impl IntoView {
    let state: BoardState = BoardState::new();

    let navigate = use_navigate();

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
            navigate(
                &format!("/tasks/{}/edit", task.id),
                NavigateOptions::default(),
            );
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

    // Se actualiza dentro del `Suspend` de abajo (leer el resource fuera de
    // Suspense/Transition provoca desajustes de hidratación).
    let attention_count: RwSignal<usize> = RwSignal::new(0);
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
        let tasks: Option<Result<Vec<TaskDto>, ServerFnError>> = state.tasks.get();
        let done: Option<Result<Vec<TaskDto>, ServerFnError>> = state.done_tasks.get();
        let pages: usize = state.done_pages.get();
        // Un fallo al cargar las finalizadas no debe ocultar las tareas activas: se pinta el
        // tablero sin ellas.
        let done: Option<Vec<TaskDto>> =
            done.map(|loaded: Result<Vec<TaskDto>, ServerFnError>| loaded.unwrap_or_default());
        match (workspaces, tasks, done) {
            (Some(Ok(workspaces)), Some(Ok(active)), Some(done)) => {
                has_more_done.set(show_done && done_has_more(done.len(), pages));
                let tasks: Vec<TaskDto> = combine_tasks(&active, &done);
                let next: Vec<LaneView> = build_lanes(&tasks, &workspaces, stage_filter, show_done);
                if lanes.with_untracked(|current: &Vec<LaneView>| *current != next) {
                    lanes.set(next);
                }
                loaded.set(true);
            }
            (Some(Err(_)), _, _) | (_, Some(Err(_)), _) => loaded.set(false),
            // Recargando: se conserva el tablero anterior.
            _ => {}
        }
    });
    let filters_active: Memo<bool> =
        Memo::new(move |_| state.stage_filter.get().is_some() || state.show_done.get());
    let document_title: Memo<String> = Memo::new(move |_| {
        let count: usize = attention_count.get();
        if count > 0 {
            format!("({count}) iworkflow")
        } else {
            "iworkflow".to_string()
        }
    });

    view! {
        <Title text=move || document_title.get() />
        <div class="flex h-full min-h-0 flex-col">
        <BoardToolbar
            stage_filter=state.stage_filter
            show_done=state.show_done
            on_new_task=Callback::new(move |_| state.open_create())
        />
        // Este `Suspend` solo espera los datos y pinta errores: el tablero se pinta fuera
        // (a partir de `lanes`) para que un refresco no lo reconstruya.
        <Transition fallback=|| view! { <p class="muted p-4">"Cargando tablero…"</p> }>
            {move || Suspend::new(async move {
                if let Err(e) = state.workspaces.await {
                    return view! { <ErrorText message=e.to_string() /> }.into_any();
                }
                match state.tasks.await {
                    Ok(tasks) => {
                        attention_count
                            .set(tasks.iter().filter(|t: &&TaskDto| t.needs_attention).count());
                    }
                    Err(e) => return view! { <ErrorText message=e.to_string() /> }.into_any(),
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
            <p class="muted p-8 text-center">"No hay tareas visibles con estos filtros."</p>
        </Show>
        <Show when=move || {
            loaded.get() && lanes.with(|l: &Vec<LaneView>| l.is_empty()) && !filters_active.get()
        }>
            <p class="muted p-8 text-center">"Aún no hay tareas."</p>
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
                    "Cargar más"
                </button>
            </div>
        </Show>
        <Transition fallback=|| ()>
            {move || Suspend::new(async move {
                // El error de carga ya se muestra en el tablero; sin workspaces no hay modal.
                match state.workspaces.await {
                    Ok(workspaces) => view! { <TaskModal state workspaces /> }.into_any(),
                    Err(_) => ().into_any(),
                }
            })}
        </Transition>
        <ToastStack toasts=state.toasts />
        </div>
    }
}
