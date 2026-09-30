use leptos::prelude::*;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskDto;
use crate::modules::tasks::presentation::components::task_card::{
    TaskCard, TaskCardBusy, TaskCardHandlers,
};
use crate::modules::tasks::presentation::state::board_state::{LaneView, card_key};

/// Carril del tablero: cabecera con contadores del workspace y sus tarjetas.
#[component]
pub fn BoardLane(
    workspace_id: String,
    lanes: RwSignal<Vec<LaneView>>,
    handlers: TaskCardHandlers,
    busy: TaskCardBusy,
) -> impl IntoView {
    let anchor: String = format!("lane-{workspace_id}");
    let lane: Memo<Option<LaneView>> = Memo::new(move |_| {
        lanes.with(|all: &Vec<LaneView>| {
            all.iter()
                .find(|l: &&LaneView| l.workspace_id == workspace_id)
                .cloned()
        })
    });

    view! {
        <section
            id=anchor
            class="flex h-full min-h-0 w-[85vw] shrink-0 snap-start flex-col gap-2 sm:w-80"
        >
            <header class="flex shrink-0 items-start justify-between gap-2">
                <div class="stack gap-0.5">
                    <h2 class="font-semibold text-fg">
                        {move || lane.with(|l: &Option<LaneView>| l.as_ref().map(|l: &LaneView| l.workspace_name.clone()))}
                    </h2>
                    <code class="text-xs text-muted">
                        {move || lane.with(|l: &Option<LaneView>| l.as_ref().map(|l: &LaneView| l.base_branch.clone()))}
                    </code>
                </div>
                <div class="row gap-1.5">
                    <span class="badge badge-muted">
                        {move || lane.with(|l: &Option<LaneView>| l.as_ref().map(|l: &LaneView| l.total))}
                    </span>
                    {move || {
                        let attention: usize = lane
                            .with(|l: &Option<LaneView>| l.as_ref().map_or(0, |l: &LaneView| l.attention_count));
                        (attention > 0).then(|| view! { <span class="badge badge-attention">{attention}</span> })
                    }}
                </div>
            </header>
            <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pr-1">
                <For
                    each=move || {
                        lane.with(|l: &Option<LaneView>| {
                            l.as_ref().map(|l: &LaneView| l.tasks.clone()).unwrap_or_default()
                        })
                    }
                    key=|task: &TaskDto| card_key(task)
                    children=move |task: TaskDto| view! { <TaskCard task handlers busy /> }
                />
            </div>
        </section>
    }
}
