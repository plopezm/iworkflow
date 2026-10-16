use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::JiraConnectionDto;
use crate::modules::jira::presentation::state::formatting::short_timestamp;
use crate::modules::jira::presentation::state::jira_connection_state::JiraConnectionState;

/// The stored connection with edit / disconnect actions.
#[component]
pub fn JiraConnectionSummary(
    connection: JiraConnectionDto,
    state: JiraConnectionState,
) -> impl IntoView {
    let for_edit: JiraConnectionDto = connection.clone();
    let last_sync: String = connection
        .last_sync_at
        .as_deref()
        .map(short_timestamp)
        .unwrap_or_else(|| "nunca".to_string());
    view! {
        <div class="stack gap-2">
            <p class="text-sm">
                <span class="muted">"Servidor: "</span>
                <strong>{connection.base_url}</strong>
            </p>
            <p class="text-sm">
                <span class="muted">"Última sincronización: "</span>
                {last_sync}
            </p>
            {connection
                .last_sync_error
                .map(|e: String| view! { <ErrorText message=format!("Último fallo: {e}") /> })}
            <div class="row flex-wrap gap-2">
                <button
                    class="btn-secondary"
                    on:click=move |_| state.begin_edit(Some(&for_edit))
                >
                    "Editar conexión"
                </button>
                {move || {
                    if state.confirming_delete.get() {
                        view! {
                            <span class="row items-center gap-2 text-sm">
                                "Se olvidan el token y las tareas importadas."
                                <button
                                    class="btn-danger"
                                    disabled=move || state.delete.pending().get()
                                    on:click=move |_| state.disconnect()
                                >
                                    "Desconectar"
                                </button>
                                <button
                                    class="btn-ghost"
                                    on:click=move |_| state.confirming_delete.set(false)
                                >
                                    "Cancelar"
                                </button>
                            </span>
                        }
                            .into_any()
                    } else {
                        view! {
                            <button
                                class="btn-ghost"
                                on:click=move |_| state.confirming_delete.set(true)
                            >
                                "Desconectar"
                            </button>
                        }
                            .into_any()
                    }
                }}
            </div>
        </div>
    }
}
