use leptos::prelude::*;

use crate::modules::tasks::presentation::components::task_card::{
    TaskCard, TaskCardBusy, TaskCardHandlers,
};
use crate::modules::tasks::presentation::state::board_state::LaneView;

/// Carril del tablero: cabecera con contadores del workspace y sus tarjetas.
#[component]
pub fn BoardLane(lane: LaneView, handlers: TaskCardHandlers, busy: TaskCardBusy) -> impl IntoView {
    let anchor: String = format!("lane-{}", lane.workspace_id);

    view! {
        <section
            id=anchor
            class="stack w-[85vw] shrink-0 snap-start scroll-mt-24 gap-3 sm:w-80 sm:scroll-mt-4"
        >
            <header class="row items-start justify-between gap-2">
                <div class="stack gap-0.5">
                    <h2 class="font-semibold text-fg">{lane.workspace_name.clone()}</h2>
                    <code class="text-xs text-muted">{lane.base_branch.clone()}</code>
                </div>
                <div class="row gap-1.5">
                    <span class="badge badge-muted">{lane.total}</span>
                    {(lane.attention_count > 0)
                        .then(|| view! { <span class="badge badge-attention">{lane.attention_count}</span> })}
                </div>
            </header>
            <div class="stack max-h-[70vh] gap-2 overflow-y-auto pr-1">
                {lane
                    .tasks
                    .into_iter()
                    .map(|task| view! { <TaskCard task handlers busy /> })
                    .collect_view()}
            </div>
        </section>
    }
}
