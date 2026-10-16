use leptos::prelude::*;

use crate::core::components::form::ActionError;
use crate::core::components::layout::{Card, ErrorText};
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    JiraIssueListDto, JiraLaneDto, JiraLaneListingDto, SavedLaneDto,
};
use crate::modules::jira::presentation::components::connection_card::JiraConnectionCard;
use crate::modules::jira::presentation::components::issue_filters::IssueFilters;
use crate::modules::jira::presentation::components::lane_form::LaneForm;
use crate::modules::jira::presentation::components::lane_section::LaneSection;
use crate::modules::jira::presentation::state::formatting::sync_message;
use crate::modules::jira::presentation::state::jira_connection_state::JiraConnectionState;
use crate::modules::jira::presentation::state::jira_issues_state::{
    JiraIssuesState, LaneFormMode, lane_sync_notice,
};

/// Conexión con Jira y carriles (consultas JQL) con las tareas importadas.
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
                <JiraConnectionCard state=connection />
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
    let failed_lanes: Memo<Vec<String>> = Memo::new(move |_| {
        state
            .sync
            .value()
            .get()
            .and_then(Result::ok)
            .map(|outcome| outcome.failed_lanes)
            .unwrap_or_default()
    });
    let lane_notice: Memo<Option<String>> = Memo::new(move |_| {
        let saved: Option<SavedLaneDto> = match (
            state.create_lane.value().get(),
            state.update_lane.value().get(),
        ) {
            (_, Some(Ok(saved))) => Some(saved),
            (Some(Ok(saved)), _) => Some(saved),
            _ => None,
        };
        saved.as_ref().and_then(lane_sync_notice)
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
                <button
                    class="btn-secondary"
                    disabled=move || state.lane_form.get() != LaneFormMode::Closed
                    on:click=move |_| state.begin_create_lane()
                >
                    "+ Nuevo carril"
                </button>
                {move || sync_status.get().map(|m: String| view! { <span class="muted text-sm">{m}</span> })}
            </div>
            {move || {
                failed_lanes
                    .get()
                    .into_iter()
                    .map(|m: String| view! { <ErrorText message=format!("Carril {m}") /> })
                    .collect_view()
            }}
            {move || lane_notice.get().map(|m: String| view! { <ErrorText message=m /> })}
            <ActionError result=state.sync.value().into() />
            <ActionError result=state.ignore.value().into() />
            <ActionError result=state.delete_lane.value().into() />
            <IssueFilters state />
        </Card>
        {move || (state.lane_form.get() != LaneFormMode::Closed).then(|| view! { <LaneForm state /> })}
        <Transition fallback=|| view! { <p class="muted">"Cargando tareas…"</p> }>
            {move || Suspend::new(async move {
                match state.issues.await {
                    Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                    Ok(list) => lanes_view(state, list, on_ignore).into_any(),
                }
            })}
        </Transition>
    }
}

fn lanes_view(
    state: JiraIssuesState,
    list: JiraIssueListDto,
    on_ignore: Callback<(String, bool)>,
) -> impl IntoView {
    let on_toggle: Callback<(String, bool)> =
        Callback::new(move |(id, collapsed): (String, bool)| state.toggle_lane(id, collapsed));
    let on_edit: Callback<JiraLaneDto> =
        Callback::new(move |lane: JiraLaneDto| state.begin_edit_lane(&lane));
    let on_delete: Callback<String> = Callback::new(move |id: String| state.remove_lane(id));
    let deleting: Signal<bool> = state.delete_lane.pending().into();
    view! {
        <div class="stack gap-4">
            {list
                .lanes
                .into_iter()
                .map(|listing: JiraLaneListingDto| {
                    view! {
                        <LaneSection
                            listing
                            on_ignore
                            on_toggle
                            on_edit
                            on_delete
                            deleting
                        />
                    }
                })
                .collect_view()}
        </div>
    }
}
