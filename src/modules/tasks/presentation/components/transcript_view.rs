use leptos::prelude::*;

use crate::core::i18n::Msg;

use crate::core::components::layout::ErrorText;
use crate::core::markdown::render_markdown;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::NoticeLevelDto;
use crate::modules::tasks::presentation::components::task_status_badge::TaskStatusBadge;
use crate::modules::tasks::presentation::state::transcript::{
    Transcript, TranscriptItem, images_label,
};

const LAUNCH_DETAILS: Msg = Msg {
    en: "Launch details",
    es: "Detalles de arranque",
    zh: "启动详情",
    de: "Startdetails",
    fr: "Détails de démarrage",
};

const ROLE_USER: Msg = Msg {
    en: "You",
    es: "Tú",
    zh: "你",
    de: "Du",
    fr: "Vous",
};

const ROLE_AGENT: Msg = Msg {
    en: "Agent",
    es: "Agente",
    zh: "智能体",
    de: "Agent",
    fr: "Agent",
};

const REASONING: Msg = Msg {
    en: "Reasoning",
    es: "Razonamiento",
    zh: "推理过程",
    de: "Überlegungen",
    fr: "Raisonnement",
};

const TOOL_RUNNING: Msg = Msg {
    en: "running",
    es: "en curso",
    zh: "进行中",
    de: "läuft",
    fr: "en cours",
};

const TOOL_OK: Msg = Msg {
    en: "ok",
    es: "ok",
    zh: "成功",
    de: "ok",
    fr: "ok",
};

const TOOL_ERROR: Msg = Msg {
    en: "error",
    es: "error",
    zh: "错误",
    de: "Fehler",
    fr: "erreur",
};

const DIALOG_ANSWERED: Msg = Msg {
    en: "answered",
    es: "respondido",
    zh: "已回答",
    de: "beantwortet",
    fr: "répondu",
};

const DIALOG_PENDING: Msg = Msg {
    en: "pending",
    es: "pendiente",
    zh: "待处理",
    de: "ausstehend",
    fr: "en attente",
};

const AGENT_STOPPED_CODE: Msg = Msg {
    en: "Agent stopped (code {code})",
    es: "Agente detenido (código {code})",
    zh: "智能体已停止（代码 {code}）",
    de: "Agent angehalten (Code {code})",
    fr: "Agent arrêté (code {code})",
};

const AGENT_STOPPED_NO_CODE: Msg = Msg {
    en: "Agent stopped (no code)",
    es: "Agente detenido (sin código)",
    zh: "智能体已停止（无代码）",
    de: "Agent angehalten (kein Code)",
    fr: "Agent arrêté (sans code)",
};

const NO_ACTIVITY: Msg = Msg {
    en: "No activity yet.",
    es: "Sin actividad todavía.",
    zh: "暂无活动。",
    de: "Noch keine Aktivität.",
    fr: "Aucune activité pour l’instant.",
};

const AGENT_WRITING: Msg = Msg {
    en: "Agent · writing…",
    es: "Agente · escribiendo…",
    zh: "智能体 · 正在输入…",
    de: "Agent · schreibt …",
    fr: "Agent · en train d’écrire…",
};

/// Longitud máxima del resumen de argumentos de una herramienta.
const ARGS_PREVIEW_CHARS: usize = 80;

fn preview(text: &str) -> String {
    let single_line: String = text.split_whitespace().collect::<Vec<&str>>().join(" ");
    if single_line.chars().count() > ARGS_PREVIEW_CHARS {
        format!(
            "{}…",
            single_line
                .chars()
                .take(ARGS_PREVIEW_CHARS)
                .collect::<String>()
        )
    } else {
        single_line
    }
}

/// Argumentos que mejor identifican una llamada, por orden de preferencia.
const KEY_ARGS: [&str; 8] = [
    "path",
    "command",
    "pattern",
    "query",
    "url",
    "file",
    "operation",
    "task",
];

/// Resumen legible de los argumentos (p. ej. la ruta o el comando) en vez del JSON crudo.
fn args_summary(args: &str) -> String {
    let parsed: Option<serde_json::Value> = serde_json::from_str(args).ok();
    let Some(serde_json::Value::Object(map)) = parsed else {
        return preview(args);
    };
    // Sin clave conocida no se resume: elegir un valor cualquiera podría exponer datos sensibles.
    let chosen: Option<&str> = KEY_ARGS
        .iter()
        .find_map(|key: &&str| map.get(*key).and_then(serde_json::Value::as_str));
    chosen.map(preview).unwrap_or_default()
}

/// Argumentos como pares clave/valor legibles (sin llaves ni comillas JSON).
fn arg_rows(args: &str) -> Vec<(String, String)> {
    let parsed: Option<serde_json::Value> = serde_json::from_str(args).ok();
    match parsed {
        Some(serde_json::Value::Object(map)) => map
            .into_iter()
            .map(|(key, value): (String, serde_json::Value)| {
                let text: String = match value {
                    serde_json::Value::String(text) => text,
                    other => other.to_string(),
                };
                (key, text)
            })
            .collect(),
        _ if args.trim().is_empty() => Vec::new(),
        _ => vec![(String::new(), args.to_string())],
    }
}

fn markdown_body(text: String) -> impl IntoView {
    view! { <div class="markdown" inner_html=render_markdown(&text)></div> }
}

/// Summary for startup notices that dump launch details (`args=... cwd=...`), which are
/// collapsed by default; `None` for ordinary notices.
fn launch_notice_summary(message: &str) -> Option<&'static str> {
    (message.contains("args=") || message.contains("cwd=")).then_some(LAUNCH_DETAILS.get())
}

fn notice_class(level: NoticeLevelDto) -> &'static str {
    match level {
        NoticeLevelDto::Info => "notice notice-info",
        NoticeLevelDto::Warning => "notice notice-warn",
        NoticeLevelDto::Error => "notice notice-error",
    }
}

fn render_item(item: TranscriptItem) -> AnyView {
    match item {
        TranscriptItem::User { text, images } => view! {
            <div class="msg msg-user animate-slide-in-right">
                <span class="msg-role">{ROLE_USER.get()}</span>
                {(images > 0)
                    .then(|| view! { <span class="msg-attachments">{images_label(images)}</span> })}
                {(!text.is_empty())
                    .then(|| view! { <pre class="whitespace-pre-wrap break-words">{text}</pre> })}
            </div>
        }
        .into_any(),
        TranscriptItem::Assistant {
            text,
            thinking,
            error,
        } => view! {
            <div class="msg msg-assistant animate-slide-in-right">
                <span class="msg-role">{ROLE_AGENT.get()}</span>
                {thinking
                    .map(|t: String| {
                        view! {
                            <details class="thinking">
                                <summary>{REASONING.get()}</summary>
                                <pre class="whitespace-pre-wrap break-words">{t}</pre>
                            </details>
                        }
                    })}
                {(!text.is_empty())
                    .then(|| markdown_body(text))}
                {error.map(|e: String| view! { <ErrorText message=e /> })}
            </div>
        }
        .into_any(),
        TranscriptItem::Tool {
            name,
            args,
            output,
            is_error,
            ..
        } => {
            let (state, class): (&str, &str) = match is_error {
                None => (TOOL_RUNNING.get(), "tool tool-running"),
                Some(false) => (TOOL_OK.get(), "tool tool-ok"),
                Some(true) => (TOOL_ERROR.get(), "tool tool-error"),
            };
            let summary: String = format!("🔧 {name} · {state} · {}", args_summary(&args));
            let rows: Vec<(String, String)> = arg_rows(&args);
            view! {
                <details class=class>
                    <summary>{summary}</summary>
                    <dl class="tool-args">
                        {rows
                            .into_iter()
                            .map(|(key, value): (String, String)| {
                                let multiline: bool = value.contains('\n') || value.len() > 80;
                                view! {
                                    <div class="tool-arg">
                                        {(!key.is_empty())
                                            .then(|| view! { <dt>{key}</dt> })}
                                        <dd>
                                            {if multiline {
                                                view! {
                                                    <pre class="whitespace-pre-wrap break-words">{value}</pre>
                                                }
                                                    .into_any()
                                            } else {
                                                view! { <code>{value}</code> }.into_any()
                                            }}
                                        </dd>
                                    </div>
                                }
                            })
                            .collect_view()}
                    </dl>
                    {(!output.is_empty())
                        .then(|| {
                            view! {
                                <pre class="tool-output whitespace-pre-wrap break-words">{output}</pre>
                            }
                        })}
                </details>
            }
            .into_any()
        }
        TranscriptItem::Notice { level, message } => {
            match launch_notice_summary(&message) {
                Some(summary) => view! {
                    <details class=notice_class(level)>
                        <summary class="cursor-pointer">{summary}</summary>
                        <pre class="mt-2 font-mono text-[11px] break-all whitespace-pre-wrap">{message}</pre>
                    </details>
                }
                .into_any(),
                None => view! { <div class=notice_class(level)>{message}</div> }.into_any(),
            }
        }
        TranscriptItem::Status { status, error } => view! {
            <div class="status-line">
                <TaskStatusBadge status />
                {error.map(|e: String| view! { <span class="muted">{e}</span> })}
            </div>
        }
        .into_any(),
        TranscriptItem::Dialog { request, resolved } => {
            let state: &str = if resolved {
                DIALOG_ANSWERED.get()
            } else {
                DIALOG_PENDING.get()
            };
            view! { <div class="notice notice-info">{format!("❓ {} ({state})", request.title)}</div> }
                .into_any()
        }
        TranscriptItem::Exit { code } => {
            let message: String = match code {
                Some(c) => AGENT_STOPPED_CODE.fmt(&[("code", &c.to_string())]),
                None => AGENT_STOPPED_NO_CODE.get().to_string(),
            };
            view! { <div class="status-line muted">{message}</div> }.into_any()
        }
    }
}

/// Conversación de la tarea: elementos cerrados y el mensaje en generación.
#[component]
pub fn TranscriptView(transcript: RwSignal<Transcript>) -> impl IntoView {
    let items: Memo<Vec<TranscriptItem>> =
        Memo::new(move |_| transcript.with(|t: &Transcript| t.items.clone()));
    let item_count: Memo<usize> = Memo::new(move |_| items.with(Vec::len));
    let streaming: Memo<(String, String)> = Memo::new(move |_| {
        transcript.with(|t: &Transcript| (t.streaming_thinking.clone(), t.streaming_text.clone()))
    });

    view! {
        <div class="transcript">
            <Show when=move || item_count.get() == 0>
                <p class="muted">{NO_ACTIVITY.get()}</p>
            </Show>
            <For
                each=move || 0..item_count.get()
                key=|index: &usize| *index
                children=move |index: usize| {
                    let row: Memo<Option<TranscriptItem>> = Memo::new(move |_| {
                        items.with(|list: &Vec<TranscriptItem>| list.get(index).cloned())
                    });
                    move || row.get().map(render_item)
                }
            />
            {move || {
                let (thinking, text): (String, String) = streaming.get();
                (!thinking.is_empty() || !text.is_empty())
                    .then(|| {
                        view! {
                            <div class="msg msg-assistant streaming animate-fade-in">
                                <span class="msg-role">{AGENT_WRITING.get()}</span>
                                {(!thinking.is_empty())
                                    .then(|| {
                                        view! {
                                            <pre class="muted whitespace-pre-wrap break-words">{thinking}</pre>
                                        }
                                    })}
                                {markdown_body(text)}
                            </div>
                        }
                    })
            }}
        </div>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    #[test]
    fn launch_notices_are_collapsed() {
        assert_eq!(
            launch_notice_summary("Arrancando agente args=[--x] cwd=/wt"),
            Some("Launch details")
        );
        assert_eq!(LAUNCH_DETAILS.in_locale(Locale::Es), "Detalles de arranque");
        assert_eq!(launch_notice_summary("Conflictos en a.rs"), None);
    }

    #[test]
    fn args_summary_prefers_path_over_raw_json() {
        assert_eq!(
            args_summary(r#"{"limit":230,"offset":40,"path":"src/a.rs"}"#),
            "src/a.rs"
        );
        assert_eq!(args_summary("no es json"), "no es json");
    }

    #[test]
    fn args_summary_ignores_unknown_string_keys() {
        assert_eq!(args_summary(r#"{"token":"s3cr3t","limit":3}"#), "");
    }

    #[test]
    fn arg_rows_flattens_json_into_pairs() {
        assert_eq!(
            arg_rows(r#"{"command":"ls -la","limit":3}"#),
            vec![
                ("command".to_string(), "ls -la".to_string()),
                ("limit".to_string(), "3".to_string())
            ]
        );
        assert!(arg_rows("").is_empty());
    }

    #[test]
    fn preview_collapses_whitespace_and_truncates() {
        assert_eq!(preview("{\n  \"a\": 1\n}"), "{ \"a\": 1 }");
        assert!(preview(&"x".repeat(200)).ends_with('…'));
    }
}
