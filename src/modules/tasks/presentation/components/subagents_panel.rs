use leptos::prelude::*;

use crate::core::components::busy::Loading;

use crate::core::i18n::Msg;

use crate::core::components::layout::Badge;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::subagent_task_dto::{
    SubagentStatusDto, SubagentTaskDto, SubagentThreadItemDto,
};

/// Texto mostrado cuando el agente no ha lanzado subagentes.
const EMPTY_TEXT: Msg = Msg {
    en: "This agent has not launched any subagents.",
    es: "Este agente no ha lanzado subagentes.",
    zh: "此智能体尚未启动任何子智能体。",
    de: "Dieser Agent hat noch keine Subagents gestartet.",
    fr: "Cet agent n’a lancé aucun sous-agent.",
};

const STATUS_QUEUED: Msg = Msg {
    en: "Queued",
    es: "En cola",
    zh: "排队中",
    de: "In Warteschlange",
    fr: "En file d’attente",
};

const STATUS_RUNNING: Msg = Msg {
    en: "Running",
    es: "Ejecutando",
    zh: "运行中",
    de: "Läuft",
    fr: "En cours d’exécution",
};

const STATUS_WAITING: Msg = Msg {
    en: "Waiting",
    es: "Esperando",
    zh: "等待中",
    de: "Wartet",
    fr: "En attente",
};

const STATUS_COMPLETED: Msg = Msg {
    en: "Completed",
    es: "Completado",
    zh: "已完成",
    de: "Abgeschlossen",
    fr: "Terminé",
};

const STATUS_FAILED: Msg = Msg {
    en: "Failed",
    es: "Fallido",
    zh: "失败",
    de: "Fehlgeschlagen",
    fr: "Échoué",
};

const STATUS_CANCELLED: Msg = Msg {
    en: "Cancelled",
    es: "Cancelado",
    zh: "已取消",
    de: "Abgebrochen",
    fr: "Annulé",
};

const STATUS_TIMED_OUT: Msg = Msg {
    en: "Timed out",
    es: "Tiempo agotado",
    zh: "已超时",
    de: "Zeitüberschreitung",
    fr: "Délai dépassé",
};

const STATUS_UNKNOWN: Msg = Msg {
    en: "Unknown",
    es: "Desconocido",
    zh: "未知",
    de: "Unbekannt",
    fr: "Inconnu",
};

const ACTIVITY_SUMMARY: Msg = Msg {
    en: "{turns} turns · {tools} tools",
    es: "{turns} turnos · {tools} herramientas",
    zh: "{turns} 轮 · {tools} 个工具",
    de: "{turns} Durchgänge · {tools} Werkzeuge",
    fr: "{turns} tours · {tools} outils",
};

const BADGE_RUNNING: Msg = Msg {
    en: "running",
    es: "ejecutando",
    zh: "运行中",
    de: "läuft",
    fr: "en cours",
};

const BADGE_ERROR: Msg = Msg {
    en: "error",
    es: "error",
    zh: "错误",
    de: "Fehler",
    fr: "erreur",
};

const ACTIVITY: Msg = Msg {
    en: "Activity",
    es: "Actividad",
    zh: "活动",
    de: "Aktivität",
    fr: "Activité",
};

/// Etiqueta legible del estado de un subagente.
pub fn status_label(status: SubagentStatusDto) -> &'static str {
    match status {
        SubagentStatusDto::Queued => STATUS_QUEUED.get(),
        SubagentStatusDto::Running => STATUS_RUNNING.get(),
        SubagentStatusDto::Waiting => STATUS_WAITING.get(),
        SubagentStatusDto::Completed => STATUS_COMPLETED.get(),
        SubagentStatusDto::Failed => STATUS_FAILED.get(),
        SubagentStatusDto::Cancelled => STATUS_CANCELLED.get(),
        SubagentStatusDto::TimedOut => STATUS_TIMED_OUT.get(),
        SubagentStatusDto::Unknown => STATUS_UNKNOWN.get(),
    }
}

/// Variante de color (`Badge`) del estado de un subagente.
pub fn status_variant(status: SubagentStatusDto) -> &'static str {
    match status {
        SubagentStatusDto::Queued | SubagentStatusDto::Cancelled | SubagentStatusDto::Unknown => {
            "muted"
        }
        SubagentStatusDto::Running => "info",
        SubagentStatusDto::Waiting | SubagentStatusDto::TimedOut => "warn",
        SubagentStatusDto::Completed => "ok",
        SubagentStatusDto::Failed => "error",
    }
}

/// Nombre mostrado de un subagente: agente y, si lo hay, su etiqueta.
pub fn display_name(task: &SubagentTaskDto) -> String {
    match task.label.as_deref().filter(|l: &&str| !l.is_empty()) {
        Some(label) => format!("{} · {label}", task.agent),
        None => task.agent.clone(),
    }
}

/// Mantiene la selección solo si el subagente sigue en la lista (función pura).
pub fn resolve_selection(
    selected: Option<String>,
    subagents: &[SubagentTaskDto],
) -> Option<String> {
    selected.filter(|id: &String| subagents.iter().any(|s: &SubagentTaskDto| &s.id == id))
}

/// Resumen de actividad ("3 turnos · 5 herramientas").
pub fn activity_label(turns: u32, tool_calls: u32) -> String {
    ACTIVITY_SUMMARY.fmt(&[
        ("turns", &turns.to_string()),
        ("tools", &tool_calls.to_string()),
    ])
}

/// Bloque monoespaciado con scroll para textos largos.
#[component]
fn CodeBlock(#[prop(into)] text: String) -> impl IntoView {
    view! {
        <pre class="max-h-48 overflow-auto rounded-md bg-surface-2/60 p-2 font-mono text-[11px] whitespace-pre-wrap break-all text-muted">
            {text}
        </pre>
    }
}

#[component]
fn ThreadItem(item: SubagentThreadItemDto) -> impl IntoView {
    match item {
        SubagentThreadItemDto::Text { text } => {
            view! { <p class="text-sm whitespace-pre-wrap text-fg">{text}</p> }.into_any()
        }
        SubagentThreadItemDto::Thinking { text } => {
            view! { <p class="text-xs whitespace-pre-wrap text-muted italic">{text}</p> }.into_any()
        }
        SubagentThreadItemDto::Note { text } => {
            view! { <p class="notice notice-info">{text}</p> }.into_any()
        }
        SubagentThreadItemDto::Tool {
            name,
            args,
            running,
            is_error,
            output,
        } => view! {
            <div class="stack gap-1 rounded-md border border-border/60 p-2">
                <div class="row gap-2">
                    <code class="font-mono text-xs font-semibold">{name}</code>
                    {running.then(|| view! { <Badge text=BADGE_RUNNING.get() variant="info" /> })}
                    {is_error.then(|| view! { <Badge text=BADGE_ERROR.get() variant="error" /> })}
                </div>
                {args.map(|a: String| view! { <CodeBlock text=a /> })}
                {output.map(|o: String| view! { <CodeBlock text=o /> })}
            </div>
        }
        .into_any(),
    }
}

#[component]
fn SubagentDetail(task: SubagentTaskDto) -> impl IntoView {
    view! {
        <div class="stack gap-2">
            {task
                .prompt
                .map(|p: String| {
                    view! {
                        <div class="stack gap-1">
                            <span class="eyebrow">"Prompt"</span>
                            <CodeBlock text=p />
                        </div>
                    }
                })}
            {task.error.map(|e: String| view! { <p class="notice notice-warn">{e}</p> })}
            <div class="stack gap-1">
                <span class="eyebrow">{ACTIVITY.get()}</span>
                {task
                    .thread
                    .into_iter()
                    .map(|item: SubagentThreadItemDto| view! { <ThreadItem item /> })
                    .collect_view()}
            </div>
        </div>
    }
}

/// Lista de subagentes con el detalle del seleccionado. Solo presentacional: los datos
/// y la selección llegan como props.
#[component]
pub fn SubagentsPanel(
    #[prop(into)] subagents: Signal<Vec<SubagentTaskDto>>,
    #[prop(into)] selected: Signal<Option<String>>,
    on_select: Callback<String>,
) -> impl IntoView {
    let detail: Memo<Option<SubagentTaskDto>> = Memo::new(move |_| {
        let id: Option<String> = selected.get();
        subagents.with(|list: &Vec<SubagentTaskDto>| {
            list.iter()
                .find(|s: &&SubagentTaskDto| Some(&s.id) == id.as_ref())
                .cloned()
        })
    });
    view! {
        <Show
            when=move || !subagents.with(Vec::is_empty)
            fallback=|| view! { <Loading text=EMPTY_TEXT.get() /> }
        >
            <ul class="stack gap-1">
                {move || {
                    subagents
                        .get()
                        .into_iter()
                        .map(|s: SubagentTaskDto| {
                            let id: String = s.id.clone();
                            let is_selected: bool = selected.get().as_deref() == Some(s.id.as_str());
                            let row_class: &str = if is_selected {
                                "stack w-full gap-1 rounded-lg border border-border/60 bg-surface-2 p-2 text-left transition-colors"
                            } else {
                                "stack w-full gap-1 rounded-lg border border-border/60 p-2 text-left transition-colors hover:bg-surface-2"
                            };
                            view! {
                                <li>
                                    <button
                                        class=row_class
                                        aria-pressed=is_selected.to_string()
                                        on:click=move |_| on_select.run(id.clone())
                                    >
                                        <div class="row justify-between gap-2">
                                            <span class="truncate text-sm font-semibold">{display_name(&s)}</span>
                                            <Badge text=status_label(s.status) variant=status_variant(s.status) />
                                        </div>
                                        <span class="text-[11px] text-muted">
                                            {activity_label(s.turns, s.tool_calls)}
                                        </span>
                                        {s
                                            .last_step
                                            .clone()
                                            .map(|step: String| {
                                                view! { <span class="truncate text-[11px] text-muted">{step}</span> }
                                            })}
                                    </button>
                                </li>
                            }
                        })
                        .collect_view()
                }}
            </ul>
            {move || detail.get().map(|task: SubagentTaskDto| view! { <SubagentDetail task /> })}
        </Show>
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::core::i18n::Locale;

    fn task(id: &str, label: Option<&str>) -> SubagentTaskDto {
        SubagentTaskDto {
            id: id.to_string(),
            agent: "explore".to_string(),
            label: label.map(str::to_string),
            prompt: None,
            status: SubagentStatusDto::Running,
            created_at: None,
            started_at: None,
            ended_at: None,
            last_step: None,
            last_activity_at: None,
            turns: 0,
            tool_calls: 0,
            error: None,
            thread: vec![],
        }
    }

    #[test]
    fn every_status_has_label_and_known_variant() {
        for status in [
            SubagentStatusDto::Queued,
            SubagentStatusDto::Running,
            SubagentStatusDto::Waiting,
            SubagentStatusDto::Completed,
            SubagentStatusDto::Failed,
            SubagentStatusDto::Cancelled,
            SubagentStatusDto::TimedOut,
            SubagentStatusDto::Unknown,
        ] {
            assert!(!status_label(status).is_empty());
            assert!(["ok", "warn", "error", "info", "muted"].contains(&status_variant(status)));
        }
        assert_eq!(status_variant(SubagentStatusDto::Failed), "error");
        assert_eq!(status_variant(SubagentStatusDto::Completed), "ok");
        assert_eq!(status_variant(SubagentStatusDto::Running), "info");
    }

    #[test]
    fn display_name_appends_non_empty_label() {
        assert_eq!(display_name(&task("a", Some("Mapa"))), "explore · Mapa");
        assert_eq!(display_name(&task("a", Some(""))), "explore");
        assert_eq!(display_name(&task("a", None)), "explore");
    }

    #[test]
    fn selection_survives_only_while_the_subagent_exists() {
        let list: Vec<SubagentTaskDto> = vec![task("a", None), task("b", None)];
        assert_eq!(
            resolve_selection(Some("b".to_string()), &list),
            Some("b".to_string())
        );
        assert_eq!(resolve_selection(Some("z".to_string()), &list), None);
        assert_eq!(resolve_selection(None, &list), None);
        assert_eq!(resolve_selection(Some("a".to_string()), &[]), None);
    }

    #[test]
    fn activity_label_counts_turns_and_tools() {
        assert_eq!(activity_label(3, 5), "3 turns · 5 tools");
        assert_eq!(
            ACTIVITY_SUMMARY
                .in_locale(Locale::Es)
                .replace("{turns}", "3")
                .replace("{tools}", "5"),
            "3 turnos · 5 herramientas"
        );
    }
}
