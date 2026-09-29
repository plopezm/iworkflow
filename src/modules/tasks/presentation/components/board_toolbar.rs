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
        <div class="sticky top-14 z-10 stack gap-3 border-b border-border/40 bg-bg/80 py-3 backdrop-blur-xl">
            <div class="mx-auto flex w-full max-w-[1600px] items-center justify-between gap-3 px-4 sm:px-6">
                <h1 class="bg-gradient-to-r from-fg to-muted bg-clip-text text-xl font-bold text-transparent">
                    "Tablero"
                </h1>
                <button
                    class="btn-primary"
                    on:click=move |_| on_new_task.run(())
                >
                    <span class="text-lg leading-none">+</span>
                    <span class=move || format!("hidden sm:{}", "inline")> "Nueva tarea" </span>
                </button>
            </div>
            <div class="mx-auto flex w-full max-w-[1600px] items-center gap-2 overflow-x-auto px-4 pb-1 sm:px-6">
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
                <div class="mx-1 h-5 w-px shrink-0 bg-border/60"></div>
                <label class="row min-h-10 items-center gap-2 text-sm text-muted">
                    <input
                        type="checkbox"
                        class="h-5 w-5 accent-accent"
                        prop:checked=move || show_done.get()
                        on:change=move |ev| show_done.set(event_target_checked(&ev))
                    />
                    "Mostrar terminadas"
                </label>
            </div>
        </div>
    }
}
