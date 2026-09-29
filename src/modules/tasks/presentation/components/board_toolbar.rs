use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStageDto;
use crate::modules::tasks::presentation::state::board_state::{FILTER_STAGES, filter_label};

/// Clase Tailwind completa del punto de color de un chip de filtro.
///
/// Debe devolver literales completos (no construidos con `format!`) para que el escaneo
/// de contenido de Tailwind detecte y genere las utilidades `bg-stage-*`.
fn filter_dot_class(stage: TaskStageDto) -> &'static str {
    match stage {
        TaskStageDto::Backlog => "h-1.5 w-1.5 rounded-full bg-stage-backlog",
        TaskStageDto::Working => "h-1.5 w-1.5 rounded-full bg-stage-working",
        TaskStageDto::Attention => "h-1.5 w-1.5 rounded-full bg-stage-attention",
        TaskStageDto::Review => "h-1.5 w-1.5 rounded-full bg-stage-review",
        TaskStageDto::Done => "h-1.5 w-1.5 rounded-full bg-stage-done",
    }
}

/// Barra superior del tablero: filtros de etapa, toggle de terminadas y alta de tarea.
#[component]
pub fn BoardToolbar(
    stage_filter: RwSignal<Option<TaskStageDto>>,
    show_done: RwSignal<bool>,
    on_new_task: Callback<()>,
) -> impl IntoView {
    view! {
        <div class="sticky top-14 z-10 stack gap-3 border-b border-border bg-bg/90 py-3 backdrop-blur">
            <div class="row justify-between gap-3">
                <h1 class="text-xl font-semibold text-fg">"Tablero"</h1>
                <button class="btn-primary" on:click=move |_| on_new_task.run(())>
                    "+ Nueva tarea"
                </button>
            </div>
            <div class="row gap-2 overflow-x-auto pb-1">
                {FILTER_STAGES
                    .into_iter()
                    .map(|stage: Option<TaskStageDto>| {
                        let class: Memo<&'static str> = Memo::new(move |_| {
                            if stage_filter.get() == stage { "chip chip-selected" } else { "chip" }
                        });
                        view! {
                            <button class=move || class.get() on:click=move |_| stage_filter.set(stage)>
                                {stage.map(|s: TaskStageDto| view! { <span class=filter_dot_class(s)></span> })}
                                {filter_label(stage)}
                            </button>
                        }
                    })
                    .collect_view()}
                <label class="row min-h-10 items-center gap-2 text-sm text-muted">
                    <input
                        type="checkbox"
                        class="h-5 w-5"
                        prop:checked=move || show_done.get()
                        on:change=move |ev| show_done.set(event_target_checked(&ev))
                    />
                    "Mostrar terminadas"
                </label>
            </div>
        </div>
    }
}
