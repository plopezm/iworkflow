use leptos::prelude::*;

use crate::core::components::form::{ActionError, SelectField, SelectOption};
use crate::core::components::layout::{Card, ErrorText};
use crate::core::dock::context::{DockContext, use_dock, use_tab_params};
use crate::core::dock::link::TabLink;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    IssueLocalStateDto, JiraCommentDto, JiraIssueDto, JiraTransitionDto,
};
use crate::modules::jira::presentation::components::local_state_badge::LocalStateBadge;
use crate::modules::jira::presentation::state::formatting::short_timestamp;
use crate::modules::jira::presentation::state::jira_issue_state::JiraIssueState;

/// Detalle de una tarea importada. Se reconstruye por cada clave.
#[component]
pub fn JiraIssuePage() -> impl IntoView {
    let params = use_tab_params();
    let key: Memo<String> = Memo::new(move |_| params.with(|p| p.get("key").unwrap_or_default()));
    view! { {move || view! { <JiraIssueContent key=key.get() /> }} }
}

#[component]
fn JiraIssueContent(key: String) -> impl IntoView {
    let state: JiraIssueState = JiraIssueState::new(key.clone());
    let dock: DockContext = use_dock();
    let tab_path: String = format!("/jira/{key}");
    Effect::new(move |_| {
        if let Some(Ok(issue)) = state.issue.get() {
            dock.set_title(&tab_path, &issue.key);
        }
    });

    view! {
        <div class="h-full min-h-0 overflow-y-auto">
            <div class="mx-auto stack w-full max-w-5xl gap-4 p-4 pb-10 md:p-6 md:pb-12">
                <Transition fallback=|| view! { <p class="muted">"Cargando tarea…"</p> }>
                    {move || Suspend::new(async move {
                        match state.issue.await {
                            Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                            Ok(issue) => issue_view(state, issue).into_any(),
                        }
                    })}
                </Transition>
            </div>
        </div>
    }
}

fn issue_view(state: JiraIssueState, issue: JiraIssueDto) -> impl IntoView {
    let convert_path: String = format!("/jira/{}/convert", issue.key);
    let task_path: Option<String> = issue
        .task_id
        .as_ref()
        .map(|id: &String| format!("/tasks/{id}"));
    let local: IssueLocalStateDto = issue.local_state;
    let description: String = if issue.description.trim().is_empty() {
        "(sin descripción)".to_string()
    } else {
        issue.description.clone()
    };
    view! {
        <Card>
            <div class="row flex-wrap items-center gap-2">
                <h1 class="page-title">{format!("{}: {}", issue.key, issue.summary)}</h1>
            </div>
            <div class="row flex-wrap items-center gap-2 text-sm">
                <span class="badge badge-warn">{issue.status.clone()}</span>
                <LocalStateBadge state=local />
                <span class="muted">{issue.issue_type.clone()}</span>
                {issue.priority.clone().map(|p: String| view! { <span class="muted">{p}</span> })}
            </div>
            <dl class="grid gap-x-4 gap-y-1 text-sm md:grid-cols-2">
                <div>
                    <dt class="muted inline">"Responsable: "</dt>
                    <dd class="inline">{issue.assignee.clone().unwrap_or_else(|| "—".to_string())}</dd>
                </div>
                <div>
                    <dt class="muted inline">"Informador: "</dt>
                    <dd class="inline">{issue.reporter.clone().unwrap_or_else(|| "—".to_string())}</dd>
                </div>
                <div>
                    <dt class="muted inline">"Actualizada en Jira: "</dt>
                    <dd class="inline">{short_timestamp(&issue.jira_updated_at)}</dd>
                </div>
                <div>
                    <dt class="muted inline">"Etiquetas: "</dt>
                    <dd class="inline">
                        {if issue.labels.is_empty() { "—".to_string() } else { issue.labels.join(", ") }}
                    </dd>
                </div>
            </dl>
            <div class="row flex-wrap gap-2">
                {match (local, task_path) {
                    (IssueLocalStateDto::Converted, Some(path)) => {
                        view! { <TabLink href=path class="btn-secondary">"Ver tarea de iworkflow"</TabLink> }
                            .into_any()
                    }
                    (IssueLocalStateDto::Converted, None) => ().into_any(),
                    _ => {
                        view! { <TabLink href=convert_path class="btn-primary">"Convertir en tarea"</TabLink> }
                            .into_any()
                    }
                }}
                {match local {
                    IssueLocalStateDto::New => {
                        view! {
                            <button class="btn-ghost" on:click=move |_| state.set_ignored(true)>
                                "Ignorar"
                            </button>
                        }
                            .into_any()
                    }
                    IssueLocalStateDto::Ignored => {
                        view! {
                            <button class="btn-ghost" on:click=move |_| state.set_ignored(false)>
                                "Restaurar"
                            </button>
                        }
                            .into_any()
                    }
                    IssueLocalStateDto::Converted => {
                        view! {
                            <button class="btn-ghost" on:click=move |_| state.set_ignored(false)>
                                "Marcar como nueva"
                            </button>
                        }
                            .into_any()
                    }
                }}
                <a
                    href=issue.url.clone()
                    target="_blank"
                    rel="noopener noreferrer"
                    class="btn-ghost"
                >
                    "Abrir en Jira"
                </a>
            </div>
            <ActionError result=state.ignore.value().into() />
        </Card>
        <Card title="Descripción">
            <pre class="whitespace-pre-wrap break-words text-sm">{description}</pre>
        </Card>
        <Card title="Cambiar estado en Jira">
            <Transition fallback=|| view! { <p class="muted">"Consultando transiciones…"</p> }>
                {move || Suspend::new(async move {
                    match state.transitions.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(transitions) => transition_form(state, transitions).into_any(),
                    }
                })}
            </Transition>
            <ActionError result=state.transition.value().into() />
        </Card>
        <Card title="Comentarios">
            <Transition fallback=|| view! { <p class="muted">"Cargando comentarios…"</p> }>
                {move || Suspend::new(async move {
                    match state.comments.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(comments) => comment_list(comments).into_any(),
                    }
                })}
            </Transition>
            <form
                class="stack"
                on:submit=move |ev: leptos::ev::SubmitEvent| {
                    ev.prevent_default();
                    state.send_comment();
                }
            >
                <textarea
                    class="input min-h-24"
                    placeholder="Escribe un comentario para Jira…"
                    prop:value=move || state.comment_text.get()
                    on:input=move |ev| state.comment_text.set(event_target_value(&ev))
                ></textarea>
                <ActionError result=state.add_comment.value().into() />
                <div class="row justify-end">
                    <button
                        type="submit"
                        class="btn-primary"
                        disabled=move || {
                            state.add_comment.pending().get()
                                || state.comment_text.with(|t: &String| t.trim().is_empty())
                        }
                    >
                        {move || {
                            if state.add_comment.pending().get() { "Enviando…" } else { "Comentar en Jira" }
                        }}
                    </button>
                </div>
            </form>
        </Card>
    }
}

fn transition_form(state: JiraIssueState, transitions: Vec<JiraTransitionDto>) -> impl IntoView {
    if transitions.is_empty() {
        return view! { <p class="muted">"Jira no permite cambiar el estado ahora."</p> }
            .into_any();
    }
    let options: Vec<SelectOption> = transitions
        .into_iter()
        .map(|t: JiraTransitionDto| SelectOption {
            label: format!("{} → {}", t.name, t.to_status),
            value: t.id,
        })
        .collect();
    view! {
        <div class="row flex-wrap items-end gap-2">
            <div class="min-w-0 flex-1">
                <SelectField
                    label="Transición"
                    value=state.selected_transition
                    options=options
                    placeholder="Elige una transición…"
                />
            </div>
            <button
                class="btn-primary"
                disabled=move || {
                    state.transition.pending().get() || state.selected_transition.with(|t: &String| t.is_empty())
                }
                on:click=move |_| state.apply_transition()
            >
                {move || if state.transition.pending().get() { "Aplicando…" } else { "Aplicar" }}
            </button>
        </div>
    }
    .into_any()
}

fn comment_list(comments: Vec<JiraCommentDto>) -> impl IntoView {
    if comments.is_empty() {
        return view! { <p class="muted">"Sin comentarios."</p> }.into_any();
    }
    view! {
        <ul class="stack gap-3">
            {comments
                .into_iter()
                .map(|c: JiraCommentDto| {
                    view! {
                        <li class="stack gap-1 border-l-2 border-border pl-3">
                            <p class="text-xs">
                                <strong>{c.author}</strong>
                                <span class="muted">{format!(" · {}", short_timestamp(&c.created_at))}</span>
                            </p>
                            <pre class="whitespace-pre-wrap break-words text-sm">{c.body}</pre>
                        </li>
                    }
                })
                .collect_view()}
        </ul>
    }
    .into_any()
}
