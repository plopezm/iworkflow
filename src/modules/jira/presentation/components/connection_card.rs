use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::JiraConnectionDto;
use crate::modules::jira::presentation::components::connection_form::JiraConnectionForm;
use crate::modules::jira::presentation::components::connection_summary::JiraConnectionSummary;
use crate::modules::jira::presentation::state::jira_connection_state::JiraConnectionState;

/// Card to view, set, edit and remove the Jira connection. Shared by the Jira and the
/// settings screens.
#[component]
pub fn JiraConnectionCard(state: JiraConnectionState) -> impl IntoView {
    view! {
        <Card title="Conexión con Jira">
            <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                {move || Suspend::new(async move {
                    match state.connection.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(current) => connection_panel(state, current).into_any(),
                    }
                })}
            </Transition>
        </Card>
    }
}

fn connection_panel(
    state: JiraConnectionState,
    current: Option<JiraConnectionDto>,
) -> impl IntoView {
    let stored_url: Option<String> = current
        .as_ref()
        .map(|c: &JiraConnectionDto| c.base_url.clone());
    let initial: Option<JiraConnectionDto> = current.clone();
    view! {
        {move || {
            if state.editing.get() || initial.is_none() {
                view! { <JiraConnectionForm state stored_url=stored_url.clone() /> }.into_any()
            } else if let Some(connection) = current.clone() {
                view! { <JiraConnectionSummary connection state /> }.into_any()
            } else {
                ().into_any()
            }
        }}
        <ActionError result=state.delete.value().into() />
    }
}

/// The connection card with its own state, for screens that only need the settings.
#[component]
pub fn JiraSettingsSection() -> impl IntoView {
    let state: JiraConnectionState = JiraConnectionState::new();
    view! { <JiraConnectionCard state /> }
}
