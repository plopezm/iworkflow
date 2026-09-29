use leptos::prelude::*;
use leptos_meta::Title;

use crate::core::components::layout::ErrorText;
use crate::core::components::toast::ToastStack;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::{
    TaskDto, TaskStageDto, WorkspaceOptionDto,
};
use crate::modules::tasks::presentation::components::board_lane::BoardLane;
use crate::modules::tasks::presentation::components::board_toolbar::BoardToolbar;
use crate::modules::tasks::presentation::components::task_card::{
    TaskActionBusy, TaskCardBusy, TaskCardHandlers,
};
use crate::modules::tasks::presentation::components::task_modal::TaskModal;
use crate::modules::tasks::presentation::state::board_state::{BoardState, LaneView, build_lanes};

#[component]
pub fn BoardPage() -> impl IntoView {
    let state: BoardState = BoardState::new();

    // Cierra el modal tras crear o editar con éxito (estado de UI, sin lógica de negocio).
    Effect::new(move |_| {
        let created: bool = state
            .create
            .value()
            .with(|r: &Option<Result<TaskDto, ServerFnError>>| matches!(r, Some(Ok(_))));
        let updated: bool = state
            .update
            .value()
            .with(|r: &Option<Result<TaskDto, ServerFnError>>| matches!(r, Some(Ok(_))));
        if created || updated {
            state.close_modal();
        }
    });

    let handlers: TaskCardHandlers = TaskCardHandlers {
        start: Callback::new(move |id: String| state.start_task(id)),
        edit: Callback::new(move |task: TaskDto| state.open_edit(&task)),
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
    let document_title: Memo<String> = Memo::new(move |_| {
        let count: usize = attention_count.get();
        if count > 0 {
            format!("({count}) pi-workflow")
        } else {
            "pi-workflow".to_string()
        }
    });

    view! {
        <Title text=move || document_title.get() />
        <BoardToolbar
            stage_filter=state.stage_filter
            show_done=state.show_done
            on_new_task=Callback::new(move |_| state.open_create())
        />
        <Transition fallback=|| view! { <p class="muted py-4">"Cargando tablero…"</p> }>
            {move || {
                let stage_filter: Option<TaskStageDto> = state.stage_filter.get();
                let show_done: bool = state.show_done.get();
                Suspend::new(async move {
                    let workspaces: Vec<WorkspaceOptionDto> =
                        state.workspaces.await.unwrap_or_default();
                    match state.tasks.await {
                        Ok(tasks) => {
                            attention_count
                                .set(tasks.iter().filter(|t: &&TaskDto| t.needs_attention).count());
                            let lanes: Vec<LaneView> =
                                build_lanes(&tasks, &workspaces, stage_filter, show_done);
                            if lanes.is_empty() {
                                view! {
                                    <p class="muted py-8 text-center">
                                        "No hay tareas visibles con estos filtros."
                                    </p>
                                }
                                    .into_any()
                            } else {
                                view! {
                                    <div class="flex snap-x snap-mandatory gap-4 overflow-x-auto py-4">
                                        {lanes
                                            .into_iter()
                                            .map(|lane: LaneView| view! { <BoardLane lane handlers busy /> })
                                            .collect_view()}
                                    </div>
                                }
                                    .into_any()
                            }
                        }
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    }
                })
            }}
        </Transition>
        <Transition fallback=|| ()>
            {move || Suspend::new(async move {
                let workspaces: Vec<WorkspaceOptionDto> = state.workspaces.await.unwrap_or_default();
                view! { <TaskModal state workspaces /> }
            })}
        </Transition>
        <ToastStack toasts=state.toasts />
    }
}
