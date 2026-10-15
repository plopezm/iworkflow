//! Traducción entre el protocolo stream-json de `claude -p` y el dominio.
//!
//! Un [`ClaudeProtocol`] por proceso: el decoder necesita recordar el nombre de cada
//! herramienta (el resultado solo trae el id) y las entradas de los permisos pendientes (la
//! respuesta `allow` debe devolverlas en `updatedInput`).

use std::collections::HashMap;

use serde_json::{Value, json};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::{
    AgentEvent, DialogAnswer, DialogKind, DialogRequest, NoticeLevel,
};
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;

/// Límite de caracteres de salidas y argumentos de herramientas que se conservan.
const MAX_TOOL_TEXT_CHARS: usize = 20_000;
const DENY_MESSAGE: &str = "El usuario rechazó el uso de la herramienta.";
const INTERRUPTED_MESSAGE: &str = "ejecución interrumpida";

#[derive(Debug, Default)]
pub struct ClaudeProtocol {
    running: bool,
    tool_names: HashMap<String, String>,
    pending_permissions: HashMap<String, Value>,
}

impl ClaudeProtocol {
    pub fn new() -> Self {
        Self::default()
    }

    /// Eventos de dominio de una línea de salida de `claude`. Las líneas que no son JSON o que
    /// no interesan (hooks, límites de uso, subagentes) no producen eventos.
    pub fn decode(&mut self, line: &str) -> Vec<AgentEvent> {
        let Ok(record) = serde_json::from_str::<Value>(line) else {
            return Vec::new();
        };
        if record
            .get("parent_tool_use_id")
            .is_some_and(|v: &Value| !v.is_null())
        {
            return Vec::new();
        }
        match str_field(&record, "type").unwrap_or_default() {
            "stream_event" => self.decode_stream_event(&record),
            "assistant" => self.decode_assistant(&record),
            "user" => self.decode_user(&record),
            "control_request" => self.decode_control_request(&record),
            "result" => self.decode_result(&record),
            _ => Vec::new(),
        }
    }

    /// Línea JSONL (sin salto final) para una orden. `request_id` identifica la petición de
    /// control cuando la orden la necesita.
    pub fn encode(
        &mut self,
        command: &AgentCommand,
        request_id: &str,
    ) -> Result<String, TaskError> {
        let record: Value = match command {
            AgentCommand::Prompt { message, images }
            | AgentCommand::Steer { message, images }
            | AgentCommand::FollowUp { message, images } => user_message(message, images),
            AgentCommand::Abort => json!({
                "type": "control_request",
                "request_id": request_id,
                "request": { "subtype": "interrupt" },
            }),
            AgentCommand::AnswerDialog { dialog_id, answer } => {
                let input: Value = self.pending_permissions.remove(dialog_id).ok_or_else(|| {
                    TaskError::Runtime(format!("no hay un permiso pendiente con id {dialog_id}"))
                })?;
                permission_response(dialog_id, answer, input)
            }
        };
        Ok(record.to_string())
    }

    /// Marca el inicio de una ejecución; solo emite `RunStarted` la primera vez.
    fn start_run(&mut self, events: &mut Vec<AgentEvent>) {
        if !self.running {
            self.running = true;
            events.push(AgentEvent::RunStarted);
        }
    }

    fn decode_stream_event(&mut self, record: &Value) -> Vec<AgentEvent> {
        let Some(event) = record.get("event") else {
            return Vec::new();
        };
        if str_field(event, "type") != Some("content_block_delta") {
            return Vec::new();
        }
        let Some(delta) = event.get("delta") else {
            return Vec::new();
        };
        let decoded: Option<AgentEvent> = match str_field(delta, "type") {
            Some("text_delta") => Some(AgentEvent::TextDelta {
                delta: string_of(delta, "text"),
            }),
            Some("thinking_delta") => Some(AgentEvent::ThinkingDelta {
                delta: string_of(delta, "thinking"),
            }),
            _ => None,
        };
        let mut events: Vec<AgentEvent> = Vec::new();
        if let Some(decoded) = decoded {
            self.start_run(&mut events);
            events.push(decoded);
        }
        events
    }

    fn decode_assistant(&mut self, record: &Value) -> Vec<AgentEvent> {
        let blocks: &[Value] = content_blocks(record);
        let text: String = join_blocks(blocks, "text", "text");
        let thinking: String = join_blocks(blocks, "thinking", "thinking");
        let mut events: Vec<AgentEvent> = Vec::new();
        self.start_run(&mut events);
        if !text.is_empty() || !thinking.is_empty() {
            events.push(AgentEvent::AssistantMessage {
                text,
                thinking: (!thinking.is_empty()).then_some(thinking),
                error: None,
            });
        }
        for block in blocks
            .iter()
            .filter(|b: &&Value| str_field(b, "type") == Some("tool_use"))
        {
            let call_id: String = string_of(block, "id");
            let tool_name: String = string_of(block, "name");
            self.tool_names.insert(call_id.clone(), tool_name.clone());
            events.push(AgentEvent::ToolStarted {
                call_id,
                tool_name,
                args: truncate(block.get("input").map(Value::to_string).unwrap_or_default()),
            });
        }
        events
    }

    fn decode_user(&mut self, record: &Value) -> Vec<AgentEvent> {
        let blocks: &[Value] = content_blocks(record);
        let results: Vec<AgentEvent> = blocks
            .iter()
            .filter(|b: &&Value| str_field(b, "type") == Some("tool_result"))
            .map(|block: &Value| {
                let call_id: String = string_of(block, "tool_use_id");
                AgentEvent::ToolFinished {
                    tool_name: self.tool_names.remove(&call_id).unwrap_or_default(),
                    call_id,
                    output: truncate(tool_result_text(block.get("content"))),
                    is_error: block
                        .get("is_error")
                        .and_then(Value::as_bool)
                        .unwrap_or(false),
                }
            })
            .collect();
        if !results.is_empty() {
            return results;
        }
        // Sin `--replay-user-messages` el eco de stdin no llega; solo se muestran los reemitidos.
        if record.get("isReplay").and_then(Value::as_bool) != Some(true) {
            return Vec::new();
        }
        let text: String = match record.pointer("/message/content") {
            Some(Value::String(text)) => text.clone(),
            _ => join_blocks(blocks, "text", "text"),
        };
        let images: u32 = blocks
            .iter()
            .filter(|b: &&Value| str_field(b, "type") == Some("image"))
            .count() as u32;
        if text.is_empty() && images == 0 {
            return Vec::new();
        }
        vec![AgentEvent::UserMessage { text, images }]
    }

    fn decode_control_request(&mut self, record: &Value) -> Vec<AgentEvent> {
        let Some(request) = record.get("request") else {
            return Vec::new();
        };
        if str_field(request, "subtype") != Some("can_use_tool") {
            return Vec::new();
        }
        let id: String = string_of(record, "request_id");
        let tool: &str = str_field(request, "display_name")
            .or_else(|| str_field(request, "tool_name"))
            .unwrap_or("herramienta");
        let message: Option<String> = str_field(request, "description")
            .map(str::to_string)
            .or_else(|| {
                request
                    .get("input")
                    .map(|input: &Value| truncate(input.to_string()))
            });
        self.pending_permissions.insert(
            id.clone(),
            request.get("input").cloned().unwrap_or_else(|| json!({})),
        );
        vec![AgentEvent::DialogRequested(DialogRequest {
            id,
            title: format!("Permitir {tool}"),
            kind: DialogKind::Confirm { message },
            timeout_ms: None,
        })]
    }

    fn decode_result(&mut self, record: &Value) -> Vec<AgentEvent> {
        self.running = false;
        let failed: bool = record
            .get("is_error")
            .and_then(Value::as_bool)
            .unwrap_or(false)
            || str_field(record, "subtype").is_some_and(|s: &str| s != "success");
        let mut events: Vec<AgentEvent> = Vec::new();
        if failed {
            events.push(
                if str_field(record, "terminal_reason") == Some("aborted_streaming") {
                    notice(NoticeLevel::Warning, INTERRUPTED_MESSAGE.to_string())
                } else {
                    notice(
                        NoticeLevel::Error,
                        str_field(record, "result")
                            .or_else(|| str_field(record, "subtype"))
                            .unwrap_or("error del agente")
                            .to_string(),
                    )
                },
            );
        }
        events.push(AgentEvent::RunSettled);
        events
    }
}

/// Mensaje de usuario: texto simple, o bloques de contenido si lleva imágenes.
fn user_message(message: &str, images: &[ImageAttachment]) -> Value {
    let content: Value = if images.is_empty() {
        json!(message)
    } else {
        let mut blocks: Vec<Value> = vec![json!({ "type": "text", "text": message })];
        blocks.extend(images.iter().map(|image: &ImageAttachment| {
            json!({
                "type": "image",
                "source": { "type": "base64", "media_type": image.mime_type, "data": image.data },
            })
        }));
        Value::Array(blocks)
    };
    json!({ "type": "user", "message": { "role": "user", "content": content } })
}

/// Respuesta a `can_use_tool`: solo `Confirmed { true }` permite la herramienta.
fn permission_response(request_id: &str, answer: &DialogAnswer, input: Value) -> Value {
    let decision: Value = match answer {
        DialogAnswer::Confirmed { confirmed: true } => {
            json!({ "behavior": "allow", "updatedInput": input })
        }
        _ => json!({ "behavior": "deny", "message": DENY_MESSAGE }),
    };
    json!({
        "type": "control_response",
        "response": { "subtype": "success", "request_id": request_id, "response": decision },
    })
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

/// Bloques de contenido de `message.content` (vacío si es una cadena o no existe).
fn content_blocks(record: &Value) -> &[Value] {
    record
        .pointer("/message/content")
        .and_then(Value::as_array)
        .map(Vec::as_slice)
        .unwrap_or_default()
}

/// Concatena el campo `field` de los bloques de tipo `kind`, separados por salto de línea.
fn join_blocks(blocks: &[Value], kind: &str, field: &str) -> String {
    blocks
        .iter()
        .filter(|b: &&Value| str_field(b, "type") == Some(kind))
        .filter_map(|b: &Value| str_field(b, field))
        .filter(|t: &&str| !t.is_empty())
        .collect::<Vec<&str>>()
        .join("\n")
}

/// Texto de un `tool_result`: cadena o lista de bloques de texto.
fn tool_result_text(content: Option<&Value>) -> String {
    match content {
        Some(Value::String(text)) => text.clone(),
        Some(Value::Array(blocks)) => join_blocks(blocks, "text", "text"),
        _ => String::new(),
    }
}

fn truncate(text: String) -> String {
    if text.chars().count() <= MAX_TOOL_TEXT_CHARS {
        return text;
    }
    text.chars().take(MAX_TOOL_TEXT_CHARS).collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::domain::models::agent_event::DialogAnswer;

    fn decode_all(protocol: &mut ClaudeProtocol, lines: &[Value]) -> Vec<AgentEvent> {
        lines
            .iter()
            .flat_map(|line: &Value| protocol.decode(&line.to_string()))
            .collect()
    }

    fn stream_delta(delta: Value) -> Value {
        json!({"type": "stream_event", "parent_tool_use_id": null,
               "event": {"type": "content_block_delta", "index": 0, "delta": delta}})
    }

    fn result_line(subtype: &str, is_error: bool) -> Value {
        json!({"type": "result", "subtype": subtype, "is_error": is_error, "result": "boom"})
    }

    #[test]
    fn text_and_thinking_deltas_start_the_run_once() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                stream_delta(json!({"type": "text_delta", "text": "ho"})),
                stream_delta(json!({"type": "thinking_delta", "thinking": "mm"})),
            ],
        );
        assert_eq!(
            events,
            vec![
                AgentEvent::RunStarted,
                AgentEvent::TextDelta { delta: "ho".into() },
                AgentEvent::ThinkingDelta { delta: "mm".into() },
            ]
        );
    }

    #[test]
    fn assistant_text_and_thinking_become_one_message() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "assistant", "parent_tool_use_id": null, "message": {"content": [
                {"type": "thinking", "thinking": "pienso"},
                {"type": "text", "text": "hola"}]}}),
            ],
        );
        assert_eq!(
            events,
            vec![
                AgentEvent::RunStarted,
                AgentEvent::AssistantMessage {
                    text: "hola".into(),
                    thinking: Some("pienso".into()),
                    error: None
                },
            ]
        );
    }

    #[test]
    fn empty_thinking_signature_blocks_produce_nothing() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[json!({"type": "assistant", "message": {"content": [
                {"type": "thinking", "thinking": "", "signature": "abc"}]}})],
        );
        assert_eq!(events, vec![AgentEvent::RunStarted]);
    }

    #[test]
    fn tool_use_and_result_are_paired_by_id() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "assistant", "message": {"content": [
                    {"type": "tool_use", "id": "t1", "name": "Bash", "input": {"command": "ls"}}]}}),
                json!({"type": "user", "message": {"content": [
                    {"type": "tool_result", "tool_use_id": "t1", "content": "a.txt", "is_error": false}]}}),
            ],
        );
        assert_eq!(
            events,
            vec![
                AgentEvent::RunStarted,
                AgentEvent::ToolStarted {
                    call_id: "t1".into(),
                    tool_name: "Bash".into(),
                    args: r#"{"command":"ls"}"#.into()
                },
                AgentEvent::ToolFinished {
                    call_id: "t1".into(),
                    tool_name: "Bash".into(),
                    output: "a.txt".into(),
                    is_error: false
                },
            ]
        );
    }

    #[test]
    fn tool_result_blocks_list_is_joined_and_error_flag_kept() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[json!({"type": "user", "message": {"content": [
                {"type": "tool_result", "tool_use_id": "x", "is_error": true,
                 "content": [{"type": "text", "text": "uno"}, {"type": "text", "text": "dos"}]}]}})],
        );
        assert_eq!(
            events,
            vec![AgentEvent::ToolFinished {
                call_id: "x".into(),
                tool_name: String::new(),
                output: "uno\ndos".into(),
                is_error: true
            }]
        );
    }

    #[test]
    fn replayed_user_message_is_emitted_but_plain_stdin_echo_is_not() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "user", "isReplay": true, "message": {"content": "di hola"}}),
                json!({"type": "user", "message": {"content": "otro"}}),
            ],
        );
        assert_eq!(
            events,
            vec![AgentEvent::UserMessage {
                text: "di hola".into(),
                images: 0
            }]
        );
    }

    #[test]
    fn subagent_lines_are_ignored() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[json!({"type": "assistant", "parent_tool_use_id": "p1",
                     "message": {"content": [{"type": "text", "text": "sub"}]}})],
        );
        assert!(events.is_empty());
    }

    #[test]
    fn permission_request_becomes_a_confirm_dialog() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "control_request", "request_id": "r1", "request": {
                "subtype": "can_use_tool", "tool_name": "Bash", "display_name": "Bash",
                "description": "Escribe a.txt", "input": {"command": "echo hola > a.txt"}}}),
            ],
        );
        assert_eq!(
            events,
            vec![AgentEvent::DialogRequested(DialogRequest {
                id: "r1".into(),
                title: "Permitir Bash".into(),
                kind: DialogKind::Confirm {
                    message: Some("Escribe a.txt".into())
                },
                timeout_ms: None
            })]
        );
    }

    #[test]
    fn other_control_requests_and_noise_are_ignored() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "control_request", "request_id": "r", "request": {"subtype": "hook_callback"}}),
                json!({"type": "control_response", "response": {"subtype": "success"}}),
                json!({"type": "system", "subtype": "init"}),
                json!({"type": "rate_limit_event"}),
            ],
        );
        assert!(events.is_empty());
        assert!(protocol.decode("no es json").is_empty());
    }

    #[test]
    fn successful_result_settles_the_run_and_a_new_run_restarts() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                stream_delta(json!({"type": "text_delta", "text": "a"})),
                result_line("success", false),
                stream_delta(json!({"type": "text_delta", "text": "b"})),
            ],
        );
        assert_eq!(
            events,
            vec![
                AgentEvent::RunStarted,
                AgentEvent::TextDelta { delta: "a".into() },
                AgentEvent::RunSettled,
                AgentEvent::RunStarted,
                AgentEvent::TextDelta { delta: "b".into() },
            ]
        );
    }

    #[test]
    fn aborted_result_warns_and_failed_result_reports_the_error() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let aborted: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "result", "subtype": "error_during_execution",
                     "is_error": true, "terminal_reason": "aborted_streaming"}),
            ],
        );
        assert_eq!(
            aborted,
            vec![
                AgentEvent::Notice {
                    level: NoticeLevel::Warning,
                    message: "ejecución interrumpida".into()
                },
                AgentEvent::RunSettled
            ]
        );
        let failed: Vec<AgentEvent> =
            decode_all(&mut protocol, &[result_line("error_max_turns", true)]);
        assert_eq!(
            failed,
            vec![
                AgentEvent::Notice {
                    level: NoticeLevel::Error,
                    message: "boom".into()
                },
                AgentEvent::RunSettled
            ]
        );
    }

    #[test]
    fn long_tool_output_is_truncated() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let big: String = "x".repeat(MAX_TOOL_TEXT_CHARS + 50);
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[json!({"type": "user", "message": {"content": [
                {"type": "tool_result", "tool_use_id": "t", "content": big}]}})],
        );
        let AgentEvent::ToolFinished { output, .. } = &events[0] else {
            panic!("se esperaba ToolFinished");
        };
        assert_eq!(output.chars().count(), MAX_TOOL_TEXT_CHARS);
    }

    fn parse(line: &str) -> Value {
        serde_json::from_str(line).expect("json válido")
    }

    #[test]
    fn prompt_is_a_user_message_with_plain_text() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let line: String = protocol
            .encode(
                &AgentCommand::Prompt {
                    message: "hola".into(),
                    images: Vec::new(),
                },
                "id",
            )
            .expect("prompt");
        assert_eq!(
            parse(&line),
            json!({"type": "user", "message": {"role": "user", "content": "hola"}})
        );
        assert!(!line.contains('\n'));
    }

    #[test]
    fn prompt_with_images_uses_content_blocks() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let line: String = protocol
            .encode(
                &AgentCommand::Prompt {
                    message: "mira".into(),
                    images: vec![ImageAttachment {
                        name: "a.png".into(),
                        mime_type: "image/png".into(),
                        data: "QUJD".into(),
                    }],
                },
                "id",
            )
            .expect("prompt");
        assert_eq!(
            parse(&line)["message"]["content"],
            json!([
                {"type": "text", "text": "mira"},
                {"type": "image", "source": {"type": "base64", "media_type": "image/png", "data": "QUJD"}}
            ])
        );
    }

    #[test]
    fn abort_is_an_interrupt_control_request() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let line: String = protocol
            .encode(&AgentCommand::Abort, "req-9")
            .expect("abort");
        assert_eq!(
            parse(&line),
            json!({"type": "control_request", "request_id": "req-9",
                   "request": {"subtype": "interrupt"}})
        );
    }

    fn request_permission(protocol: &mut ClaudeProtocol) {
        protocol.decode(
            &json!({"type": "control_request", "request_id": "r1", "request": {
                "subtype": "can_use_tool", "tool_name": "Bash",
                "input": {"command": "ls"}}})
            .to_string(),
        );
    }

    #[test]
    fn confirmed_dialog_allows_the_tool_with_its_original_input() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        request_permission(&mut protocol);
        let line: String = protocol
            .encode(
                &AgentCommand::AnswerDialog {
                    dialog_id: "r1".into(),
                    answer: DialogAnswer::Confirmed { confirmed: true },
                },
                "x",
            )
            .expect("answer");
        assert_eq!(
            parse(&line),
            json!({"type": "control_response", "response": {
                "subtype": "success", "request_id": "r1",
                "response": {"behavior": "allow", "updatedInput": {"command": "ls"}}}})
        );
    }

    #[test]
    fn declined_or_cancelled_dialog_denies_the_tool() {
        for answer in [
            DialogAnswer::Confirmed { confirmed: false },
            DialogAnswer::Cancelled,
        ] {
            let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
            request_permission(&mut protocol);
            let line: String = protocol
                .encode(
                    &AgentCommand::AnswerDialog {
                        dialog_id: "r1".into(),
                        answer,
                    },
                    "x",
                )
                .expect("answer");
            assert_eq!(
                parse(&line)["response"]["response"],
                json!({"behavior": "deny", "message": DENY_MESSAGE})
            );
        }
    }

    #[test]
    fn answering_an_unknown_or_already_answered_dialog_fails() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        request_permission(&mut protocol);
        let answer: AgentCommand = AgentCommand::AnswerDialog {
            dialog_id: "r1".into(),
            answer: DialogAnswer::Confirmed { confirmed: true },
        };
        assert!(protocol.encode(&answer, "x").is_ok());
        assert!(protocol.encode(&answer, "x").is_err());
    }
}
