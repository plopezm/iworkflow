//! Traducción entre el protocolo JSONL de `pi --mode rpc` y el dominio.

use serde_json::{Map, Value, json};

use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::{
    AgentEvent, DialogAnswer, DialogKind, DialogRequest, NoticeLevel,
};
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::subagent_task::{
    SubagentStatus, SubagentTask, SubagentThreadItem,
};
use crate::modules::tasks::domain::models::usage_limit::looks_like_usage_limit;

/// Límite de caracteres de salidas y argumentos de herramientas que se conservan.
const MAX_TOOL_TEXT_CHARS: usize = 20_000;

/// Añade el campo `images` de pi solo si hay imágenes que enviar.
fn with_images(mut record: Value, images: &[ImageAttachment]) -> Value {
    if !images.is_empty() {
        record["images"] = images
            .iter()
            .map(|image: &ImageAttachment| {
                json!({ "type": "image", "data": image.data, "mimeType": image.mime_type })
            })
            .collect();
    }
    record
}

/// Serializa una orden como un registro JSONL (sin salto de línea final).
pub fn encode_command(command: &AgentCommand, request_id: &str) -> Option<String> {
    let record: Value = match command {
        AgentCommand::Prompt { message, images } => with_images(
            json!({
                "id": request_id,
                "type": "prompt",
                "message": message,
                // Evita el error de pi si el agente siguiera ocupado.
                "streamingBehavior": "followUp",
            }),
            images,
        ),
        AgentCommand::Steer { message, images } => with_images(
            json!({ "id": request_id, "type": "steer", "message": message }),
            images,
        ),
        AgentCommand::FollowUp { message, images } => with_images(
            json!({ "id": request_id, "type": "follow_up", "message": message }),
            images,
        ),
        AgentCommand::Abort => json!({ "id": request_id, "type": "abort" }),
        // Pi no expone modos: el caso de uso lo filtra por capacidad y aquí no hay nada que enviar.
        AgentCommand::SetMode(_) => return None,
        AgentCommand::AnswerDialog { dialog_id, answer } => {
            let mut response: Map<String, Value> = Map::new();
            response.insert("type".into(), json!("extension_ui_response"));
            response.insert("id".into(), json!(dialog_id));
            match answer {
                DialogAnswer::Value { value } => response.insert("value".into(), json!(value)),
                DialogAnswer::Confirmed { confirmed } => {
                    response.insert("confirmed".into(), json!(confirmed))
                }
                DialogAnswer::Cancelled => response.insert("cancelled".into(), json!(true)),
            };
            Value::Object(response)
        }
    };
    Some(record.to_string())
}

/// Traduce un registro de stdout a eventos de dominio (vacío si no es relevante).
pub fn decode(line: &str) -> Vec<AgentEvent> {
    let record: Value = match serde_json::from_str(line) {
        Ok(value) => value,
        Err(_) => return Vec::new(),
    };
    let kind: &str = str_field(&record, "type").unwrap_or_default();
    let event: Option<AgentEvent> = match kind {
        "agent_start" => Some(AgentEvent::RunStarted),
        "agent_settled" => Some(AgentEvent::RunSettled),
        "message_end" => decode_message(record.get("message")),
        "message_update" => decode_update(&record),
        "tool_execution_start" => Some(AgentEvent::ToolStarted {
            call_id: string_of(&record, "toolCallId"),
            tool_name: string_of(&record, "toolName"),
            args: truncate(&record.get("args").map(Value::to_string).unwrap_or_default()),
        }),
        "tool_execution_update" => Some(AgentEvent::ToolUpdated {
            call_id: string_of(&record, "toolCallId"),
            output: truncate(&content_text(
                record
                    .get("partialResult")
                    .and_then(|r: &Value| r.get("content")),
            )),
        }),
        "tool_execution_end" => Some(AgentEvent::ToolFinished {
            call_id: string_of(&record, "toolCallId"),
            tool_name: string_of(&record, "toolName"),
            output: truncate(&content_text(
                record.get("result").and_then(|r: &Value| r.get("content")),
            )),
            is_error: record
                .get("isError")
                .and_then(Value::as_bool)
                .unwrap_or(false),
        }),
        "queue_update" => Some(AgentEvent::QueueChanged {
            steering: string_list(record.get("steering")),
            follow_up: string_list(record.get("followUp")),
        }),
        "auto_retry_start" => Some(notice(
            NoticeLevel::Warning,
            format!(
                "Reintento {}/{}: {}",
                record.get("attempt").and_then(Value::as_u64).unwrap_or(0),
                record
                    .get("maxAttempts")
                    .and_then(Value::as_u64)
                    .unwrap_or(0),
                string_of(&record, "errorMessage")
            ),
        )),
        "auto_retry_end" if record.get("success").and_then(Value::as_bool) == Some(false) => {
            Some(notice(
                NoticeLevel::Error,
                format!("Reintentos agotados: {}", string_of(&record, "finalError")),
            ))
        }
        "compaction_start" => Some(notice(
            NoticeLevel::Info,
            format!("Compactando contexto ({})", string_of(&record, "reason")),
        )),
        "compaction_end" => str_field(&record, "errorMessage").map(|error: &str| {
            notice(
                NoticeLevel::Warning,
                format!("Compactación fallida: {error}"),
            )
        }),
        "extension_error" => Some(notice(
            NoticeLevel::Error,
            format!(
                "Error de extensión ({}): {}",
                string_of(&record, "extensionPath"),
                string_of(&record, "error")
            ),
        )),
        "response" if record.get("success").and_then(Value::as_bool) == Some(false) => {
            Some(notice(
                NoticeLevel::Error,
                format!(
                    "{} rechazado: {}",
                    string_of(&record, "command"),
                    string_of(&record, "error")
                ),
            ))
        }
        "extension_ui_request" => decode_ui_request(&record),
        _ => None,
    };
    let mut events: Vec<AgentEvent> = event.into_iter().collect();
    if let Some(limit) = usage_limit_of(&events) {
        events.push(limit);
    }
    events.extend(decode_subagents(&record));
    events
}

/// Un mensaje del asistente que falló por límite de uso (pi solo informa del error en
/// texto, sin instante de renovación).
fn usage_limit_of(events: &[AgentEvent]) -> Option<AgentEvent> {
    events.iter().find_map(|event: &AgentEvent| match event {
        AgentEvent::AssistantMessage {
            error: Some(message),
            ..
        } if looks_like_usage_limit(message) => Some(AgentEvent::UsageLimitReached {
            resets_at: None,
            message: message.clone(),
        }),
        _ => None,
    })
}

/// Clave del widget con el que la extensión de subagentes publica su actividad.
const SUBAGENTS_WIDGET_KEY: &str = "gentle-agents";

/// Línea JSON del widget `gentle-agents`: `None` si el registro no es ese widget,
/// `Some(None)` si se ha limpiado (sin `widgetLines`).
fn subagents_widget_line(record: &Value) -> Option<Option<&str>> {
    if str_field(record, "type") != Some("extension_ui_request")
        || str_field(record, "method") != Some("setWidget")
        || str_field(record, "widgetKey") != Some(SUBAGENTS_WIDGET_KEY)
    {
        return None;
    }
    Some(
        record
            .get("widgetLines")
            .and_then(Value::as_array)
            .and_then(|lines: &Vec<Value>| lines.first())
            .and_then(Value::as_str),
    )
}

/// Resumen de subagentes vivos a partir del widget `gentle-agents` (primera línea JSON
/// con `summary: {running, queued, waiting}`). Limpiar el widget equivale a ninguno.
fn decode_subagents(record: &Value) -> Option<AgentEvent> {
    let Some(line) = subagents_widget_line(record)? else {
        return Some(AgentEvent::SubagentsChanged {
            active: 0,
            waiting: 0,
        });
    };
    let activity: Value = serde_json::from_str(line).ok()?;
    let summary: &Value = activity.get("summary")?;
    let count = |key: &str| -> u32 {
        summary
            .get(key)
            .and_then(Value::as_u64)
            .map(|n: u64| u32::try_from(n).unwrap_or(u32::MAX))
            .unwrap_or(0)
    };
    Some(AgentEvent::SubagentsChanged {
        active: count("running").saturating_add(count("queued")),
        waiting: count("waiting"),
    })
}

/// Prefijo del campo `schema` de las cargas que sabemos leer (`gentle-agents.activity/v1`…).
const SUBAGENTS_SCHEMA_PREFIX: &str = "gentle-agents.activity/";

/// Instantánea completa de los subagentes a partir del widget `gentle-agents`. `None` si la
/// línea no es ese widget o su carga está malformada (el llamador conserva lo que tenía);
/// limpiar el widget equivale a ninguno.
pub fn decode_subagent_tasks(line: &str) -> Option<Vec<SubagentTask>> {
    let record: Value = serde_json::from_str(line).ok()?;
    let Some(payload_line) = subagents_widget_line(&record)? else {
        return Some(Vec::new());
    };
    let payload: Value = serde_json::from_str(payload_line).ok()?;
    if str_field(&payload, "schema")
        .is_some_and(|schema: &str| !schema.starts_with(SUBAGENTS_SCHEMA_PREFIX))
    {
        return None;
    }
    let tasks: &Vec<Value> = payload.get("tasks")?.as_array()?;
    Some(tasks.iter().filter_map(decode_subagent_task).collect())
}

fn decode_subagent_task(task: &Value) -> Option<SubagentTask> {
    let summary: &Value = task.get("summary")?;
    let text = |key: &str| -> Option<String> { str_field(summary, key).map(str::to_string) };
    let millis = |key: &str| -> Option<i64> { summary.get(key).and_then(Value::as_i64) };
    let counter = |key: &str| -> u32 {
        summary
            .get(key)
            .and_then(Value::as_u64)
            .map(|n: u64| u32::try_from(n).unwrap_or(u32::MAX))
            .unwrap_or(0)
    };
    let thread: Vec<SubagentThreadItem> = task
        .get("thread")
        .and_then(|thread: &Value| thread.get("items"))
        .and_then(Value::as_array)
        .map(|items: &Vec<Value>| items.iter().filter_map(decode_thread_item).collect())
        .unwrap_or_default();
    Some(SubagentTask {
        id: text("id")?,
        agent: text("agent").unwrap_or_default(),
        label: text("label"),
        prompt: text("prompt"),
        status: decode_subagent_status(str_field(summary, "status")),
        created_at: millis("createdAt"),
        started_at: millis("startedAt"),
        ended_at: millis("endedAt"),
        last_step: text("lastStep"),
        last_activity_at: millis("lastActivityAt"),
        turns: counter("turns"),
        tool_calls: counter("toolCalls"),
        error: text("error"),
        thread,
    })
}

fn decode_subagent_status(raw: Option<&str>) -> SubagentStatus {
    match raw {
        Some("queued") => SubagentStatus::Queued,
        Some("running") => SubagentStatus::Running,
        Some("waiting") => SubagentStatus::Waiting,
        Some("completed") => SubagentStatus::Completed,
        Some("failed") => SubagentStatus::Failed,
        Some("cancelled") => SubagentStatus::Cancelled,
        Some("timed_out") => SubagentStatus::TimedOut,
        _ => SubagentStatus::Unknown,
    }
}

/// Un tipo de elemento desconocido con `text` se degrada a nota; sin texto se descarta.
fn decode_thread_item(item: &Value) -> Option<SubagentThreadItem> {
    let text: Option<String> = str_field(item, "text").map(str::to_string);
    match str_field(item, "kind")? {
        "text" => text.map(SubagentThreadItem::Text),
        "thinking" => text.map(SubagentThreadItem::Thinking),
        "tool" => Some(SubagentThreadItem::Tool {
            name: string_of(item, "name"),
            args: str_field(item, "args").map(str::to_string),
            running: item
                .get("running")
                .and_then(Value::as_bool)
                .unwrap_or(false),
            is_error: item
                .get("isError")
                .and_then(Value::as_bool)
                .unwrap_or(false),
            output: str_field(item, "output").map(str::to_string),
        }),
        _ => text.map(SubagentThreadItem::Note),
    }
}

fn decode_message(message: Option<&Value>) -> Option<AgentEvent> {
    let message: &Value = message?;
    match str_field(message, "role")? {
        "user" => {
            let text: String = content_text(message.get("content"));
            let images: u32 = count_blocks(message.get("content"), "image");
            (!text.is_empty() || images > 0).then_some(AgentEvent::UserMessage { text, images })
        }
        "assistant" => {
            let text: String = blocks_text(message.get("content"), "text", "text");
            let thinking: String = blocks_text(message.get("content"), "thinking", "thinking");
            let error: Option<String> = match str_field(message, "stopReason") {
                Some("error") | Some("aborted") => Some(
                    str_field(message, "errorMessage")
                        .unwrap_or("ejecución interrumpida")
                        .to_string(),
                ),
                _ => None,
            };
            if text.is_empty() && thinking.is_empty() && error.is_none() {
                return None;
            }
            Some(AgentEvent::AssistantMessage {
                text,
                thinking: (!thinking.is_empty()).then_some(thinking),
                error,
            })
        }
        _ => None,
    }
}

fn decode_update(record: &Value) -> Option<AgentEvent> {
    let update: &Value = record.get("assistantMessageEvent")?;
    let delta: String = string_of(update, "delta");
    match str_field(update, "type")? {
        "text_delta" => Some(AgentEvent::TextDelta { delta }),
        "thinking_delta" => Some(AgentEvent::ThinkingDelta { delta }),
        _ => None,
    }
}

fn decode_ui_request(record: &Value) -> Option<AgentEvent> {
    let id: String = string_of(record, "id");
    let title: String = strip_ansi(&string_of(record, "title"));
    let timeout_ms: Option<u64> = record.get("timeout").and_then(Value::as_u64);
    let dialog = |kind: DialogKind| {
        AgentEvent::DialogRequested(DialogRequest {
            id: id.clone(),
            title: title.clone(),
            kind,
            timeout_ms,
        })
    };
    match str_field(record, "method")? {
        "select" => Some(dialog(DialogKind::Select {
            options: string_list(record.get("options")),
        })),
        "confirm" => Some(dialog(DialogKind::Confirm {
            message: str_field(record, "message").map(str::to_string),
        })),
        "input" => Some(dialog(DialogKind::Input {
            placeholder: str_field(record, "placeholder").map(str::to_string),
        })),
        "editor" => Some(dialog(DialogKind::Editor {
            prefill: str_field(record, "prefill").map(str::to_string),
        })),
        "notify" => Some(notice(
            match str_field(record, "notifyType") {
                Some("warning") => NoticeLevel::Warning,
                Some("error") => NoticeLevel::Error,
                _ => NoticeLevel::Info,
            },
            strip_ansi(&string_of(record, "message")),
        )),
        "setStatus" => Some(AgentEvent::StatusChanged {
            key: string_of(record, "statusKey"),
            text: str_field(record, "statusText").map(strip_ansi),
        }),
        "setWidget" => Some(AgentEvent::WidgetChanged {
            key: string_of(record, "widgetKey"),
            lines: record
                .get("widgetLines")
                .and_then(Value::as_array)
                .map(|lines: &Vec<Value>| {
                    lines
                        .iter()
                        .filter_map(Value::as_str)
                        .map(strip_ansi)
                        .collect()
                }),
        }),
        _ => None,
    }
}

/// Clave de estado con la que pi publica los jobs en background (`⧗ 2 jobs`).
const JOBS_STATUS_KEY: &str = "gentle-jobs";

/// Trabajo pendiente del agente que no requiere al usuario: subagentes y jobs en background.
/// Mientras haya alguno, la tarea sigue en marcha aunque el agente esté esperando.
#[derive(Debug, Default)]
pub struct ActivityTracker {
    subagents: u32,
    waiting: u32,
    jobs: u32,
}

impl ActivityTracker {
    /// Recuerda los contadores de los eventos y, tras cada cambio, emite un único
    /// `SubagentsChanged` que suma los jobs en background a los subagentes activos.
    pub fn fold(&mut self, events: Vec<AgentEvent>) -> Vec<AgentEvent> {
        let mut out: Vec<AgentEvent> = Vec::with_capacity(events.len());
        for event in events {
            match event {
                AgentEvent::SubagentsChanged { active, waiting } => {
                    self.subagents = active;
                    self.waiting = waiting;
                    out.push(self.combined());
                }
                AgentEvent::StatusChanged { key, text } if key == JOBS_STATUS_KEY => {
                    self.jobs = text.as_deref().map(jobs_in_status).unwrap_or(0);
                    out.push(AgentEvent::StatusChanged { key, text });
                    out.push(self.combined());
                }
                other => out.push(other),
            }
        }
        out
    }

    fn combined(&self) -> AgentEvent {
        AgentEvent::SubagentsChanged {
            active: self.subagents.saturating_add(self.jobs),
            waiting: self.waiting,
        }
    }
}

/// Primer entero del texto de estado de jobs (`⧗ 2 jobs` → 2); 0 si no hay ninguno.
fn jobs_in_status(text: &str) -> u32 {
    text.split_whitespace()
        .find_map(|word: &str| word.parse::<u32>().ok())
        .unwrap_or(0)
}

fn notice(level: NoticeLevel, message: String) -> AgentEvent {
    AgentEvent::Notice { level, message }
}

fn str_field<'a>(value: &'a Value, key: &str) -> Option<&'a str> {
    value.get(key).and_then(Value::as_str)
}

fn string_of(value: &Value, key: &str) -> String {
    str_field(value, key).unwrap_or_default().to_string()
}

/// Lista de textos: admite cadenas u objetos con campo `text`/`message`.
fn string_list(value: Option<&Value>) -> Vec<String> {
    value
        .and_then(Value::as_array)
        .map(|items: &Vec<Value>| {
            items
                .iter()
                .filter_map(|item: &Value| {
                    item.as_str()
                        .or_else(|| str_field(item, "text"))
                        .or_else(|| str_field(item, "message"))
                        .map(str::to_string)
                })
                .collect()
        })
        .unwrap_or_default()
}

/// Texto de un `content` que puede ser cadena o lista de bloques `{type: "text"}`.
fn content_text(content: Option<&Value>) -> String {
    match content {
        Some(Value::String(text)) => text.clone(),
        other => blocks_text(other, "text", "text"),
    }
}

fn count_blocks(content: Option<&Value>, block_type: &str) -> u32 {
    let count: usize = content
        .and_then(Value::as_array)
        .map(|blocks: &Vec<Value>| {
            blocks
                .iter()
                .filter(|b: &&Value| str_field(b, "type") == Some(block_type))
                .count()
        })
        .unwrap_or(0);
    u32::try_from(count).unwrap_or(u32::MAX)
}

fn blocks_text(content: Option<&Value>, block_type: &str, field: &str) -> String {
    content
        .and_then(Value::as_array)
        .map(|blocks: &Vec<Value>| {
            blocks
                .iter()
                .filter(|b: &&Value| str_field(b, "type") == Some(block_type))
                .filter_map(|b: &Value| str_field(b, field))
                .collect::<Vec<&str>>()
                .join("\n")
        })
        .unwrap_or_default()
}

fn truncate(text: &str) -> String {
    if text.chars().count() <= MAX_TOOL_TEXT_CHARS {
        return text.to_string();
    }
    let kept: String = text.chars().take(MAX_TOOL_TEXT_CHARS).collect();
    format!("{kept}\n… (truncado)")
}

/// Elimina secuencias de escape ANSI (colores de terminal).
pub fn strip_ansi(text: &str) -> String {
    let mut output: String = String::with_capacity(text.len());
    let mut chars: std::iter::Peekable<std::str::Chars<'_>> = text.chars().peekable();
    while let Some(c) = chars.next() {
        if c == '\u{1b}' {
            if chars.peek() == Some(&'[') {
                chars.next();
                for next in chars.by_ref() {
                    if next.is_ascii_alphabetic() {
                        break;
                    }
                }
            }
            continue;
        }
        output.push(c);
    }
    output
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::agent_mode::AgentMode;

    /// Como `encode_command` para las órdenes que siempre se envían.
    fn encode(command: &AgentCommand, request_id: &str) -> String {
        encode_command(command, request_id).expect("orden enviable")
    }

    #[test]
    fn pi_has_nothing_to_send_for_a_mode_change() {
        assert_eq!(
            encode_command(&AgentCommand::SetMode(AgentMode::Plan), "r"),
            None
        );
    }

    fn one(line: &str) -> AgentEvent {
        let mut events: Vec<AgentEvent> = decode(line);
        assert_eq!(events.len(), 1, "se esperaba un evento para {line}");
        events.remove(0)
    }

    #[test]
    fn encodes_commands() {
        assert_eq!(
            encode(
                &AgentCommand::Steer {
                    message: "para".into(),
                    images: Vec::new(),
                },
                "r1"
            ),
            r#"{"id":"r1","message":"para","type":"steer"}"#
        );
        assert_eq!(
            encode(&AgentCommand::Abort, "r2"),
            r#"{"id":"r2","type":"abort"}"#
        );
        let prompt: Value = serde_json::from_str(&encode(
            &AgentCommand::Prompt {
                message: "hola".into(),
                images: Vec::new(),
            },
            "r3",
        ))
        .expect("json");
        assert_eq!(prompt["type"], "prompt");
        assert_eq!(prompt["streamingBehavior"], "followUp");
        assert!(prompt.get("images").is_none());
    }

    #[test]
    fn encodes_images_for_prompt_steer_and_follow_up() {
        let images: Vec<ImageAttachment> = vec![ImageAttachment {
            name: "a.png".into(),
            mime_type: "image/png".into(),
            data: "aGk=".into(),
        }];
        let commands: Vec<(AgentCommand, &str)> = vec![
            (
                AgentCommand::Prompt {
                    message: "m".into(),
                    images: images.clone(),
                },
                "prompt",
            ),
            (
                AgentCommand::Steer {
                    message: "m".into(),
                    images: images.clone(),
                },
                "steer",
            ),
            (
                AgentCommand::FollowUp {
                    message: "m".into(),
                    images: images.clone(),
                },
                "follow_up",
            ),
        ];
        for (command, kind) in commands {
            let record: Value = serde_json::from_str(&encode(&command, "r")).expect("json");
            assert_eq!(record["type"], kind);
            assert_eq!(
                record["images"],
                json!([{ "type": "image", "data": "aGk=", "mimeType": "image/png" }])
            );
        }
    }

    #[test]
    fn encodes_dialog_answers_with_dialog_id() {
        let confirmed: Value = serde_json::from_str(&encode(
            &AgentCommand::AnswerDialog {
                dialog_id: "d1".into(),
                answer: DialogAnswer::Confirmed { confirmed: false },
            },
            "ignored",
        ))
        .expect("json");
        assert_eq!(
            confirmed,
            json!({"type":"extension_ui_response","id":"d1","confirmed":false})
        );
        let cancelled: Value = serde_json::from_str(&encode(
            &AgentCommand::AnswerDialog {
                dialog_id: "d2".into(),
                answer: DialogAnswer::Cancelled,
            },
            "ignored",
        ))
        .expect("json");
        assert_eq!(cancelled["cancelled"], true);
    }

    #[test]
    fn decodes_run_lifecycle() {
        assert_eq!(one(r#"{"type":"agent_start"}"#), AgentEvent::RunStarted);
        assert_eq!(one(r#"{"type":"agent_settled"}"#), AgentEvent::RunSettled);
        assert!(decode(r#"{"type":"turn_start"}"#).is_empty());
        assert!(decode("no es json").is_empty());
    }

    #[test]
    fn decodes_assistant_message_blocks() {
        let event: AgentEvent = one(
            r#"{"type":"message_end","message":{"role":"assistant","stopReason":"stop","content":[{"type":"thinking","thinking":"pienso"},{"type":"text","text":"Hecho"},{"type":"toolCall","id":"c1"}]}}"#,
        );
        assert_eq!(
            event,
            AgentEvent::AssistantMessage {
                text: "Hecho".into(),
                thinking: Some("pienso".into()),
                error: None
            }
        );
    }

    #[test]
    fn assistant_error_about_a_usage_limit_also_reports_the_limit() {
        let events: Vec<AgentEvent> = decode(
            r#"{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"429 usage limit reached"}}"#,
        );
        assert!(matches!(events[0], AgentEvent::AssistantMessage { .. }));
        assert_eq!(
            events[1],
            AgentEvent::UsageLimitReached {
                resets_at: None,
                message: "429 usage limit reached".to_string()
            }
        );
        let unrelated: Vec<AgentEvent> = decode(
            r#"{"type":"message_end","message":{"role":"assistant","content":[],"stopReason":"error","errorMessage":"boom"}}"#,
        );
        assert_eq!(unrelated.len(), 1);
    }

    #[test]
    fn skips_tool_only_assistant_messages_and_keeps_errors() {
        assert!(decode(r#"{"type":"message_end","message":{"role":"assistant","content":[{"type":"toolCall"}]}}"#).is_empty());
        let aborted: AgentEvent = one(
            r#"{"type":"message_end","message":{"role":"assistant","stopReason":"aborted","content":[]}}"#,
        );
        assert!(matches!(
            aborted,
            AgentEvent::AssistantMessage { error: Some(_), .. }
        ));
    }

    #[test]
    fn decodes_user_message_with_string_content() {
        assert_eq!(
            one(r#"{"type":"message_end","message":{"role":"user","content":"Arregla"}}"#),
            AgentEvent::UserMessage {
                text: "Arregla".into(),
                images: 0
            }
        );
    }

    #[test]
    fn decodes_user_message_counting_images() {
        assert_eq!(
            one(
                r#"{"type":"message_end","message":{"role":"user","content":[{"type":"text","text":"mira"},{"type":"image","data":"x","mimeType":"image/png"},{"type":"image","data":"y","mimeType":"image/png"}]}}"#
            ),
            AgentEvent::UserMessage {
                text: "mira".into(),
                images: 2
            }
        );
    }

    #[test]
    fn decodes_user_message_with_images_but_no_text() {
        assert_eq!(
            one(
                r#"{"type":"message_end","message":{"role":"user","content":[{"type":"image","data":"x","mimeType":"image/png"}]}}"#
            ),
            AgentEvent::UserMessage {
                text: String::new(),
                images: 1
            }
        );
    }

    #[test]
    fn legacy_user_message_events_deserialize_without_images() {
        let event: AgentEvent =
            serde_json::from_str(r#"{"type":"user_message","text":"hola"}"#).expect("legacy");
        assert_eq!(
            event,
            AgentEvent::UserMessage {
                text: "hola".into(),
                images: 0
            }
        );
    }

    #[test]
    fn decodes_deltas() {
        assert_eq!(
            one(
                r#"{"type":"message_update","assistantMessageEvent":{"type":"text_delta","contentIndex":0,"delta":"Ho"}}"#
            ),
            AgentEvent::TextDelta { delta: "Ho".into() }
        );
        assert!(decode(r#"{"type":"message_update","assistantMessageEvent":{"type":"toolcall_delta","delta":"{"}}"#).is_empty());
    }

    #[test]
    fn decodes_tool_lifecycle() {
        assert_eq!(
            one(
                r#"{"type":"tool_execution_start","toolCallId":"c1","toolName":"bash","args":{"command":"ls"}}"#
            ),
            AgentEvent::ToolStarted {
                call_id: "c1".into(),
                tool_name: "bash".into(),
                args: r#"{"command":"ls"}"#.into()
            }
        );
        assert_eq!(
            one(
                r#"{"type":"tool_execution_end","toolCallId":"c1","toolName":"bash","result":{"content":[{"type":"text","text":"a\nb"}]},"isError":true}"#
            ),
            AgentEvent::ToolFinished {
                call_id: "c1".into(),
                tool_name: "bash".into(),
                output: "a\nb".into(),
                is_error: true
            }
        );
    }

    #[test]
    fn decodes_dialogs_and_notifications() {
        let select: AgentEvent = one(
            r#"{"type":"extension_ui_request","id":"u1","method":"select","title":"¿Permitir?","options":["Sí","No"],"timeout":10000}"#,
        );
        assert_eq!(
            select,
            AgentEvent::DialogRequested(DialogRequest {
                id: "u1".into(),
                title: "¿Permitir?".into(),
                kind: DialogKind::Select {
                    options: vec!["Sí".into(), "No".into()]
                },
                timeout_ms: Some(10000)
            })
        );
        assert_eq!(
            one(
                r#"{"type":"extension_ui_request","id":"u2","method":"notify","message":"ojo","notifyType":"warning"}"#
            ),
            AgentEvent::Notice {
                level: NoticeLevel::Warning,
                message: "ojo".into()
            }
        );
        assert!(
            decode(r#"{"type":"extension_ui_request","id":"u3","method":"setTitle","title":"x"}"#)
                .is_empty()
        );
    }

    fn status(text: Option<&str>) -> AgentEvent {
        AgentEvent::StatusChanged {
            key: "gentle-jobs".into(),
            text: text.map(str::to_string),
        }
    }

    fn last_activity(events: &[AgentEvent]) -> Option<(u32, u32)> {
        events.iter().rev().find_map(|e: &AgentEvent| match e {
            AgentEvent::SubagentsChanged { active, waiting } => Some((*active, *waiting)),
            _ => None,
        })
    }

    #[test]
    fn background_jobs_count_as_active_work() {
        let mut tracker: ActivityTracker = ActivityTracker::default();
        let events: Vec<AgentEvent> = tracker.fold(vec![status(Some("\u{29d7} 2 jobs"))]);
        assert_eq!(events[0], status(Some("\u{29d7} 2 jobs")));
        assert_eq!(last_activity(&events), Some((2, 0)));
        let cleared: Vec<AgentEvent> = tracker.fold(vec![status(None)]);
        assert_eq!(last_activity(&cleared), Some((0, 0)));
    }

    #[test]
    fn jobs_and_subagents_are_summed_and_survive_each_other() {
        let mut tracker: ActivityTracker = ActivityTracker::default();
        tracker.fold(vec![status(Some("\u{29d7} 1 job"))]);
        let events: Vec<AgentEvent> = tracker.fold(vec![AgentEvent::SubagentsChanged {
            active: 2,
            waiting: 1,
        }]);
        assert_eq!(last_activity(&events), Some((3, 1)));
        let events: Vec<AgentEvent> = tracker.fold(vec![AgentEvent::SubagentsChanged {
            active: 0,
            waiting: 0,
        }]);
        assert_eq!(last_activity(&events), Some((1, 0)));
    }

    #[test]
    fn other_events_pass_through_untouched() {
        let mut tracker: ActivityTracker = ActivityTracker::default();
        let other: AgentEvent = AgentEvent::StatusChanged {
            key: "mcp".into(),
            text: Some("ok".into()),
        };
        assert_eq!(tracker.fold(vec![other.clone()]), vec![other]);
    }

    #[test]
    fn decodes_status_without_ansi_and_widget_clear() {
        assert_eq!(
            one(
                "{\"type\":\"extension_ui_request\",\"id\":\"u\",\"method\":\"setStatus\",\"statusKey\":\"mcp\",\"statusText\":\"\\u001b[38;2;1;2;3mMCP ok\\u001b[39m\"}"
            ),
            AgentEvent::StatusChanged {
                key: "mcp".into(),
                text: Some("MCP ok".into())
            }
        );
        assert_eq!(
            one(
                r#"{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"btw"}"#
            ),
            AgentEvent::WidgetChanged {
                key: "btw".into(),
                lines: None
            }
        );
    }

    #[test]
    fn decodes_subagent_activity_from_gentle_agents_widget() {
        let events: Vec<AgentEvent> = decode(
            r#"{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"gentle-agents","widgetLines":["{\"schema\":\"gentle-agents.activity/v1\",\"summary\":{\"running\":2,\"queued\":1,\"waiting\":1,\"finished\":4},\"tasks\":[]}"]}"#,
        );
        assert_eq!(events.len(), 2);
        assert!(matches!(events[0], AgentEvent::WidgetChanged { .. }));
        assert_eq!(
            events[1],
            AgentEvent::SubagentsChanged {
                active: 3,
                waiting: 1
            }
        );
    }

    #[test]
    fn clearing_gentle_agents_widget_means_no_subagents() {
        let events: Vec<AgentEvent> = decode(
            r#"{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"gentle-agents"}"#,
        );
        assert_eq!(
            events.last(),
            Some(&AgentEvent::SubagentsChanged {
                active: 0,
                waiting: 0
            })
        );
    }

    #[test]
    fn other_widgets_and_malformed_payloads_do_not_change_subagents() {
        assert!(!decode(
            r#"{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"btw","widgetLines":["x"]}"#
        )
        .iter()
        .any(|e: &AgentEvent| matches!(e, AgentEvent::SubagentsChanged { .. })));
        assert!(!decode(
            r#"{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"gentle-agents","widgetLines":["no json"]}"#
        )
        .iter()
        .any(|e: &AgentEvent| matches!(e, AgentEvent::SubagentsChanged { .. })));
    }

    #[test]
    fn failed_responses_become_error_notices() {
        assert_eq!(
            one(
                r#"{"id":"r","type":"response","command":"steer","success":false,"error":"not streaming"}"#
            ),
            AgentEvent::Notice {
                level: NoticeLevel::Error,
                message: "steer rechazado: not streaming".into()
            }
        );
        assert!(
            decode(r#"{"id":"r","type":"response","command":"prompt","success":true}"#).is_empty()
        );
    }

    #[test]
    fn truncates_long_tool_output() {
        let long: String = "x".repeat(MAX_TOOL_TEXT_CHARS + 10);
        assert!(truncate(&long).ends_with("(truncado)"));
    }

    fn widget(payload: &str) -> String {
        json!({
            "type": "extension_ui_request",
            "id": "u",
            "method": "setWidget",
            "widgetKey": "gentle-agents",
            "widgetLines": [payload],
        })
        .to_string()
    }

    #[test]
    fn decodes_the_full_subagent_snapshot() {
        let payload: String = json!({
            "schema": "gentle-agents.activity/v1",
            "summary": {"running": 1, "queued": 0, "waiting": 0, "finished": 1},
            "tasks": [
                {
                    "summary": {
                        "id": "t1", "agent": "explore", "label": "Mapa", "prompt": "Explora",
                        "status": "running", "createdAt": 10, "startedAt": 11, "endedAt": null,
                        "lastStep": "leyendo", "lastActivityAt": 12, "turns": 2,
                        "toolCalls": 3, "error": null
                    },
                    "thread": {"version": 1, "dropped": 0, "items": [
                        {"kind": "text", "text": "hola"},
                        {"kind": "thinking", "text": "pienso"},
                        {"kind": "note", "text": "nota"},
                        {"kind": "tool", "name": "read", "args": "{}", "running": false,
                         "isError": true, "output": "boom"}
                    ]}
                },
                {"summary": {"id": "t2", "agent": "plan", "status": "timed_out",
                             "error": "tarde"}}
            ]
        })
        .to_string();

        let tasks: Vec<SubagentTask> =
            decode_subagent_tasks(&widget(&payload)).expect("instantánea");

        assert_eq!(tasks.len(), 2);
        assert_eq!(tasks[0].id, "t1");
        assert_eq!(tasks[0].agent, "explore");
        assert_eq!(tasks[0].label.as_deref(), Some("Mapa"));
        assert_eq!(tasks[0].prompt.as_deref(), Some("Explora"));
        assert_eq!(tasks[0].status, SubagentStatus::Running);
        assert_eq!(
            (tasks[0].created_at, tasks[0].started_at, tasks[0].ended_at),
            (Some(10), Some(11), None)
        );
        assert_eq!(tasks[0].last_step.as_deref(), Some("leyendo"));
        assert_eq!(tasks[0].last_activity_at, Some(12));
        assert_eq!((tasks[0].turns, tasks[0].tool_calls), (2, 3));
        assert_eq!(
            tasks[0].thread,
            vec![
                SubagentThreadItem::Text("hola".into()),
                SubagentThreadItem::Thinking("pienso".into()),
                SubagentThreadItem::Note("nota".into()),
                SubagentThreadItem::Tool {
                    name: "read".into(),
                    args: Some("{}".into()),
                    running: false,
                    is_error: true,
                    output: Some("boom".into()),
                },
            ]
        );
        assert_eq!(tasks[1].status, SubagentStatus::TimedOut);
        assert_eq!(tasks[1].error.as_deref(), Some("tarde"));
        assert_eq!((tasks[1].turns, tasks[1].tool_calls), (0, 0));
        assert!(tasks[1].thread.is_empty());
    }

    #[test]
    fn maps_every_documented_status_and_falls_back_to_unknown() {
        for (raw, expected) in [
            ("queued", SubagentStatus::Queued),
            ("running", SubagentStatus::Running),
            ("waiting", SubagentStatus::Waiting),
            ("completed", SubagentStatus::Completed),
            ("failed", SubagentStatus::Failed),
            ("cancelled", SubagentStatus::Cancelled),
            ("timed_out", SubagentStatus::TimedOut),
            ("exploded", SubagentStatus::Unknown),
        ] {
            let payload: String = json!({"tasks": [{"summary": {"id": "a", "agent": "x",
                "status": raw}}]})
            .to_string();
            let tasks: Vec<SubagentTask> =
                decode_subagent_tasks(&widget(&payload)).expect("instantánea");
            assert_eq!(tasks[0].status, expected, "{raw}");
        }
        let missing: String =
            json!({"tasks": [{"summary": {"id": "a", "agent": "x"}}]}).to_string();
        assert_eq!(
            decode_subagent_tasks(&widget(&missing)).expect("instantánea")[0].status,
            SubagentStatus::Unknown
        );
    }

    #[test]
    fn unknown_thread_items_degrade_to_notes_or_are_skipped() {
        let payload: String = json!({"tasks": [{"summary": {"id": "a", "agent": "x"},
        "thread": {"items": [
            {"kind": "hologram", "text": "raro"},
            {"kind": "hologram"},
            {"kind": "tool"},
            "no soy un objeto"
        ]}}]})
        .to_string();

        let tasks: Vec<SubagentTask> =
            decode_subagent_tasks(&widget(&payload)).expect("instantánea");

        assert_eq!(
            tasks[0].thread,
            vec![
                SubagentThreadItem::Note("raro".into()),
                SubagentThreadItem::Tool {
                    name: String::new(),
                    args: None,
                    running: false,
                    is_error: false,
                    output: None,
                },
            ]
        );
    }

    #[test]
    fn tasks_without_an_id_are_skipped() {
        let payload: String =
            json!({"tasks": [{"summary": {"agent": "x"}}, {"summary": {"id": "ok", "agent": "x"}}]})
                .to_string();

        let tasks: Vec<SubagentTask> =
            decode_subagent_tasks(&widget(&payload)).expect("instantánea");

        assert_eq!(tasks.len(), 1);
        assert_eq!(tasks[0].id, "ok");
    }

    #[test]
    fn cleared_widget_is_an_empty_snapshot() {
        assert_eq!(
            decode_subagent_tasks(
                r#"{"type":"extension_ui_request","id":"u","method":"setWidget","widgetKey":"gentle-agents"}"#
            ),
            Some(Vec::new())
        );
    }

    #[test]
    fn malformed_or_foreign_lines_are_not_a_snapshot() {
        assert_eq!(decode_subagent_tasks(&widget("no json")), None);
        assert_eq!(decode_subagent_tasks(&widget(r#"{"summary":{}}"#)), None);
        assert_eq!(decode_subagent_tasks(&widget(r#"{"tasks":"x"}"#)), None);
        assert_eq!(
            decode_subagent_tasks(&widget(r#"{"schema":"otra/v1","tasks":[]}"#)),
            None
        );
        assert_eq!(decode_subagent_tasks("basura"), None);
        assert_eq!(decode_subagent_tasks(r#"{"type":"agent_start"}"#), None);
        assert_eq!(
            decode_subagent_tasks(
                r#"{"type":"extension_ui_request","method":"setWidget","widgetKey":"otro","widgetLines":["{}"]}"#
            ),
            None
        );
    }
}
