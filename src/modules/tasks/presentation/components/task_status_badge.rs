use leptos::prelude::*;

use crate::core::components::layout::Badge;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStatusDto;

#[component]
pub fn TaskStatusBadge(status: TaskStatusDto) -> impl IntoView {
    view! { <Badge text=status.label() variant=status.variant() /> }
}
