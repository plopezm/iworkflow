//! Traducción entre el protocolo stream-json de `claude -p` y el dominio.
//!
//! Un [`ClaudeProtocol`] por proceso: el decoder necesita recordar el nombre de cada
//! herramienta (el resultado solo trae el id) y las entradas de los permisos pendientes (la
//! respuesta `allow` debe devolverlas en `updatedInput`).

use std::collections::{HashMap, HashSet};

use serde_json::{Value, json};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::{
    AgentEvent, DialogAnswer, DialogKind, DialogRequest, NoticeLevel,
};
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::domain::models::usage_limit::looks_like_usage_limit;

/// Límite de caracteres de salidas y argumentos de herramientas que se conservan.
const MAX_TOOL_TEXT_CHARS: usize = 20_000;
/// Límite del resumen de la entrada de una herramienta en el diálogo de permiso.
const MAX_SUMMARY_CHARS: usize = 500;
const DENY_PLAN_MESSAGE: &str = "El usuario rechazó el plan; ajústalo y vuelve a proponerlo.";
const DENY_MESSAGE: &str = "El usuario rechazó el uso de la herramienta.";
const DENY_QUESTIONS_MESSAGE: &str = "El usuario canceló las preguntas.";
const PLAN_TOOL: &str = "ExitPlanMode";
const QUESTIONS_TOOL: &str = "AskUserQuestion";
/// Separa el id de la petición del índice de pregunta en los diálogos de `AskUserQuestion`.
const QUESTION_ID_SEPARATOR: char = '#';
const INTERRUPTED_MESSAGE: &str = "ejecución interrumpida";

/// Petición de permiso a la espera de la respuesta del usuario.
#[derive(Debug)]
enum Pending {
    /// Herramienta que pide confirmación; `input` se devuelve tal cual si se permite.
    Tool {
        input: Value,
        deny_message: &'static str,
    },
    /// `AskUserQuestion`: un diálogo por pregunta; se responde cuando están todas.
    Questions {
        input: Value,
        questions: Vec<Question>,
        answers: Vec<Option<String>>,
    },
}

#[derive(Debug)]
struct Question {
    text: String,
    /// `(texto mostrado, etiqueta que entiende claude)` de cada opción.
    options: Vec<(String, String)>,
}

#[derive(Debug, Default)]
pub struct ClaudeProtocol {
    running: bool,
    /// Razonamiento recibido por deltas: el bloque `thinking` final llega vacío (solo firma).
    thinking: String,
    tool_names: HashMap<String, String>,
    pending: HashMap<String, Pending>,
    /// Peticiones ya resueltas (canceladas) cuyos diálogos hermanos aún pueden llegar.
    settled: HashSet<String>,
    /// El último `rate_limit_event` rechazó la petición: el `result` de error que sigue es un
    /// límite de uso (el texto del error varía según el plan y el idioma).
    limit_rejected: bool,
    /// Renovación del límite (epoch en segundos) indicada por ese `rate_limit_event`.
    limit_resets_at: Option<i64>,
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
            "rate_limit_event" => {
                self.note_rate_limit(&record);
                Vec::new()
            }
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
    ) -> Result<Option<String>, TaskError> {
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
                return self.answer_dialog(dialog_id, answer);
            }
        };
        Ok(Some(record.to_string()))
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
            Some("thinking_delta") => {
                let text: String = string_of(delta, "thinking");
                self.thinking.push_str(&text);
                Some(AgentEvent::ThinkingDelta { delta: text })
            }
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
        let has_content: bool = !text.is_empty()
            || blocks
                .iter()
                .any(|b: &Value| str_field(b, "type") == Some("tool_use"));
        let block_thinking: String = join_blocks(blocks, "thinking", "thinking");
        // Las líneas solo-thinking (vacías) no consumen lo acumulado: se adjunta al mensaje
        // de texto o a la herramienta que las siguen.
        let thinking: String = if !block_thinking.is_empty() {
            self.thinking.clear();
            block_thinking
        } else if has_content {
            std::mem::take(&mut self.thinking)
        } else {
            String::new()
        };
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
        let tool_name: &str = str_field(request, "tool_name").unwrap_or_default();
        let input: Value = request.get("input").cloned().unwrap_or_else(|| json!({}));
        if tool_name == QUESTIONS_TOOL
            && let Some(questions) = parse_questions(&input)
        {
            return self.request_questions(id, input, questions);
        }
        let display: &str = str_field(request, "display_name")
            .filter(|d: &&str| !d.is_empty())
            .unwrap_or(tool_name);
        let (title, message, deny_message): (String, Option<String>, &'static str) =
            if tool_name == PLAN_TOOL {
                (
                    "Aprobar plan".to_string(),
                    str_field(&input, "plan").map(str::to_string),
                    DENY_PLAN_MESSAGE,
                )
            } else {
                (
                    format!(
                        "Permitir {}",
                        if display.is_empty() {
                            "herramienta"
                        } else {
                            display
                        }
                    ),
                    permission_message(tool_name, str_field(request, "description"), &input),
                    DENY_MESSAGE,
                )
            };
        self.pending.insert(
            id.clone(),
            Pending::Tool {
                input,
                deny_message,
            },
        );
        vec![AgentEvent::DialogRequested(DialogRequest {
            id,
            title,
            kind: DialogKind::Confirm { message },
            timeout_ms: None,
        })]
    }

    /// Un diálogo `Select` por pregunta, con id `<petición>#<índice>`.
    fn request_questions(
        &mut self,
        request_id: String,
        input: Value,
        questions: Vec<(String, Question)>,
    ) -> Vec<AgentEvent> {
        let events: Vec<AgentEvent> = questions
            .iter()
            .enumerate()
            .map(|(index, (title, question)): (usize, &(String, Question))| {
                AgentEvent::DialogRequested(DialogRequest {
                    id: format!("{request_id}{QUESTION_ID_SEPARATOR}{index}"),
                    title: title.clone(),
                    kind: DialogKind::Select {
                        options: question
                            .options
                            .iter()
                            .map(|(shown, _): &(String, String)| shown.clone())
                            .collect(),
                    },
                    timeout_ms: None,
                })
            })
            .collect();
        let questions: Vec<Question> = questions
            .into_iter()
            .map(|(_, question): (String, Question)| question)
            .collect();
        let answers: Vec<Option<String>> = vec![None; questions.len()];
        self.pending.insert(
            request_id,
            Pending::Questions {
                input,
                questions,
                answers,
            },
        );
        events
    }

    fn answer_dialog(
        &mut self,
        dialog_id: &str,
        answer: &DialogAnswer,
    ) -> Result<Option<String>, TaskError> {
        let (request_id, index): (&str, Option<usize>) =
            match dialog_id.rsplit_once(QUESTION_ID_SEPARATOR) {
                Some((request_id, index)) => (request_id, index.parse::<usize>().ok()),
                None => (dialog_id, None),
            };
        let unknown =
            || TaskError::Runtime(format!("no hay un permiso pendiente con id {dialog_id}"));
        // Un id exacto es una petición de herramienta (o de plan); `<petición>#<n>` solo puede
        // referirse a una petición de preguntas, nunca a una herramienta ajena.
        let direct: Option<Pending> = if self.pending.contains_key(dialog_id) {
            self.pending.remove(dialog_id)
        } else if matches!(
            self.pending.get(request_id),
            Some(Pending::Questions { .. })
        ) {
            self.pending.remove(request_id)
        } else {
            None
        };
        let Some(pending) = direct else {
            return if self.settled.contains(request_id) {
                Ok(None)
            } else {
                Err(unknown())
            };
        };
        match pending {
            Pending::Tool {
                input,
                deny_message,
            } => Ok(Some(
                permission_response(dialog_id, answer, input, deny_message).to_string(),
            )),
            Pending::Questions {
                input,
                questions,
                mut answers,
            } => {
                let Some(index) = index.filter(|i: &usize| *i < answers.len()) else {
                    self.pending.insert(
                        request_id.to_string(),
                        Pending::Questions {
                            input,
                            questions,
                            answers,
                        },
                    );
                    return Err(unknown());
                };
                let DialogAnswer::Value { value } = answer else {
                    self.settled.insert(request_id.to_string());
                    return Ok(Some(
                        deny_response(request_id, DENY_QUESTIONS_MESSAGE).to_string(),
                    ));
                };
                answers[index] = Some(label_for(&questions[index], value));
                if answers.iter().any(Option::is_none) {
                    self.pending.insert(
                        request_id.to_string(),
                        Pending::Questions {
                            input,
                            questions,
                            answers,
                        },
                    );
                    return Ok(None);
                }
                Ok(Some(
                    allow_response(request_id, with_answers(input, &questions, &answers))
                        .to_string(),
                ))
            }
        }
    }

    /// Recuerda si claude rechazó la petición por límite de uso y cuándo se renueva.
    fn note_rate_limit(&mut self, record: &Value) {
        let Some(info) = record.get("rate_limit_info") else {
            return;
        };
        let Some(status) = str_field(info, "status") else {
            return;
        };
        self.limit_rejected = !matches!(status, "allowed" | "allowed_warning");
        self.limit_resets_at = self
            .limit_rejected
            .then(|| info.get("resetsAt").and_then(Value::as_i64))
            .flatten();
    }

    fn decode_result(&mut self, record: &Value) -> Vec<AgentEvent> {
        self.running = false;
        self.thinking.clear();
        self.tool_names.clear();
        self.pending.clear();
        self.settled.clear();
        let failed: bool = record
            .get("is_error")
            .and_then(Value::as_bool)
            .unwrap_or(false)
            || str_field(record, "subtype").is_some_and(|s: &str| s != "success");
        let limit_rejected: bool = std::mem::take(&mut self.limit_rejected);
        let limit_resets_at: Option<i64> = self.limit_resets_at.take();
        let mut events: Vec<AgentEvent> = Vec::new();
        if failed {
            let message: String = str_field(record, "result")
                .or_else(|| str_field(record, "subtype"))
                .unwrap_or("error del agente")
                .to_string();
            events.push(
                if str_field(record, "terminal_reason") == Some("aborted_streaming") {
                    notice(NoticeLevel::Warning, INTERRUPTED_MESSAGE.to_string())
                } else if limit_rejected || looks_like_usage_limit(&message) {
                    AgentEvent::UsageLimitReached {
                        resets_at: limit_resets_at,
                        message,
                    }
                } else {
                    notice(NoticeLevel::Error, message)
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
fn permission_response(
    request_id: &str,
    answer: &DialogAnswer,
    input: Value,
    deny_message: &str,
) -> Value {
    match answer {
        DialogAnswer::Confirmed { confirmed: true } => allow_response(request_id, input),
        _ => deny_response(request_id, deny_message),
    }
}

fn allow_response(request_id: &str, updated_input: Value) -> Value {
    control_success(
        request_id,
        json!({ "behavior": "allow", "updatedInput": updated_input }),
    )
}

fn deny_response(request_id: &str, message: &str) -> Value {
    control_success(
        request_id,
        json!({ "behavior": "deny", "message": message }),
    )
}

fn control_success(request_id: &str, decision: Value) -> Value {
    json!({
        "type": "control_response",
        "response": { "subtype": "success", "request_id": request_id, "response": decision },
    })
}

/// Preguntas de `AskUserQuestion` con su título; `None` si faltan preguntas u opciones.
fn parse_questions(input: &Value) -> Option<Vec<(String, Question)>> {
    let questions: &Vec<Value> = input.get("questions")?.as_array()?;
    if questions.is_empty() {
        return None;
    }
    questions
        .iter()
        .map(|raw: &Value| {
            let text: String = str_field(raw, "question")
                .filter(|t: &&str| !t.is_empty())?
                .to_string();
            let options: Vec<(String, String)> = raw
                .get("options")?
                .as_array()?
                .iter()
                .map(|option: &Value| {
                    let label: &str =
                        str_field(option, "label").filter(|l: &&str| !l.is_empty())?;
                    let shown: String = match str_field(option, "description") {
                        Some(description) if !description.is_empty() => {
                            format!("{label} — {description}")
                        }
                        _ => label.to_string(),
                    };
                    Some((shown, label.to_string()))
                })
                .collect::<Option<Vec<(String, String)>>>()?;
            if options.is_empty() {
                return None;
            }
            let title: String = match str_field(raw, "header").filter(|h: &&str| !h.is_empty()) {
                Some(header) => format!("{header}: {text}"),
                None => text.clone(),
            };
            Some((title, Question { text, options }))
        })
        .collect()
}

/// Etiqueta de la opción elegida; un texto libre que no es una opción se pasa tal cual.
fn label_for(question: &Question, value: &str) -> String {
    question
        .options
        .iter()
        .find(|(shown, label): &&(String, String)| shown == value || label == value)
        .map(|(_, label): &(String, String)| label.clone())
        .unwrap_or_else(|| value.to_string())
}

/// `input` con el campo `answers` (pregunta → etiqueta) que espera `AskUserQuestion`.
fn with_answers(mut input: Value, questions: &[Question], answers: &[Option<String>]) -> Value {
    let map: serde_json::Map<String, Value> = questions
        .iter()
        .zip(answers)
        .map(|(question, answer): (&Question, &Option<String>)| {
            (
                question.text.clone(),
                Value::String(answer.clone().unwrap_or_default()),
            )
        })
        .collect();
    input["answers"] = Value::Object(map);
    input
}

/// Texto del diálogo de permiso: lo que describe el modelo más lo que realmente se va a
/// ejecutar o tocar (comando, fichero, URL...), para que el usuario decida con datos reales.
fn permission_message(tool: &str, description: Option<&str>, input: &Value) -> Option<String> {
    let key: Option<&str> = match tool {
        "Bash" => Some("command"),
        "Write" | "Edit" | "MultiEdit" | "Read" => Some("file_path"),
        "NotebookEdit" => Some("notebook_path"),
        "WebFetch" => Some("url"),
        "WebSearch" => Some("query"),
        _ => None,
    };
    let summary: Option<String> = key
        .and_then(|k: &str| str_field(input, k))
        .map(str::to_string)
        .or_else(|| {
            input
                .as_object()
                .filter(|o: &&serde_json::Map<String, Value>| !o.is_empty())
                .map(|_| truncate_chars(input.to_string(), MAX_SUMMARY_CHARS))
        });
    let parts: Vec<String> = [summary, description.map(str::to_string)]
        .into_iter()
        .flatten()
        .filter(|p: &String| !p.is_empty())
        .collect();
    let mut unique: Vec<String> = Vec::new();
    for part in parts {
        if !unique.contains(&part) {
            unique.push(part);
        }
    }
    (!unique.is_empty()).then(|| unique.join("\n"))
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
    truncate_chars(text, MAX_TOOL_TEXT_CHARS)
}

fn truncate_chars(text: String, limit: usize) -> String {
    if text.chars().count() <= limit {
        return text;
    }
    text.chars().take(limit).collect()
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

    fn thinking_delta(text: &str) -> Value {
        stream_delta(json!({"type": "thinking_delta", "thinking": text}))
    }

    #[test]
    fn streamed_thinking_is_attached_to_the_next_text_message() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                thinking_delta("pien"),
                thinking_delta("so"),
                json!({"type": "assistant", "message": {"content": [
                    {"type": "thinking", "thinking": "", "signature": "abc"}]}}),
                json!({"type": "assistant", "message": {"content": [
                    {"type": "text", "text": "hola"}]}}),
                json!({"type": "assistant", "message": {"content": [
                    {"type": "text", "text": "otra"}]}}),
            ],
        );
        let messages: Vec<&AgentEvent> = events
            .iter()
            .filter(|e: &&AgentEvent| matches!(e, AgentEvent::AssistantMessage { .. }))
            .collect();
        assert_eq!(
            messages,
            vec![
                &AgentEvent::AssistantMessage {
                    text: "hola".into(),
                    thinking: Some("pienso".into()),
                    error: None
                },
                &AgentEvent::AssistantMessage {
                    text: "otra".into(),
                    thinking: None,
                    error: None
                },
            ]
        );
    }

    #[test]
    fn streamed_thinking_before_a_tool_call_is_emitted_before_the_tool() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                thinking_delta("voy a listar"),
                json!({"type": "assistant", "message": {"content": [
                    {"type": "tool_use", "id": "t1", "name": "Bash", "input": {}}]}}),
            ],
        );
        assert_eq!(
            &events[2..],
            &[
                AgentEvent::AssistantMessage {
                    text: String::new(),
                    thinking: Some("voy a listar".into()),
                    error: None
                },
                AgentEvent::ToolStarted {
                    call_id: "t1".into(),
                    tool_name: "Bash".into(),
                    args: "{}".into()
                },
            ]
        );
    }

    #[test]
    fn a_finished_run_forgets_its_pending_permissions_and_thinking() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        request_permission(&mut protocol);
        decode_all(
            &mut protocol,
            &[thinking_delta("sin usar"), result_line("success", false)],
        );

        let answer: AgentCommand = AgentCommand::AnswerDialog {
            dialog_id: "r1".into(),
            answer: DialogAnswer::Confirmed { confirmed: true },
        };
        assert!(protocol.encode(&answer, "x").is_err());
        let next: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[json!({"type": "assistant", "message": {"content": [
                {"type": "text", "text": "nuevo"}]}})],
        );
        assert_eq!(
            next[1],
            AgentEvent::AssistantMessage {
                text: "nuevo".into(),
                thinking: None,
                error: None
            }
        );
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
                    message: Some("echo hola > a.txt\nEscribe a.txt".into())
                },
                timeout_ms: None
            })]
        );
    }

    fn control_request(request_id: &str, tool: &str, input: Value) -> Value {
        json!({"type": "control_request", "request_id": request_id, "request": {
            "subtype": "can_use_tool", "tool_name": tool, "display_name": tool, "input": input}})
    }

    fn two_questions() -> Value {
        json!({"questions": [
            {"question": "¿Rojo o azul?", "header": "Color", "multiSelect": false, "options": [
                {"label": "Rojo", "description": "Elegir rojo"}, {"label": "Azul", "description": ""}]},
            {"question": "¿Cuántos?", "header": "", "multiSelect": false, "options": [
                {"label": "Uno", "description": ""}, {"label": "Dos", "description": ""}]}]})
    }

    fn answer(dialog_id: &str, answer: DialogAnswer) -> AgentCommand {
        AgentCommand::AnswerDialog {
            dialog_id: dialog_id.into(),
            answer,
        }
    }

    fn value(text: &str) -> DialogAnswer {
        DialogAnswer::Value { value: text.into() }
    }

    #[test]
    fn permission_dialog_shows_the_file_a_tool_will_touch() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[control_request(
                "r1",
                "Write",
                json!({"file_path": "/w/a.txt", "content": "hola"}),
            )],
        );
        assert_eq!(
            events,
            vec![AgentEvent::DialogRequested(DialogRequest {
                id: "r1".into(),
                title: "Permitir Write".into(),
                kind: DialogKind::Confirm {
                    message: Some("/w/a.txt".into())
                },
                timeout_ms: None
            })]
        );
    }

    #[test]
    fn an_invented_question_id_does_not_consume_a_tool_permission() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        request_permission(&mut protocol);

        for invented in ["r1#0", "r1#5", "r1#x"] {
            assert!(
                protocol
                    .encode(
                        &answer(invented, DialogAnswer::Confirmed { confirmed: true }),
                        "x"
                    )
                    .is_err(),
                "{invented}"
            );
        }
        let line: Option<String> = protocol
            .encode(
                &answer("r1", DialogAnswer::Confirmed { confirmed: true }),
                "x",
            )
            .expect("el permiso real sigue pendiente");
        assert_eq!(
            parse(&line.expect("línea"))["response"]["response"]["behavior"],
            "allow"
        );
    }

    #[test]
    fn an_empty_display_name_falls_back_to_the_tool_name() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "control_request", "request_id": "r1", "request": {
                "subtype": "can_use_tool", "tool_name": "Bash", "display_name": "",
                "input": {"command": "ls"}}}),
            ],
        );
        assert!(matches!(
            &events[0],
            AgentEvent::DialogRequested(DialogRequest { title, .. }) if title == "Permitir Bash"
        ));
    }

    #[test]
    fn a_long_command_is_shown_whole_before_the_models_description() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let command: String = format!("echo {}\nrm -rf ./build", "x".repeat(MAX_SUMMARY_CHARS * 2));
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "control_request", "request_id": "r1", "request": {
                "subtype": "can_use_tool", "tool_name": "Bash", "display_name": "Bash",
                "description": "Inofensivo", "input": {"command": command}}}),
            ],
        );
        let AgentEvent::DialogRequested(DialogRequest {
            kind: DialogKind::Confirm {
                message: Some(message),
            },
            ..
        }) = &events[0]
        else {
            panic!("se esperaba una confirmación");
        };
        assert_eq!(message, &format!("{command}\nInofensivo"));
    }

    #[test]
    fn ask_user_question_becomes_one_select_dialog_per_question() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[control_request("r1", "AskUserQuestion", two_questions())],
        );
        assert_eq!(
            events,
            vec![
                AgentEvent::DialogRequested(DialogRequest {
                    id: "r1#0".into(),
                    title: "Color: ¿Rojo o azul?".into(),
                    kind: DialogKind::Select {
                        options: vec!["Rojo — Elegir rojo".into(), "Azul".into()]
                    },
                    timeout_ms: None
                }),
                AgentEvent::DialogRequested(DialogRequest {
                    id: "r1#1".into(),
                    title: "¿Cuántos?".into(),
                    kind: DialogKind::Select {
                        options: vec!["Uno".into(), "Dos".into()]
                    },
                    timeout_ms: None
                }),
            ]
        );
    }

    #[test]
    fn answers_are_sent_once_every_question_has_one() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        decode_all(
            &mut protocol,
            &[control_request("r1", "AskUserQuestion", two_questions())],
        );

        let partial: Option<String> = protocol
            .encode(&answer("r1#1", value("Dos")), "x")
            .expect("respuesta parcial");
        assert_eq!(partial, None);
        let line: String = protocol
            .encode(&answer("r1#0", value("Rojo — Elegir rojo")), "x")
            .expect("respuesta final")
            .expect("línea");

        assert_eq!(
            parse(&line),
            json!({"type": "control_response", "response": {
                "subtype": "success", "request_id": "r1",
                "response": {"behavior": "allow", "updatedInput": {
                    "questions": two_questions()["questions"],
                    "answers": {"¿Rojo o azul?": "Rojo", "¿Cuántos?": "Dos"}}}}})
        );
    }

    #[test]
    fn a_free_text_answer_is_passed_through_unchanged() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        decode_all(
            &mut protocol,
            &[control_request(
                "r1",
                "AskUserQuestion",
                json!({"questions": [
                {"question": "¿Qué?", "options": [{"label": "A", "description": ""}]}]}),
            )],
        );
        let line: String = protocol
            .encode(&answer("r1#0", value("otra cosa")), "x")
            .expect("respuesta")
            .expect("línea");
        assert_eq!(
            parse(&line)["response"]["response"]["updatedInput"]["answers"],
            json!({"¿Qué?": "otra cosa"})
        );
    }

    #[test]
    fn cancelling_a_question_denies_the_request_and_ignores_its_siblings() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        decode_all(
            &mut protocol,
            &[control_request("r1", "AskUserQuestion", two_questions())],
        );

        let line: String = protocol
            .encode(&answer("r1#0", DialogAnswer::Cancelled), "x")
            .expect("cancelar")
            .expect("línea");
        assert_eq!(parse(&line)["response"]["response"]["behavior"], "deny");
        assert_eq!(
            protocol
                .encode(&answer("r1#1", value("Uno")), "x")
                .expect("hermana ignorada"),
            None
        );
    }

    #[test]
    fn exit_plan_mode_asks_to_approve_the_plan() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[control_request(
                "r1",
                "ExitPlanMode",
                json!({"plan": "# Plan\n1. Crear a.txt"}),
            )],
        );
        assert_eq!(
            events,
            vec![AgentEvent::DialogRequested(DialogRequest {
                id: "r1".into(),
                title: "Aprobar plan".into(),
                kind: DialogKind::Confirm {
                    message: Some("# Plan\n1. Crear a.txt".into())
                },
                timeout_ms: None
            })]
        );
        let line: String = protocol
            .encode(
                &answer("r1", DialogAnswer::Confirmed { confirmed: false }),
                "x",
            )
            .expect("rechazo")
            .expect("línea");
        assert_eq!(
            parse(&line)["response"]["response"],
            json!({"behavior": "deny", "message": DENY_PLAN_MESSAGE})
        );
    }

    #[test]
    fn a_malformed_question_request_falls_back_to_a_plain_confirmation() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[control_request(
                "r1",
                "AskUserQuestion",
                json!({"questions": []}),
            )],
        );
        assert!(matches!(
            &events[0],
            AgentEvent::DialogRequested(DialogRequest { id, kind: DialogKind::Confirm { .. }, .. })
                if id == "r1"
        ));
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
    fn rejected_rate_limit_followed_by_error_result_reports_the_usage_limit() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "rate_limit_event", "rate_limit_info": {
                    "status": "rejected", "resetsAt": 1784283600, "rateLimitType": "five_hour"}}),
                json!({"type": "result", "subtype": "success", "is_error": true,
                     "result": "You've hit your session limit · resets 3:45pm"}),
            ],
        );
        assert_eq!(
            events,
            vec![
                AgentEvent::UsageLimitReached {
                    resets_at: Some(1_784_283_600),
                    message: "You've hit your session limit · resets 3:45pm".to_string(),
                },
                AgentEvent::RunSettled,
            ]
        );
    }

    #[test]
    fn limit_text_alone_is_detected_without_a_reset_time() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "result", "subtype": "error", "is_error": true,
                     "result": "Claude AI usage limit reached"}),
            ],
        );
        assert!(matches!(
            events[0],
            AgentEvent::UsageLimitReached {
                resets_at: None,
                ..
            }
        ));
    }

    #[test]
    fn allowed_rate_limit_and_successful_results_do_not_report_a_limit() {
        let mut protocol: ClaudeProtocol = ClaudeProtocol::new();
        let events: Vec<AgentEvent> = decode_all(
            &mut protocol,
            &[
                json!({"type": "rate_limit_event", "rate_limit_info": {"status": "rejected", "resetsAt": 5}}),
                json!({"type": "rate_limit_event", "rate_limit_info": {"status": "allowed"}}),
                result_line("error_max_turns", true),
            ],
        );
        assert!(matches!(
            events[0],
            AgentEvent::Notice {
                level: NoticeLevel::Error,
                ..
            }
        ));
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
            .expect("prompt")
            .expect("línea");
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
            .expect("prompt")
            .expect("línea");
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
            .expect("abort")
            .expect("línea");
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
            .expect("answer")
            .expect("línea");
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
                .expect("answer")
                .expect("línea");
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
