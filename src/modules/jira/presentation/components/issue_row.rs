use leptos::prelude::*;

use crate::core::components::layout::Badge;
use crate::core::dock::link::TabLink;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    IssueLocalStateDto, JiraIssueDto,
};
use crate::modules::jira::presentation::components::local_state_badge::LocalStateBadge;
use crate::modules::jira::presentation::state::formatting::short_timestamp;

/// One imported issue in the list, with its quick actions.
#[component]
pub fn IssueRow(
    issue: JiraIssueDto,
    /// `(key, ignored)`: ignore or restore the issue.
    on_ignore: Callback<(String, bool)>,
) -> impl IntoView {
    let detail_path: String = format!("/jira/{}", issue.key);
    let convert_path: String = format!("/jira/{}/convert", issue.key);
    let task_path: Option<String> = issue
        .task_id
        .as_ref()
        .map(|id: &String| format!("/tasks/{id}"));
    let state: IssueLocalStateDto = issue.local_state;
    let key: String = issue.key.clone();
    view! {
        <li class="card stack gap-2">
            <div class="row flex-wrap items-center gap-2">
                <TabLink href=detail_path class="font-semibold text-accent">
                    {issue.key.clone()}
                </TabLink>
                <span class="min-w-0 flex-1 break-words text-sm font-medium">{issue.summary}</span>
            </div>
            <div class="row flex-wrap items-center gap-2 text-xs">
                <Badge text=issue.status variant="warn" />
                <LocalStateBadge state />
                <span class="muted">{issue.issue_type}</span>
                {issue.priority.map(|p: String| view! { <span class="muted">{p}</span> })}
                {issue
                    .assignee
                    .map(|a: String| view! { <span class="muted">{format!("→ {a}")}</span> })}
                <span class="muted">{short_timestamp(&issue.jira_updated_at)}</span>
            </div>
            <div class="row flex-wrap gap-2">
                {match (state, task_path) {
                    (IssueLocalStateDto::Converted, Some(path)) => {
                        view! {
                            <TabLink href=path class="btn-secondary min-h-8 px-3 py-1 text-xs">
                                "Ver tarea"
                            </TabLink>
                        }
                            .into_any()
                    }
                    (IssueLocalStateDto::Converted, None) => ().into_any(),
                    _ => {
                        view! {
                            <TabLink href=convert_path class="btn-primary min-h-8 px-3 py-1 text-xs">
                                "Convertir en tarea"
                            </TabLink>
                        }
                            .into_any()
                    }
                }}
                {match state {
                    IssueLocalStateDto::New => {
                        let key: String = key.clone();
                        view! {
                            <button
                                class="btn-ghost min-h-8 px-3 py-1 text-xs"
                                on:click=move |_| on_ignore.run((key.clone(), true))
                            >
                                "Ignorar"
                            </button>
                        }
                            .into_any()
                    }
                    IssueLocalStateDto::Ignored => {
                        let key: String = key.clone();
                        view! {
                            <button
                                class="btn-ghost min-h-8 px-3 py-1 text-xs"
                                on:click=move |_| on_ignore.run((key.clone(), false))
                            >
                                "Restaurar"
                            </button>
                        }
                            .into_any()
                    }
                    IssueLocalStateDto::Converted => ().into_any(),
                }}
                <a
                    href=issue.url
                    target="_blank"
                    rel="noopener noreferrer"
                    class="btn-ghost min-h-8 px-3 py-1 text-xs"
                >
                    "Abrir en Jira"
                </a>
            </div>
        </li>
    }
}
