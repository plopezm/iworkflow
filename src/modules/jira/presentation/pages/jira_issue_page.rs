use leptos::prelude::*;

use crate::core::components::form::{ActionError, SelectField, SelectOption};
use crate::core::components::layout::{Card, ErrorText};
use crate::core::dock::context::{DockContext, use_dock, use_tab_params};
use crate::core::dock::link::TabLink;
use crate::core::i18n::Msg;
use crate::modules::jira::infrastructure::adapters::inbound::dto::jira_dto::{
    IssueLocalStateDto, JiraCommentDto, JiraIssueDto, JiraTransitionDto,
};
use crate::modules::jira::presentation::components::local_state_badge::LocalStateBadge;
use crate::modules::jira::presentation::state::formatting::short_timestamp;
use crate::modules::jira::presentation::state::jira_issue_state::JiraIssueState;

const LOADING_ISSUE: Msg = Msg {
    en: "Loading issue…",
    es: "Cargando tarea…",
    zh: "正在加载任务…",
    de: "Aufgabe wird geladen…",
    fr: "Chargement de la tâche…",
};

const NO_DESCRIPTION: Msg = Msg {
    en: "(no description)",
    es: "(sin descripción)",
    zh: "（无描述）",
    de: "(keine Beschreibung)",
    fr: "(sans description)",
};

const ASSIGNEE: Msg = Msg {
    en: "Assignee: ",
    es: "Responsable: ",
    zh: "负责人：",
    de: "Bearbeiter: ",
    fr: "Responsable : ",
};

const REPORTER: Msg = Msg {
    en: "Reporter: ",
    es: "Informador: ",
    zh: "报告人：",
    de: "Melder: ",
    fr: "Rapporteur : ",
};

const UPDATED_IN_JIRA: Msg = Msg {
    en: "Updated in Jira: ",
    es: "Actualizada en Jira: ",
    zh: "Jira 中的更新时间：",
    de: "In Jira aktualisiert: ",
    fr: "Mise à jour dans Jira : ",
};

const LABELS: Msg = Msg {
    en: "Labels: ",
    es: "Etiquetas: ",
    zh: "标签：",
    de: "Labels: ",
    fr: "Étiquettes : ",
};

const VIEW_IWORKFLOW_TASK: Msg = Msg {
    en: "View iworkflow task",
    es: "Ver tarea de iworkflow",
    zh: "查看 iworkflow 任务",
    de: "iworkflow-Aufgabe ansehen",
    fr: "Voir la tâche iworkflow",
};

const CONVERT: Msg = Msg {
    en: "Convert to task",
    es: "Convertir en tarea",
    zh: "转换为任务",
    de: "In Aufgabe umwandeln",
    fr: "Convertir en tâche",
};

const IGNORE: Msg = Msg {
    en: "Ignore",
    es: "Ignorar",
    zh: "忽略",
    de: "Ignorieren",
    fr: "Ignorer",
};

const RESTORE: Msg = Msg {
    en: "Restore",
    es: "Restaurar",
    zh: "恢复",
    de: "Wiederherstellen",
    fr: "Restaurer",
};

const MARK_AS_NEW: Msg = Msg {
    en: "Mark as new",
    es: "Marcar como nueva",
    zh: "标记为新建",
    de: "Als neu markieren",
    fr: "Marquer comme nouvelle",
};

const OPEN_IN_JIRA: Msg = Msg {
    en: "Open in Jira",
    es: "Abrir en Jira",
    zh: "在 Jira 中打开",
    de: "In Jira öffnen",
    fr: "Ouvrir dans Jira",
};

const DESCRIPTION: Msg = Msg {
    en: "Description",
    es: "Descripción",
    zh: "描述",
    de: "Beschreibung",
    fr: "Description",
};

const CHANGE_STATUS: Msg = Msg {
    en: "Change status in Jira",
    es: "Cambiar estado en Jira",
    zh: "更改 Jira 中的状态",
    de: "Status in Jira ändern",
    fr: "Changer le statut dans Jira",
};

const LOADING_TRANSITIONS: Msg = Msg {
    en: "Checking transitions…",
    es: "Consultando transiciones…",
    zh: "正在查询状态转换…",
    de: "Übergänge werden abgefragt…",
    fr: "Consultation des transitions…",
};

const COMMENTS: Msg = Msg {
    en: "Comments",
    es: "Comentarios",
    zh: "评论",
    de: "Kommentare",
    fr: "Commentaires",
};

const LOADING_COMMENTS: Msg = Msg {
    en: "Loading comments…",
    es: "Cargando comentarios…",
    zh: "正在加载评论…",
    de: "Kommentare werden geladen…",
    fr: "Chargement des commentaires…",
};

const COMMENT_PLACEHOLDER: Msg = Msg {
    en: "Write a comment for Jira…",
    es: "Escribe un comentario para Jira…",
    zh: "为 Jira 撰写评论…",
    de: "Schreibe einen Kommentar für Jira…",
    fr: "Écrivez un commentaire pour Jira…",
};

const SENDING: Msg = Msg {
    en: "Sending…",
    es: "Enviando…",
    zh: "发送中…",
    de: "Wird gesendet…",
    fr: "Envoi…",
};

const COMMENT_IN_JIRA: Msg = Msg {
    en: "Comment in Jira",
    es: "Comentar en Jira",
    zh: "在 Jira 中评论",
    de: "In Jira kommentieren",
    fr: "Commenter dans Jira",
};

const NO_TRANSITIONS: Msg = Msg {
    en: "Jira does not allow changing the status right now.",
    es: "Jira no permite cambiar el estado ahora.",
    zh: "Jira 目前不允许更改状态。",
    de: "Jira erlaubt derzeit keine Statusänderung.",
    fr: "Jira n'autorise pas de changer le statut pour le moment.",
};

const TRANSITION: Msg = Msg {
    en: "Transition",
    es: "Transición",
    zh: "状态转换",
    de: "Übergang",
    fr: "Transition",
};

const PICK_TRANSITION: Msg = Msg {
    en: "Pick a transition…",
    es: "Elige una transición…",
    zh: "选择状态转换…",
    de: "Übergang auswählen…",
    fr: "Choisissez une transition…",
};

const APPLYING: Msg = Msg {
    en: "Applying…",
    es: "Aplicando…",
    zh: "应用中…",
    de: "Wird angewendet…",
    fr: "Application…",
};

const APPLY: Msg = Msg {
    en: "Apply",
    es: "Aplicar",
    zh: "应用",
    de: "Anwenden",
    fr: "Appliquer",
};

const NO_COMMENTS: Msg = Msg {
    en: "No comments.",
    es: "Sin comentarios.",
    zh: "暂无评论。",
    de: "Keine Kommentare.",
    fr: "Aucun commentaire.",
};

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
                <Transition fallback=|| view! { <p class="muted">{LOADING_ISSUE.get()}</p> }>
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
        NO_DESCRIPTION.get().to_string()
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
                    <dt class="muted inline">{ASSIGNEE.get()}</dt>
                    <dd class="inline">{issue.assignee.clone().unwrap_or_else(|| "—".to_string())}</dd>
                </div>
                <div>
                    <dt class="muted inline">{REPORTER.get()}</dt>
                    <dd class="inline">{issue.reporter.clone().unwrap_or_else(|| "—".to_string())}</dd>
                </div>
                <div>
                    <dt class="muted inline">{UPDATED_IN_JIRA.get()}</dt>
                    <dd class="inline">{short_timestamp(&issue.jira_updated_at)}</dd>
                </div>
                <div>
                    <dt class="muted inline">{LABELS.get()}</dt>
                    <dd class="inline">
                        {if issue.labels.is_empty() { "—".to_string() } else { issue.labels.join(", ") }}
                    </dd>
                </div>
            </dl>
            <div class="row flex-wrap gap-2">
                {match (local, task_path) {
                    (IssueLocalStateDto::Converted, Some(path)) => {
                        view! { <TabLink href=path class="btn-secondary">{VIEW_IWORKFLOW_TASK.get()}</TabLink> }
                            .into_any()
                    }
                    (IssueLocalStateDto::Converted, None) => ().into_any(),
                    _ => {
                        view! { <TabLink href=convert_path class="btn-primary">{CONVERT.get()}</TabLink> }
                            .into_any()
                    }
                }}
                {match local {
                    IssueLocalStateDto::New => {
                        view! {
                            <button class="btn-ghost" on:click=move |_| state.set_ignored(true)>
                                {IGNORE.get()}
                            </button>
                        }
                            .into_any()
                    }
                    IssueLocalStateDto::Ignored => {
                        view! {
                            <button class="btn-ghost" on:click=move |_| state.set_ignored(false)>
                                {RESTORE.get()}
                            </button>
                        }
                            .into_any()
                    }
                    IssueLocalStateDto::Converted => {
                        view! {
                            <button class="btn-ghost" on:click=move |_| state.set_ignored(false)>
                                {MARK_AS_NEW.get()}
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
                    {OPEN_IN_JIRA.get()}
                </a>
            </div>
            <ActionError result=state.ignore.value().into() />
        </Card>
        <Card title=DESCRIPTION.get()>
            <pre class="whitespace-pre-wrap break-words text-sm">{description}</pre>
        </Card>
        <Card title=CHANGE_STATUS.get()>
            <Transition fallback=|| view! { <p class="muted">{LOADING_TRANSITIONS.get()}</p> }>
                {move || Suspend::new(async move {
                    match state.transitions.await {
                        Err(e) => view! { <ErrorText message=e.to_string() /> }.into_any(),
                        Ok(transitions) => transition_form(state, transitions).into_any(),
                    }
                })}
            </Transition>
            <ActionError result=state.transition.value().into() />
        </Card>
        <Card title=COMMENTS.get()>
            <Transition fallback=|| view! { <p class="muted">{LOADING_COMMENTS.get()}</p> }>
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
                    placeholder=COMMENT_PLACEHOLDER.get()
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
                            if state.add_comment.pending().get() { SENDING.get() } else { COMMENT_IN_JIRA.get() }
                        }}
                    </button>
                </div>
            </form>
        </Card>
    }
}

fn transition_form(state: JiraIssueState, transitions: Vec<JiraTransitionDto>) -> impl IntoView {
    if transitions.is_empty() {
        return view! { <p class="muted">{NO_TRANSITIONS.get()}</p> }.into_any();
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
                    label=TRANSITION.get()
                    value=state.selected_transition
                    options=options
                    placeholder=PICK_TRANSITION.get()
                />
            </div>
            <button
                class="btn-primary"
                disabled=move || {
                    state.transition.pending().get() || state.selected_transition.with(|t: &String| t.is_empty())
                }
                on:click=move |_| state.apply_transition()
            >
                {move || if state.transition.pending().get() { APPLYING.get() } else { APPLY.get() }}
            </button>
        </div>
    }
    .into_any()
}

fn comment_list(comments: Vec<JiraCommentDto>) -> impl IntoView {
    if comments.is_empty() {
        return view! { <p class="muted">{NO_COMMENTS.get()}</p> }.into_any();
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
