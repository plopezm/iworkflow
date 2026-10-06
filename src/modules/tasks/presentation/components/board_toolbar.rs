use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStageDto;
use crate::modules::tasks::presentation::state::board_state::{FILTER_STAGES, filter_label};

/// Clase Tailwind completa del punto de color de un chip de filtro.
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
        <div class="flex shrink-0 flex-col gap-2 border-b border-border bg-surface px-4 py-2.5">
            <div class="flex items-center justify-between gap-3">
                <h1 class="page-title">"Tablero"</h1>
                <button class="btn-primary" on:click=move |_| on_new_task.run(())>
                    <span class="text-lg leading-none">+</span>
                    <span class="hidden sm:inline">"Nueva tarea"</span>
                </button>
            </div>
            <div class="flex items-center gap-2 overflow-x-auto pb-0.5">
                {FILTER_STAGES
                    .into_iter()
                    .map(|stage: Option<TaskStageDto>| {
                        let class: Memo<&'static str> = Memo::new(move |_| {
                            if stage_filter.get() == stage { "chip chip-selected" } else { "chip" }
                        });
                        view! {
                            <button
                                class=move || class.get()
                                on:click=move |_| stage_filter.set(stage)
                            >
                                {stage.map(|s: TaskStageDto| view! { <span class=filter_dot_class(s)></span> })}
                                {filter_label(stage)}
                            </button>
                        }
                    })
                    .collect_view()}
                <div class="mx-1 h-5 w-px shrink-0 bg-border"></div>
                <label class="flex min-h-9 shrink-0 items-center gap-2 whitespace-nowrap text-xs text-muted">
                    <input
                        type="checkbox"
                        class="h-4 w-4 accent-accent"
                        prop:checked=move || show_done.get()
                        on:change=move |ev| show_done.set(event_target_checked(&ev))
                    />
                    "Mostrar terminadas"
                </label>
            </div>
        </div>
    }
}
