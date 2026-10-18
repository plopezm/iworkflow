use leptos::prelude::*;

use crate::core::components::layout::Badge;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::IssueLocalStateDto;
use crate::modules::jira::presentation::state::formatting::{
    local_state_label, local_state_variant,
};

/// What iworkflow has done with the issue (nueva / convertida / ignorada).
#[component]
pub fn LocalStateBadge(state: IssueLocalStateDto) -> impl IntoView {
    view! { <Badge text=local_state_label(state) variant=local_state_variant(state) /> }
}
