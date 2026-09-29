use leptos::prelude::*;

use crate::core::components::layout::ErrorText;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::NoticeLevelDto;
use crate::modules::tasks::presentation::components::task_status_badge::TaskStatusBadge;
use crate::modules::tasks::presentation::state::transcript::{Transcript, TranscriptItem};

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

fn notice_class(level: NoticeLevelDto) -> &'static str {
    match level {
        NoticeLevelDto::Info => "notice notice-info",
        NoticeLevelDto::Warning => "notice notice-warn",
        NoticeLevelDto::Error => "notice notice-error",
    }
}

fn render_item(item: TranscriptItem) -> AnyView {
    match item {
        TranscriptItem::User { text } => view! {
            <div class="msg msg-user">
                <span class="msg-role">"Tú"</span>
                <pre class="whitespace-pre-wrap break-words">{text}</pre>
            </div>
        }
        .into_any(),
        TranscriptItem::Assistant {
            text,
            thinking,
            error,
        } => view! {
            <div class="msg msg-assistant">
                <span class="msg-role">"Agente"</span>
                {thinking
                    .map(|t: String| {
                        view! {
                            <details class="thinking">
                                <summary>"Razonamiento"</summary>
                                <pre class="whitespace-pre-wrap break-words">{t}</pre>
                            </details>
                        }
                    })}
                {(!text.is_empty())
                    .then(|| view! { <pre class="whitespace-pre-wrap break-words">{text}</pre> })}
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
                None => ("en curso", "tool tool-running"),
                Some(false) => ("ok", "tool tool-ok"),
                Some(true) => ("error", "tool tool-error"),
            };
            let summary: String = format!("🔧 {name} · {state} · {}", preview(&args));
            view! {
                <details class=class>
                    <summary>{summary}</summary>
                    <pre class="tool-args whitespace-pre-wrap break-words">{args}</pre>
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
            view! { <div class=notice_class(level)>{message}</div> }.into_any()
        }
        TranscriptItem::Status { status, error } => view! {
            <div class="status-line">
                <TaskStatusBadge status />
                {error.map(|e: String| view! { <span class="muted">{e}</span> })}
            </div>
        }
        .into_any(),
        TranscriptItem::Dialog { request, resolved } => {
            let state: &str = if resolved { "respondido" } else { "pendiente" };
            view! { <div class="notice notice-info">{format!("❓ {} ({state})", request.title)}</div> }
                .into_any()
        }
        TranscriptItem::Exit { code } => {
            let detail: String = code
                .map(|c: i32| format!("código {c}"))
                .unwrap_or_else(|| "sin código".to_string());
            view! { <div class="status-line muted">{format!("Agente detenido ({detail})")}</div> }
                .into_any()
        }
    }
}

/// Conversación de la tarea: elementos cerrados y el mensaje en generación.
#[component]
pub fn TranscriptView(transcript: RwSignal<Transcript>) -> impl IntoView {
    // Los deltas no cambian `items`, así que la lista solo se repinta al cerrarse mensajes.
    let items: Memo<Vec<TranscriptItem>> =
        Memo::new(move |_| transcript.with(|t: &Transcript| t.items.clone()));
    let item_count: Memo<usize> = Memo::new(move |_| items.with(Vec::len));
    let streaming: Memo<(String, String)> = Memo::new(move |_| {
        transcript.with(|t: &Transcript| (t.streaming_thinking.clone(), t.streaming_text.clone()))
    });

    view! {
        <div class="transcript">
            <Show when=move || item_count.get() == 0>
                <p class="muted">"Sin actividad todavía."</p>
            </Show>
            // Clave por posición: la transcripción solo añade al final, así que las filas
            // existentes no se recrean (conservan `<details>` abiertos). Cada fila solo se
            // repinta si su propio elemento cambia (p.ej. una herramienta recibe su salida).
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
                            <div class="msg msg-assistant streaming">
                                <span class="msg-role">"Agente · escribiendo…"</span>
                                {(!thinking.is_empty())
                                    .then(|| {
                                        view! {
                                            <pre class="muted whitespace-pre-wrap break-words">{thinking}</pre>
                                        }
                                    })}
                                <pre class="whitespace-pre-wrap break-words">{text}</pre>
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

    #[test]
    fn preview_collapses_whitespace_and_truncates() {
        assert_eq!(preview("{\n  \"a\": 1\n}"), "{ \"a\": 1 }");
        assert!(preview(&"x".repeat(200)).ends_with('…'));
    }
}
