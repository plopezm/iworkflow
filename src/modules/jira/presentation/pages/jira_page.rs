use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    JiraConnectionDto, JiraIssueDto, JiraIssueListDto,
};
use crate::modules::jira::presentation::components::connection_form::JiraConnectionForm;
use crate::modules::jira::presentation::components::connection_summary::JiraConnectionSummary;
use crate::modules::jira::presentation::components::issue_filters::IssueFilters;
use crate::modules::jira::presentation::components::issue_row::IssueRow;
use crate::modules::jira::presentation::state::formatting::sync_message;
use crate::modules::jira::presentation::state::jira_connection_state::JiraConnectionState;
use crate::modules::jira::presentation::state::jira_issues_state::JiraIssuesState;

/// Conexión con Jira y listado de las tareas importadas.
#[component]
pub fn JiraPage() -> impl IntoView {
    let connection: JiraConnectionState = JiraConnectionState::new();
    let issues: JiraIssuesState = JiraIssuesState::new(connection.version());
    let on_ignore: Callback<(String, bool)> =
        Callback::new(move |(key, ignored): (String, bool)| issues.set_ignored(key, ignored));

    view! {
        <div class="flex h-full min-h-0 flex-col">
            <div class="flex shrink-0 items-center border-b border-border bg-surface px-4 py-2.5">
                <h1 class="page-title">"Jira"</h1>
            </div>
            <div class="stack min-h-0 flex-1 overflow-y-auto p-4">
                <Card title="Conexión">
                    <Transition fallback=|| view! { <p class="muted">"Cargando…"</p> }>
                        {move || Suspend::new(async move {
                            match connection.connection.await {
                                Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                                Ok(current) => {
                                    connection_panel(connection, current).into_any()
                                }
                            }
                        })}
                    </Transition>
                </Card>
                <Transition fallback=|| ()>
                    {move || Suspend::new(async move {
                        match connection.connection.await {
                            Ok(Some(_)) => issues_panel(issues, on_ignore).into_any(),
                            _ => ().into_any(),
                        }
                    })}
                </Transition>
            </div>
        </div>
    }
}

fn connection_panel(
    state: JiraConnectionState,
    current: Option<JiraConnectionDto>,
) -> impl IntoView {
    let token_stored: bool = current.is_some();
    let initial: Option<JiraConnectionDto> = current.clone();
    view! {
        {move || {
            if state.editing.get() || initial.is_none() {
                view! { <JiraConnectionForm state token_stored /> }.into_any()
            } else if let Some(connection) = current.clone() {
                view! { <JiraConnectionSummary connection state /> }.into_any()
            } else {
                ().into_any()
            }
        }}
        <ActionError result=state.delete.value().into() />
    }
}

fn issues_panel(state: JiraIssuesState, on_ignore: Callback<(String, bool)>) -> impl IntoView {
    let syncing: Signal<bool> = state.sync.pending().into();
    let sync_status: Memo<Option<String>> = Memo::new(move |_| {
        state
            .sync
            .value()
            .get()
            .and_then(Result::ok)
            .map(|outcome| sync_message(&outcome))
    });
    view! {
        <Card title="Tareas importadas">
            <div class="row flex-wrap items-center gap-2">
                <button
                    class="btn-primary"
                    disabled=move || syncing.get()
                    on:click=move |_| state.start_sync()
                >
                    {move || if syncing.get() { "Sincronizando…" } else { "Sincronizar" }}
                </button>
                {move || sync_status.get().map(|m: String| view! { <span class="muted text-sm">{m}</span> })}
            </div>
            <ActionError result=state.sync.value().into() />
            <ActionError result=state.ignore.value().into() />
            <IssueFilters state />
            <Transition fallback=|| view! { <p class="muted">"Cargando tareas…"</p> }>
                {move || Suspend::new(async move {
                    match state.issues.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(list) => issue_list(list, on_ignore).into_any(),
                    }
                })}
            </Transition>
        </Card>
    }
}

fn issue_list(list: JiraIssueListDto, on_ignore: Callback<(String, bool)>) -> impl IntoView {
    if list.total == 0 {
        return view! {
            <p class="muted">"No hay tareas importadas. Pulsa «Sincronizar» para traerlas de Jira."</p>
        }
        .into_any();
    }
    if list.issues.is_empty() {
        return view! { <p class="muted">"Ninguna tarea coincide con los filtros."</p> }.into_any();
    }
    let count: String = format!("{} de {} tareas", list.issues.len(), list.total);
    view! {
        <p class="muted text-sm">{count}</p>
        <ul class="stack gap-3">
            {list
                .issues
                .into_iter()
                .map(|issue: JiraIssueDto| view! { <IssueRow issue on_ignore /> })
                .collect_view()}
        </ul>
    }
    .into_any()
}
