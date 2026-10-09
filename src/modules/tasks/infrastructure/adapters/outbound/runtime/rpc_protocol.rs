//! Traducción entre el protocolo JSONL de `pi --mode rpc` y el dominio.

use serde_json::{Map, Value, json};

use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::{
    AgentEvent, DialogAnswer, DialogKind, DialogRequest, NoticeLevel,
};
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;

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
pub fn encode(command: &AgentCommand, request_id: &str) -> String {
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
    record.to_string()
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
    events.extend(decode_subagents(&record));
    events
}

/// Clave del widget con el que la extensión de subagentes publica su actividad.
const SUBAGENTS_WIDGET_KEY: &str = "gentle-agents";

/// Resumen de subagentes vivos a partir del widget `gentle-agents` (primera línea JSON
/// con `summary: {running, queued, waiting}`). Limpiar el widget equivale a ninguno.
fn decode_subagents(record: &Value) -> Option<AgentEvent> {
    if str_field(record, "type") != Some("extension_ui_request")
        || str_field(record, "method") != Some("setWidget")
        || str_field(record, "widgetKey") != Some(SUBAGENTS_WIDGET_KEY)
    {
        return None;
    }
    let first_line: Option<&str> = record
        .get("widgetLines")
        .and_then(Value::as_array)
        .and_then(|lines: &Vec<Value>| lines.first())
        .and_then(Value::as_str);
    let Some(line) = first_line else {
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
}
