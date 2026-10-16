//! Traducción entre ACP (Agent Client Protocol, JSON-RPC 2.0 por líneas) de `agent acp` y el
//! dominio.
//!
//! Un [`AcpProtocol`] por proceso. Es una máquina de estados: al arrancar envía solo
//! `initialize → authenticate → session/new` (o `session/load` al reanudar), retiene los
//! prompts hasta tener sesión (y modelo aplicado) y después los envía de uno en uno. Todo lo
//! que escribe en stdin lo deja en la cola de salida (`take_outgoing`).

use std::collections::{HashMap, VecDeque};

use serde_json::{Value, json};

use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::agent_command::AgentCommand;
use crate::modules::tasks::domain::models::agent_event::{
    AgentEvent, DialogAnswer, DialogKind, DialogRequest, NoticeLevel,
};
use crate::modules::tasks::domain::models::image_attachment::ImageAttachment;
use crate::modules::tasks::infrastructure::adapters::outbound::runtime::agent_process::LineCodec;

/// Límite de caracteres de salidas y argumentos de herramientas que se conservan.
const MAX_TOOL_TEXT_CHARS: usize = 20_000;
/// Límite del resumen de la entrada de una herramienta en el diálogo de permiso.
const MAX_SUMMARY_CHARS: usize = 500;
const PROTOCOL_VERSION: u64 = 1;
const AUTH_METHOD: &str = "cursor_login";
const INTERRUPTED_MESSAGE: &str = "ejecución interrumpida";
const METHOD_NOT_FOUND: i64 = -32601;
/// Los permisos de herramientas de edición se aceptan sin preguntar (política `acceptEdits`).
const AUTO_ALLOWED_KIND: &str = "edit";
const ALLOW_ONCE: &str = "allow_once";
const REJECT_ONCE: &str = "reject_once";
const DEFAULT_ALLOW_OPTION: &str = "allow-once";
const DEFAULT_REJECT_OPTION: &str = "reject-once";
const TODOS_WIDGET_KEY: &str = "todos";
/// Separa el id de la petición del índice de pregunta en los diálogos de `cursor/ask_question`.
const QUESTION_ID_SEPARATOR: char = '#';

/// Datos con los que se abre la sesión.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct AcpSessionConfig {
    /// Carpeta de trabajo de la sesión.
    pub cwd: String,
    /// Sesión previa a reanudar con `session/load`; `None` abre una nueva.
    pub resume_session: Option<String>,
    /// `modelId` de Cursor a aplicar con `session/set_model`.
    pub model: Option<String>,
}

/// Petición propia a la espera de respuesta.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Request {
    Initialize,
    Authenticate,
    NewSession,
    LoadSession,
    SetModel,
    Prompt,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum Phase {
    /// Handshake en curso: los prompts se retienen.
    Starting,
    Ready,
    Failed,
}

#[derive(Debug, Default)]
struct ToolState {
    kind: String,
    title: String,
    args: Value,
    started: bool,
}

#[derive(Debug)]
struct Todo {
    id: String,
    content: String,
    status: String,
}

#[derive(Debug)]
struct Question {
    id: String,
    /// `(etiqueta mostrada, id de opción)`.
    options: Vec<(String, String)>,
}

/// Petición del agente a la espera de la respuesta del usuario.
#[derive(Debug)]
enum PendingDialog {
    Permission {
        rpc_id: Value,
        allow_option: String,
        reject_option: String,
    },
    Plan {
        rpc_id: Value,
    },
    Questions {
        rpc_id: Value,
        questions: Vec<Question>,
        answers: Vec<Option<String>>,
    },
}

/// Aviso con el id de la sesión recién abierta.
type SessionListener = Box<dyn FnMut(&str) + Send>;

pub struct AcpProtocol {
    config: AcpSessionConfig,
    on_session: Option<SessionListener>,
    next_id: u64,
    requests: HashMap<u64, Request>,
    phase: Phase,
    session_id: Option<String>,
    queued: VecDeque<Vec<Value>>,
    prompt_in_flight: bool,
    outgoing: Vec<String>,
    running: bool,
    text: String,
    thinking: String,
    tools: HashMap<String, ToolState>,
    dialogs: HashMap<String, PendingDialog>,
    todos: Vec<Todo>,
}

impl AcpProtocol {
    pub fn new(config: AcpSessionConfig) -> Self {
        Self {
            config,
            on_session: None,
            next_id: 1,
            requests: HashMap::new(),
            phase: Phase::Starting,
            session_id: None,
            queued: VecDeque::new(),
            prompt_in_flight: false,
            outgoing: Vec::new(),
            running: false,
            text: String::new(),
            thinking: String::new(),
            tools: HashMap::new(),
            dialogs: HashMap::new(),
            todos: Vec::new(),
        }
    }

    /// Se invoca con el id de la sesión cada vez que queda abierta (nueva o reanudada).
    pub fn with_session_listener(mut self, listener: impl FnMut(&str) + Send + 'static) -> Self {
        self.on_session = Some(Box::new(listener));
        self
    }

    fn send_request(&mut self, kind: Request, method: &str, params: Value) {
        let id: u64 = self.next_id;
        self.next_id += 1;
        self.requests.insert(id, kind);
        self.outgoing.push(
            json!({ "jsonrpc": "2.0", "id": id, "method": method, "params": params }).to_string(),
        );
    }

    fn send_notification(&mut self, method: &str, params: Value) {
        self.outgoing
            .push(json!({ "jsonrpc": "2.0", "method": method, "params": params }).to_string());
    }

    fn reply(&mut self, id: &Value, result: Value) {
        self.outgoing
            .push(json!({ "jsonrpc": "2.0", "id": id, "result": result }).to_string());
    }

    fn reply_error(&mut self, id: &Value, code: i64, message: &str) {
        self.outgoing.push(
            json!({ "jsonrpc": "2.0", "id": id, "error": { "code": code, "message": message } })
                .to_string(),
        );
    }

    fn open_session(&mut self) {
        match self.config.resume_session.clone() {
            Some(session_id) => self.send_request(
                Request::LoadSession,
                "session/load",
                json!({ "sessionId": session_id, "cwd": self.config.cwd, "mcpServers": [] }),
            ),
            None => self.open_new_session(),
        }
    }

    fn open_new_session(&mut self) {
        self.send_request(
            Request::NewSession,
            "session/new",
            json!({ "cwd": self.config.cwd, "mcpServers": [] }),
        );
    }

    /// La sesión existe: aplica el modelo (si lo hay) y, cuando está listo, suelta los prompts.
    fn session_opened(&mut self, session_id: String) {
        if let Some(listener) = self.on_session.as_mut() {
            listener(&session_id);
        }
        self.session_id = Some(session_id.clone());
        match self.config.model.clone() {
            Some(model_id) => self.send_request(
                Request::SetModel,
                "session/set_model",
                json!({ "sessionId": session_id, "modelId": model_id }),
            ),
            None => self.become_ready(),
        }
    }

    fn become_ready(&mut self) {
        self.phase = Phase::Ready;
        self.pump();
    }

    /// Envía el siguiente prompt retenido si no hay otro en curso.
    fn pump(&mut self) {
        if self.phase != Phase::Ready || self.prompt_in_flight {
            return;
        }
        let (Some(prompt), Some(session_id)) = (self.queued.pop_front(), self.session_id.clone())
        else {
            return;
        };
        self.prompt_in_flight = true;
        self.send_request(
            Request::Prompt,
            "session/prompt",
            json!({ "sessionId": session_id, "prompt": prompt }),
        );
    }

    fn fail(&mut self, message: String) -> Vec<AgentEvent> {
        self.phase = Phase::Failed;
        let had_work: bool = self.prompt_in_flight || !self.queued.is_empty();
        self.queued.clear();
        self.prompt_in_flight = false;
        let mut events: Vec<AgentEvent> = vec![notice(NoticeLevel::Error, message)];
        if had_work || self.running {
            self.running = false;
            events.push(AgentEvent::RunSettled);
        }
        events
    }

    fn start_run(&mut self, events: &mut Vec<AgentEvent>) {
        if !self.running {
            self.running = true;
            events.push(AgentEvent::RunStarted);
        }
    }

    /// Convierte el texto y el razonamiento acumulados en un mensaje persistente.
    fn flush_message(&mut self, events: &mut Vec<AgentEvent>) {
        let text: String = std::mem::take(&mut self.text);
        let thinking: String = std::mem::take(&mut self.thinking);
        if text.is_empty() && thinking.is_empty() {
            return;
        }
        events.push(AgentEvent::AssistantMessage {
            text,
            thinking: (!thinking.is_empty()).then_some(thinking),
            error: None,
        });
    }

    // ---- salida de líneas del agente --------------------------------------------------

    fn decode_record(&mut self, record: &Value) -> Vec<AgentEvent> {
        let method: Option<&str> = str_field(record, "method");
        let id: Option<&Value> = record.get("id").filter(|v: &&Value| !v.is_null());
        match (method, id) {
            (Some(method), Some(id)) => self.decode_request(method, id, record),
            (Some(method), None) => self.decode_notification(method, record),
            (None, Some(id)) => self.decode_response(id, record),
            (None, None) => Vec::new(),
        }
    }

    fn decode_response(&mut self, id: &Value, record: &Value) -> Vec<AgentEvent> {
        let Some(kind) = id.as_u64().and_then(|n: u64| self.requests.remove(&n)) else {
            return Vec::new();
        };
        let error: Option<String> = record.get("error").map(error_message);
        let result: &Value = record.get("result").unwrap_or(&Value::Null);
        match kind {
            Request::Initialize => match error {
                Some(message) => self.fail(format!("no se pudo iniciar ACP: {message}")),
                None => {
                    self.send_request(
                        Request::Authenticate,
                        "authenticate",
                        json!({ "methodId": AUTH_METHOD }),
                    );
                    Vec::new()
                }
            },
            Request::Authenticate => match error {
                Some(message) => self.fail(format!(
                    "autenticación rechazada (ejecuta `agent login`): {message}"
                )),
                None => {
                    self.open_session();
                    Vec::new()
                }
            },
            Request::NewSession => match (error, str_field(result, "sessionId")) {
                (None, Some(session_id)) => {
                    self.session_opened(session_id.to_string());
                    Vec::new()
                }
                (Some(message), _) => self.fail(format!("no se pudo abrir la sesión: {message}")),
                (None, None) => self.fail("la sesión de ACP no devolvió sessionId".to_string()),
            },
            Request::LoadSession => match (error, self.config.resume_session.clone()) {
                (None, Some(session_id)) => {
                    self.session_opened(session_id);
                    Vec::new()
                }
                (error, _) => {
                    self.open_new_session();
                    vec![notice(
                        NoticeLevel::Warning,
                        format!(
                            "no se pudo reanudar la sesión anterior ({}); se abre una nueva",
                            error.unwrap_or_else(|| "sin sesión guardada".to_string())
                        ),
                    )]
                }
            },
            Request::SetModel => {
                let events: Vec<AgentEvent> = error
                    .map(|message: String| {
                        vec![notice(
                            NoticeLevel::Warning,
                            format!("no se pudo aplicar el modelo: {message}"),
                        )]
                    })
                    .unwrap_or_default();
                self.become_ready();
                events
            }
            Request::Prompt => self.settle(error, result),
        }
    }

    /// Fin de un turno: cierra el mensaje, resuelve el run y suelta el siguiente prompt.
    fn settle(&mut self, error: Option<String>, result: &Value) -> Vec<AgentEvent> {
        let mut events: Vec<AgentEvent> = Vec::new();
        self.start_run(&mut events);
        self.flush_message(&mut events);
        match (error, str_field(result, "stopReason")) {
            (Some(message), _) => events.push(notice(NoticeLevel::Error, message)),
            (None, Some("cancelled")) => {
                events.push(notice(
                    NoticeLevel::Warning,
                    INTERRUPTED_MESSAGE.to_string(),
                ));
            }
            (None, Some(reason)) if reason != "end_turn" => events.push(notice(
                NoticeLevel::Warning,
                format!("el agente terminó con motivo «{reason}»"),
            )),
            _ => {}
        }
        events.push(AgentEvent::RunSettled);
        self.running = false;
        self.prompt_in_flight = false;
        self.tools.clear();
        self.dialogs.clear();
        self.pump();
        events
    }

    fn decode_notification(&mut self, method: &str, record: &Value) -> Vec<AgentEvent> {
        let params: &Value = record.get("params").unwrap_or(&Value::Null);
        match method {
            "session/update" if self.phase == Phase::Ready => self.decode_update(params),
            "cursor/update_todos" => self.update_todos(params),
            _ => Vec::new(),
        }
    }

    fn decode_update(&mut self, params: &Value) -> Vec<AgentEvent> {
        let Some(update) = params.get("update") else {
            return Vec::new();
        };
        match str_field(update, "sessionUpdate").unwrap_or_default() {
            "agent_message_chunk" => {
                let delta: String = chunk_text(update);
                let mut events: Vec<AgentEvent> = Vec::new();
                self.start_run(&mut events);
                self.text.push_str(&delta);
                events.push(AgentEvent::TextDelta { delta });
                events
            }
            "agent_thought_chunk" => {
                let delta: String = chunk_text(update);
                let mut events: Vec<AgentEvent> = Vec::new();
                self.start_run(&mut events);
                self.thinking.push_str(&delta);
                events.push(AgentEvent::ThinkingDelta { delta });
                events
            }
            "tool_call" => self.tool_call(update),
            "tool_call_update" => self.tool_call_update(update),
            _ => Vec::new(),
        }
    }

    fn tool_call(&mut self, update: &Value) -> Vec<AgentEvent> {
        let call_id: String = string_of(update, "toolCallId");
        self.tools.insert(call_id.clone(), ToolState::default());
        self.tool_call_update_for(call_id, update)
    }

    fn tool_call_update(&mut self, update: &Value) -> Vec<AgentEvent> {
        let call_id: String = string_of(update, "toolCallId");
        self.tool_call_update_for(call_id, update)
    }

    /// Fusiona los datos de la herramienta; `ToolStarted` se emite cuando ya hay argumentos
    /// (la primera notificación suele traer `rawInput: {}`) o cuando la herramienta avanza.
    fn tool_call_update_for(&mut self, call_id: String, update: &Value) -> Vec<AgentEvent> {
        let mut events: Vec<AgentEvent> = Vec::new();
        self.start_run(&mut events);
        let state: &mut ToolState = self.tools.entry(call_id.clone()).or_default();
        if let Some(kind) = str_field(update, "kind") {
            state.kind = kind.to_string();
        }
        if let Some(title) = str_field(update, "title") {
            state.title = title.to_string();
        }
        if let Some(input) = update
            .get("rawInput")
            .filter(|v: &&Value| !is_empty_value(v))
        {
            state.args = input.clone();
        }
        let status: Option<&str> = str_field(update, "status");
        let finished: bool = matches!(status, Some("completed" | "failed"));
        let should_start: bool = !state.started
            && (!is_empty_value(&state.args) || finished || status == Some("in_progress"));
        if should_start {
            state.started = true;
            let started: AgentEvent = AgentEvent::ToolStarted {
                call_id: call_id.clone(),
                tool_name: tool_name(state),
                args: truncate(value_text(&state.args)),
            };
            self.flush_message(&mut events);
            events.push(started);
        }
        if finished && let Some(state) = self.tools.remove(&call_id) {
            events.push(AgentEvent::ToolFinished {
                call_id,
                tool_name: tool_name(&state),
                output: truncate(tool_output(update)),
                is_error: status == Some("failed") || exit_code_failed(update),
            });
        }
        events
    }

    fn update_todos(&mut self, params: &Value) -> Vec<AgentEvent> {
        let incoming: Vec<Todo> = params
            .get("todos")
            .and_then(Value::as_array)
            .map(|items: &Vec<Value>| {
                items
                    .iter()
                    .map(|item: &Value| Todo {
                        id: string_of(item, "id"),
                        content: string_of(item, "content"),
                        status: string_of(item, "status"),
                    })
                    .collect()
            })
            .unwrap_or_default();
        let merge: bool = params
            .get("merge")
            .and_then(Value::as_bool)
            .unwrap_or(false);
        if !merge {
            self.todos.clear();
        }
        for todo in incoming {
            match self.todos.iter_mut().find(|t: &&mut Todo| t.id == todo.id) {
                Some(existing) => *existing = todo,
                None => self.todos.push(todo),
            }
        }
        let lines: Option<Vec<String>> = (!self.todos.is_empty()).then(|| {
            self.todos
                .iter()
                .map(|t: &Todo| format!("{} {}", todo_marker(&t.status), t.content))
                .collect()
        });
        vec![AgentEvent::WidgetChanged {
            key: TODOS_WIDGET_KEY.to_string(),
            lines,
        }]
    }

    // ---- peticiones del agente ---------------------------------------------------------

    fn decode_request(&mut self, method: &str, id: &Value, record: &Value) -> Vec<AgentEvent> {
        let params: &Value = record.get("params").unwrap_or(&Value::Null);
        match method {
            "session/request_permission" => self.request_permission(id, params),
            "cursor/ask_question" => self.ask_question(id, params),
            "cursor/create_plan" => self.create_plan(id, params),
            "cursor/update_todos" => {
                let events: Vec<AgentEvent> = self.update_todos(params);
                let todos: Vec<Value> = self
                    .todos
                    .iter()
                    .map(|t: &Todo| json!({ "id": t.id, "content": t.content, "status": t.status }))
                    .collect();
                self.reply(
                    id,
                    json!({ "outcome": { "outcome": "accepted", "todos": todos } }),
                );
                events
            }
            "cursor/task" | "cursor/generate_image" => {
                self.reply(id, json!({ "outcome": { "outcome": "cancelled" } }));
                Vec::new()
            }
            _ => {
                self.reply_error(id, METHOD_NOT_FOUND, "método no soportado por el cliente");
                Vec::new()
            }
        }
    }

    fn request_permission(&mut self, id: &Value, params: &Value) -> Vec<AgentEvent> {
        let tool: &Value = params.get("toolCall").unwrap_or(&Value::Null);
        let options: &[Value] = params
            .get("options")
            .and_then(Value::as_array)
            .map_or(&[], Vec::as_slice);
        let allow_option: String = option_id(options, ALLOW_ONCE, DEFAULT_ALLOW_OPTION);
        let reject_option: String = option_id(options, REJECT_ONCE, DEFAULT_REJECT_OPTION);
        if str_field(tool, "kind") == Some(AUTO_ALLOWED_KIND) {
            self.reply(id, selected(&allow_option));
            return Vec::new();
        }
        let dialog_id: String = format!("perm-{}", id_key(id));
        self.dialogs.insert(
            dialog_id.clone(),
            PendingDialog::Permission {
                rpc_id: id.clone(),
                allow_option,
                reject_option,
            },
        );
        let title: String = str_field(tool, "title")
            .filter(|t: &&str| !t.is_empty())
            .unwrap_or("Permiso de herramienta")
            .to_string();
        vec![AgentEvent::DialogRequested(DialogRequest {
            id: dialog_id,
            title,
            kind: DialogKind::Confirm {
                message: permission_summary(tool),
            },
            timeout_ms: None,
        })]
    }

    fn create_plan(&mut self, id: &Value, params: &Value) -> Vec<AgentEvent> {
        let dialog_id: String = format!("plan-{}", id_key(id));
        self.dialogs.insert(
            dialog_id.clone(),
            PendingDialog::Plan { rpc_id: id.clone() },
        );
        let title: String = str_field(params, "name")
            .filter(|t: &&str| !t.is_empty())
            .unwrap_or("Plan")
            .to_string();
        vec![AgentEvent::DialogRequested(DialogRequest {
            id: dialog_id,
            title,
            kind: DialogKind::Confirm {
                message: str_field(params, "plan").map(str::to_string),
            },
            timeout_ms: None,
        })]
    }

    fn ask_question(&mut self, id: &Value, params: &Value) -> Vec<AgentEvent> {
        let questions: Vec<(String, Question)> = parse_questions(params);
        if questions.is_empty() {
            self.reply(id, json!({ "outcome": { "outcome": "skipped" } }));
            return Vec::new();
        }
        let base: String = format!("ask-{}", id_key(id));
        let events: Vec<AgentEvent> = questions
            .iter()
            .enumerate()
            .map(
                |(index, (prompt, question)): (usize, &(String, Question))| {
                    AgentEvent::DialogRequested(DialogRequest {
                        id: format!("{base}{QUESTION_ID_SEPARATOR}{index}"),
                        title: prompt.clone(),
                        kind: DialogKind::Select {
                            options: question.options.iter().map(|(l, _)| l.clone()).collect(),
                        },
                        timeout_ms: None,
                    })
                },
            )
            .collect();
        let questions: Vec<Question> = questions.into_iter().map(|(_, q)| q).collect();
        self.dialogs.insert(
            base,
            PendingDialog::Questions {
                rpc_id: id.clone(),
                answers: vec![None; questions.len()],
                questions,
            },
        );
        events
    }

    // ---- órdenes del usuario -----------------------------------------------------------

    fn answer_dialog(&mut self, dialog_id: &str, answer: &DialogAnswer) -> Result<(), TaskError> {
        let unknown =
            || TaskError::Runtime(format!("no hay un diálogo pendiente con id {dialog_id}"));
        let (key, index): (&str, Option<usize>) = match dialog_id.rsplit_once(QUESTION_ID_SEPARATOR)
        {
            Some((key, index)) => (key, index.parse::<usize>().ok()),
            None => (dialog_id, None),
        };
        if index.is_none() && key != dialog_id {
            return Err(unknown());
        }
        // Validate before removing: a rejected answer must leave the request pending, otherwise
        // the agent would wait forever for a reply nobody can send any more.
        match (self.dialogs.get(key), index) {
            (Some(PendingDialog::Permission { .. } | PendingDialog::Plan { .. }), None) => {}
            (Some(PendingDialog::Questions { questions, .. }), Some(index)) => {
                let question: &Question = questions.get(index).ok_or_else(unknown)?;
                if let DialogAnswer::Value { value } = answer
                    && !question
                        .options
                        .iter()
                        .any(|(label, id): &(String, String)| label == value || id == value)
                {
                    return Err(unknown());
                }
            }
            _ => return Err(unknown()),
        }
        let pending: PendingDialog = self.dialogs.remove(key).ok_or_else(unknown)?;
        match (pending, index) {
            (
                PendingDialog::Permission {
                    rpc_id,
                    allow_option,
                    reject_option,
                },
                None,
            ) => {
                let result: Value = match answer {
                    DialogAnswer::Confirmed { confirmed: true } => selected(&allow_option),
                    DialogAnswer::Confirmed { confirmed: false } => selected(&reject_option),
                    _ => cancelled(),
                };
                self.reply(&rpc_id, result);
            }
            (PendingDialog::Plan { rpc_id }, None) => {
                let outcome: Value = match answer {
                    DialogAnswer::Confirmed { confirmed: true } => {
                        json!({ "outcome": { "outcome": "accepted" } })
                    }
                    DialogAnswer::Confirmed { confirmed: false } => {
                        json!({ "outcome": { "outcome": "rejected" } })
                    }
                    _ => json!({ "outcome": { "outcome": "cancelled" } }),
                };
                self.reply(&rpc_id, outcome);
            }
            (
                PendingDialog::Questions {
                    rpc_id,
                    questions,
                    mut answers,
                },
                Some(index),
            ) => {
                let Some(question) = questions.get(index) else {
                    return Err(unknown());
                };
                let DialogAnswer::Value { value } = answer else {
                    self.reply(&rpc_id, json!({ "outcome": { "outcome": "cancelled" } }));
                    return Ok(());
                };
                let option_id: String = question
                    .options
                    .iter()
                    .find(|(label, id): &&(String, String)| label == value || id == value)
                    .map(|(_, id): &(String, String)| id.clone())
                    .ok_or_else(unknown)?;
                answers[index] = Some(option_id);
                if answers.iter().all(Option::is_some) {
                    let answered: Vec<Value> = questions
                        .iter()
                        .zip(&answers)
                        .map(|(q, a): (&Question, &Option<String>)| {
                            json!({ "questionId": q.id, "selectedOptionIds": a.iter().collect::<Vec<_>>() })
                        })
                        .collect();
                    self.reply(
                        &rpc_id,
                        json!({ "outcome": { "outcome": "answered", "answers": answered } }),
                    );
                } else {
                    self.dialogs.insert(
                        key.to_string(),
                        PendingDialog::Questions {
                            rpc_id,
                            questions,
                            answers,
                        },
                    );
                }
            }
            _ => return Err(unknown()),
        }
        Ok(())
    }

    /// Cancela el turno y responde «cancelado» a las peticiones que esperan al usuario.
    fn abort(&mut self) {
        self.queued.clear();
        let pending: Vec<PendingDialog> = self.dialogs.drain().map(|(_, d)| d).collect();
        for dialog in pending {
            match dialog {
                PendingDialog::Permission { rpc_id, .. } => self.reply(&rpc_id, cancelled()),
                PendingDialog::Plan { rpc_id } | PendingDialog::Questions { rpc_id, .. } => {
                    self.reply(&rpc_id, json!({ "outcome": { "outcome": "cancelled" } }));
                }
            }
        }
        if let Some(session_id) = self.session_id.clone()
            && self.prompt_in_flight
        {
            self.send_notification("session/cancel", json!({ "sessionId": session_id }));
        }
    }
}

impl LineCodec for AcpProtocol {
    fn decode(
        &mut self,
        _task_id: crate::modules::tasks::domain::models::task::TaskId,
        line: &str,
    ) -> Vec<AgentEvent> {
        match serde_json::from_str::<Value>(line) {
            Ok(record) => self.decode_record(&record),
            Err(_) => Vec::new(),
        }
    }

    fn encode(
        &mut self,
        command: &AgentCommand,
        _request_id: &str,
    ) -> Result<Option<String>, TaskError> {
        match command {
            AgentCommand::Prompt { message, images } => {
                if self.phase == Phase::Failed {
                    return Err(TaskError::Runtime(
                        "el agente de Cursor no está disponible".to_string(),
                    ));
                }
                self.queued.push_back(prompt_blocks(message, images));
                self.pump();
            }
            AgentCommand::Steer { .. } | AgentCommand::FollowUp { .. } => {
                return Err(TaskError::Unsupported(
                    "Cursor no admite mensajes durante la ejecución".to_string(),
                ));
            }
            AgentCommand::Abort => self.abort(),
            AgentCommand::AnswerDialog { dialog_id, answer } => {
                self.answer_dialog(dialog_id, answer)?;
            }
        }
        Ok(None)
    }

    fn start(&mut self) -> Vec<String> {
        self.send_request(
            Request::Initialize,
            "initialize",
            json!({
                "protocolVersion": PROTOCOL_VERSION,
                "clientCapabilities": {
                    "fs": { "readTextFile": false, "writeTextFile": false },
                    "terminal": false,
                },
                "clientInfo": { "name": "iworkflow", "version": env!("CARGO_PKG_VERSION") },
            }),
        );
        self.take_outgoing()
    }

    fn take_outgoing(&mut self) -> Vec<String> {
        std::mem::take(&mut self.outgoing)
    }
}

// ---- utilidades ------------------------------------------------------------------------

fn prompt_blocks(message: &str, images: &[ImageAttachment]) -> Vec<Value> {
    std::iter::once(json!({ "type": "text", "text": message }))
        .chain(images.iter().map(|image: &ImageAttachment| {
            json!({ "type": "image", "mimeType": image.mime_type, "data": image.data })
        }))
        .collect()
}

fn selected(option_id: &str) -> Value {
    json!({ "outcome": { "outcome": "selected", "optionId": option_id } })
}

fn cancelled() -> Value {
    json!({ "outcome": { "outcome": "cancelled" } })
}

/// Id de la primera opción de la clase `kind`, o `fallback` si el agente no la ofrece.
fn option_id(options: &[Value], kind: &str, fallback: &str) -> String {
    options
        .iter()
        .find(|o: &&Value| str_field(o, "kind") == Some(kind))
        .and_then(|o: &Value| str_field(o, "optionId"))
        .unwrap_or(fallback)
        .to_string()
}

fn parse_questions(params: &Value) -> Vec<(String, Question)> {
    params
        .get("questions")
        .and_then(Value::as_array)
        .map(|items: &Vec<Value>| {
            items
                .iter()
                .filter_map(|item: &Value| {
                    let options: Vec<(String, String)> = item
                        .get("options")?
                        .as_array()?
                        .iter()
                        .map(|o: &Value| (string_of(o, "label"), string_of(o, "id")))
                        .collect();
                    (!options.is_empty()).then(|| {
                        (
                            string_of(item, "prompt"),
                            Question {
                                id: string_of(item, "id"),
                                options,
                            },
                        )
                    })
                })
                .collect()
        })
        .unwrap_or_default()
}

/// Lo que se muestra al pedir permiso: el comando o fichero real antes que el título.
fn permission_summary(tool: &Value) -> Option<String> {
    let input: &Value = tool.get("rawInput").unwrap_or(&Value::Null);
    let detail: Option<String> = str_field(input, "command")
        .or_else(|| str_field(input, "path"))
        .map(str::to_string)
        .or_else(|| (!is_empty_value(input)).then(|| value_text(input)));
    detail
        .or_else(|| str_field(tool, "title").map(str::to_string))
        .map(|text: String| truncate_chars(text, MAX_SUMMARY_CHARS))
}

fn tool_name(state: &ToolState) -> String {
    if state.kind.is_empty() || state.kind == "other" {
        state.title.clone()
    } else {
        state.kind.clone()
    }
}

/// Salida de una herramienta terminada: `rawOutput` (stdout/stderr) o el contenido devuelto.
fn tool_output(update: &Value) -> String {
    if let Some(raw) = update.get("rawOutput").filter(|v: &&Value| !v.is_null()) {
        if let Some(stdout) = str_field(raw, "stdout") {
            let stderr: &str = str_field(raw, "stderr").unwrap_or_default();
            return if stderr.is_empty() {
                stdout.to_string()
            } else {
                format!("{stdout}\n{stderr}")
            };
        }
        return value_text(raw);
    }
    update
        .get("content")
        .and_then(Value::as_array)
        .map(|items: &Vec<Value>| {
            items
                .iter()
                .filter_map(|item: &Value| match str_field(item, "type") {
                    Some("diff") => Some(format!(
                        "{}\n{}",
                        string_of(item, "path"),
                        string_of(item, "newText")
                    )),
                    Some("content") => item
                        .get("content")
                        .and_then(|c: &Value| str_field(c, "text"))
                        .map(str::to_string),
                    _ => None,
                })
                .collect::<Vec<String>>()
                .join("\n")
        })
        .unwrap_or_default()
}

fn exit_code_failed(update: &Value) -> bool {
    update
        .get("rawOutput")
        .and_then(|raw: &Value| raw.get("exitCode"))
        .and_then(Value::as_i64)
        .is_some_and(|code: i64| code != 0)
}

fn todo_marker(status: &str) -> &'static str {
    match status {
        "completed" => "[x]",
        "in_progress" => "[~]",
        "cancelled" => "[-]",
        _ => "[ ]",
    }
}

fn error_message(error: &Value) -> String {
    str_field(error, "message")
        .map(str::to_string)
        .unwrap_or_else(|| error.to_string())
}

/// Id JSON-RPC como texto estable para componer ids de diálogo.
fn id_key(id: &Value) -> String {
    match id {
        Value::String(s) => s.clone(),
        other => other.to_string(),
    }
}

fn is_empty_value(value: &Value) -> bool {
    match value {
        Value::Null => true,
        Value::Object(map) => map.is_empty(),
        Value::String(s) => s.is_empty(),
        Value::Array(items) => items.is_empty(),
        _ => false,
    }
}

fn value_text(value: &Value) -> String {
    match value {
        Value::Null => String::new(),
        Value::String(s) => s.clone(),
        other => other.to_string(),
    }
}

fn chunk_text(update: &Value) -> String {
    update
        .get("content")
        .map(|c: &Value| string_of(c, "text"))
        .unwrap_or_default()
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
    use crate::modules::tasks::domain::models::task::TaskId;

    fn protocol(config: AcpSessionConfig) -> AcpProtocol {
        AcpProtocol::new(AcpSessionConfig {
            cwd: "/work".to_string(),
            ..config
        })
    }

    fn feed(protocol: &mut AcpProtocol, line: Value) -> (Vec<AgentEvent>, Vec<Value>) {
        let events: Vec<AgentEvent> = protocol.decode(TaskId::new(), &line.to_string());
        (events, sent(protocol))
    }

    fn sent(protocol: &mut AcpProtocol) -> Vec<Value> {
        protocol
            .take_outgoing()
            .iter()
            .map(|l: &String| serde_json::from_str(l).expect("json"))
            .collect()
    }

    fn respond(id: u64, result: Value) -> Value {
        json!({ "jsonrpc": "2.0", "id": id, "result": result })
    }

    fn prompt(message: &str) -> AgentCommand {
        AgentCommand::Prompt {
            message: message.to_string(),
            images: Vec::new(),
        }
    }

    fn command(protocol: &mut AcpProtocol, command: AgentCommand) -> Vec<Value> {
        protocol.encode(&command, "r").expect("encode");
        sent(protocol)
    }

    /// Protocolo con la sesión `s1` abierta (ids 1-3 consumidos).
    fn ready(config: AcpSessionConfig) -> AcpProtocol {
        let mut protocol: AcpProtocol = protocol(config);
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        feed(&mut protocol, respond(2, json!({})));
        feed(&mut protocol, respond(3, json!({ "sessionId": "s1" })));
        sent(&mut protocol);
        protocol
    }

    fn update(update: Value) -> Value {
        json!({ "jsonrpc": "2.0", "method": "session/update",
            "params": { "sessionId": "s1", "update": update } })
    }

    fn dialog_ids(events: &[AgentEvent]) -> Vec<String> {
        events
            .iter()
            .filter_map(|e: &AgentEvent| match e {
                AgentEvent::DialogRequested(d) => Some(d.id.clone()),
                _ => None,
            })
            .collect()
    }

    #[test]
    fn start_sends_only_initialize() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig::default());

        let lines: Vec<String> = protocol.start();

        assert_eq!(lines.len(), 1);
        let first: Value = serde_json::from_str(&lines[0]).expect("json");
        assert_eq!(first["method"], "initialize");
        assert_eq!(first["id"], 1);
        assert_eq!(first["params"]["protocolVersion"], 1);
    }

    #[test]
    fn handshake_walks_initialize_authenticate_and_session_new() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig::default());
        protocol.start();

        let (_, after_init) = feed(&mut protocol, respond(1, json!({ "authMethods": [] })));
        let (_, after_auth) = feed(&mut protocol, respond(2, json!({})));

        assert_eq!(after_init[0]["method"], "authenticate");
        assert_eq!(after_init[0]["params"]["methodId"], "cursor_login");
        assert_eq!(after_auth[0]["method"], "session/new");
        assert_eq!(after_auth[0]["params"]["cwd"], "/work");
        assert_eq!(after_auth[0]["params"]["mcpServers"], json!([]));
    }

    #[test]
    fn a_prompt_is_held_until_session_new_answers() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig::default());
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        feed(&mut protocol, respond(2, json!({})));

        let early: Vec<Value> = command(&mut protocol, prompt("hola"));
        let (_, released) = feed(&mut protocol, respond(3, json!({ "sessionId": "s1" })));

        assert!(early.is_empty());
        assert_eq!(released.len(), 1);
        assert_eq!(released[0]["method"], "session/prompt");
        assert_eq!(released[0]["params"]["sessionId"], "s1");
        assert_eq!(
            released[0]["params"]["prompt"],
            json!([{ "type": "text", "text": "hola" }])
        );
    }

    #[test]
    fn the_model_is_applied_before_the_held_prompt_is_released() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig {
            model: Some("gpt-5.5[fast=false]".to_string()),
            ..AcpSessionConfig::default()
        });
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        feed(&mut protocol, respond(2, json!({})));
        command(&mut protocol, prompt("hola"));

        let (_, after_session) = feed(&mut protocol, respond(3, json!({ "sessionId": "s1" })));
        let (_, after_model) = feed(&mut protocol, respond(4, json!({})));

        assert_eq!(after_session.len(), 1);
        assert_eq!(after_session[0]["method"], "session/set_model");
        assert_eq!(after_session[0]["params"]["modelId"], "gpt-5.5[fast=false]");
        assert_eq!(after_model[0]["method"], "session/prompt");
    }

    #[test]
    fn a_failed_model_switch_warns_and_still_releases_the_prompt() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig {
            model: Some("x".to_string()),
            ..AcpSessionConfig::default()
        });
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        feed(&mut protocol, respond(2, json!({})));
        feed(&mut protocol, respond(3, json!({ "sessionId": "s1" })));
        command(&mut protocol, prompt("hola"));

        let (events, out) = feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 4, "error": { "code": -1, "message": "no model" } }),
        );

        assert!(matches!(
            &events[0],
            AgentEvent::Notice { level: NoticeLevel::Warning, message } if message.contains("no model")
        ));
        assert_eq!(out[0]["method"], "session/prompt");
    }

    #[test]
    fn resume_loads_the_saved_session_and_drops_its_replayed_history() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig {
            resume_session: Some("old".to_string()),
            ..AcpSessionConfig::default()
        });
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        let (_, load) = feed(&mut protocol, respond(2, json!({})));
        let (replayed, _) = feed(
            &mut protocol,
            update(json!({ "sessionUpdate": "agent_message_chunk",
                "content": { "type": "text", "text": "historia" } })),
        );
        let (_, after_load) = feed(&mut protocol, respond(3, json!({})));
        command(&mut protocol, prompt("sigue"));

        assert_eq!(load[0]["method"], "session/load");
        assert_eq!(load[0]["params"]["sessionId"], "old");
        assert!(replayed.is_empty());
        assert!(after_load.is_empty());
        assert_eq!(protocol.session_id.as_deref(), Some("old"));
    }

    #[test]
    fn a_failed_load_falls_back_to_a_new_session_with_a_warning() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig {
            resume_session: Some("old".to_string()),
            ..AcpSessionConfig::default()
        });
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        feed(&mut protocol, respond(2, json!({})));

        let (events, out) = feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 3, "error": { "code": -1, "message": "gone" } }),
        );

        assert!(matches!(
            &events[0],
            AgentEvent::Notice { level: NoticeLevel::Warning, message } if message.contains("gone")
        ));
        assert_eq!(out[0]["method"], "session/new");
    }

    #[test]
    fn the_session_listener_receives_the_new_session_id() {
        let seen: std::sync::Arc<std::sync::Mutex<Vec<String>>> = Default::default();
        let sink: std::sync::Arc<std::sync::Mutex<Vec<String>>> = seen.clone();
        let mut protocol: AcpProtocol = AcpProtocol::new(AcpSessionConfig::default())
            .with_session_listener(move |id: &str| {
                sink.lock().expect("lock").push(id.to_string());
            });
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        feed(&mut protocol, respond(2, json!({})));
        feed(&mut protocol, respond(3, json!({ "sessionId": "s9" })));

        assert_eq!(*seen.lock().expect("lock"), vec!["s9".to_string()]);
    }

    #[test]
    fn rejected_authentication_reports_an_error_and_drops_the_prompt() {
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig::default());
        protocol.start();
        feed(&mut protocol, respond(1, json!({})));
        command(&mut protocol, prompt("hola"));

        let (events, out) = feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 2, "error": { "code": -32000, "message": "login" } }),
        );

        assert!(matches!(
            &events[0],
            AgentEvent::Notice { level: NoticeLevel::Error, message } if message.contains("agent login")
        ));
        assert_eq!(events[1], AgentEvent::RunSettled);
        assert!(out.is_empty());
        assert!(protocol.encode(&prompt("otra"), "r").is_err());
    }

    #[test]
    fn streamed_text_and_thinking_become_one_message_when_the_turn_settles() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        command(&mut protocol, prompt("hola"));

        let (thought, _) = feed(
            &mut protocol,
            update(json!({ "sessionUpdate": "agent_thought_chunk",
                "content": { "type": "text", "text": "pienso" } })),
        );
        let (chunk, _) = feed(
            &mut protocol,
            update(json!({ "sessionUpdate": "agent_message_chunk",
                "content": { "type": "text", "text": "hola" } })),
        );
        let (settled, out) = feed(
            &mut protocol,
            respond(4, json!({ "stopReason": "end_turn" })),
        );

        assert_eq!(
            thought,
            vec![
                AgentEvent::RunStarted,
                AgentEvent::ThinkingDelta {
                    delta: "pienso".into()
                }
            ]
        );
        assert_eq!(
            chunk,
            vec![AgentEvent::TextDelta {
                delta: "hola".into()
            }]
        );
        assert_eq!(
            settled,
            vec![
                AgentEvent::AssistantMessage {
                    text: "hola".into(),
                    thinking: Some("pienso".into()),
                    error: None
                },
                AgentEvent::RunSettled
            ]
        );
        assert!(out.is_empty());
    }

    #[test]
    fn a_cancelled_turn_warns_and_a_queued_prompt_is_sent_after_the_current_one() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        command(&mut protocol, prompt("uno"));
        let second: Vec<Value> = command(&mut protocol, prompt("dos"));

        let (events, out) = feed(
            &mut protocol,
            respond(4, json!({ "stopReason": "cancelled" })),
        );

        assert!(second.is_empty());
        assert!(events.contains(&AgentEvent::Notice {
            level: NoticeLevel::Warning,
            message: INTERRUPTED_MESSAGE.to_string()
        }));
        assert_eq!(events.last(), Some(&AgentEvent::RunSettled));
        assert_eq!(out[0]["params"]["prompt"][0]["text"], "dos");
    }

    #[test]
    fn a_failed_prompt_reports_the_error_and_settles() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        command(&mut protocol, prompt("hola"));

        let (events, _) = feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 4, "error": { "code": -1, "message": "boom" } }),
        );

        assert!(events.contains(&AgentEvent::Notice {
            level: NoticeLevel::Error,
            message: "boom".to_string()
        }));
        assert_eq!(events.last(), Some(&AgentEvent::RunSettled));
    }

    #[test]
    fn images_are_sent_as_acp_image_blocks() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());

        let out: Vec<Value> = command(
            &mut protocol,
            AgentCommand::Prompt {
                message: "mira".into(),
                images: vec![ImageAttachment {
                    name: "a.png".into(),
                    mime_type: "image/png".into(),
                    data: "QUJD".into(),
                }],
            },
        );

        assert_eq!(
            out[0]["params"]["prompt"][1],
            json!({ "type": "image", "mimeType": "image/png", "data": "QUJD" })
        );
    }

    #[test]
    fn steer_and_follow_up_are_unsupported() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());

        for command in [
            AgentCommand::Steer {
                message: "x".into(),
                images: Vec::new(),
            },
            AgentCommand::FollowUp {
                message: "x".into(),
                images: Vec::new(),
            },
        ] {
            assert!(matches!(
                protocol.encode(&command, "r"),
                Err(TaskError::Unsupported(_))
            ));
        }
    }

    #[test]
    fn a_tool_is_started_once_its_arguments_are_known_and_finished_with_its_output() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        command(&mut protocol, prompt("hola"));
        feed(
            &mut protocol,
            update(json!({ "sessionUpdate": "agent_message_chunk",
                "content": { "type": "text", "text": "voy" } })),
        );

        let (created, _) = feed(
            &mut protocol,
            update(json!({ "sessionUpdate": "tool_call", "toolCallId": "c1",
                "title": "Edit File", "kind": "edit", "status": "pending", "rawInput": {} })),
        );
        let (known, _) = feed(
            &mut protocol,
            update(
                json!({ "sessionUpdate": "tool_call_update", "toolCallId": "c1",
                "title": "Edit `a.txt`", "rawInput": { "path": "a.txt" } }),
            ),
        );
        let (running, _) = feed(
            &mut protocol,
            update(
                json!({ "sessionUpdate": "tool_call_update", "toolCallId": "c1",
                "status": "in_progress" }),
            ),
        );
        let (done, _) = feed(
            &mut protocol,
            update(
                json!({ "sessionUpdate": "tool_call_update", "toolCallId": "c1",
                "status": "completed",
                "content": [{ "type": "diff", "path": "a.txt", "oldText": "", "newText": "hi" }] }),
            ),
        );

        assert!(created.is_empty());
        assert_eq!(
            known,
            vec![
                AgentEvent::AssistantMessage {
                    text: "voy".into(),
                    thinking: None,
                    error: None
                },
                AgentEvent::ToolStarted {
                    call_id: "c1".into(),
                    tool_name: "edit".into(),
                    args: r#"{"path":"a.txt"}"#.into()
                }
            ]
        );
        assert!(running.is_empty());
        assert_eq!(
            done,
            vec![AgentEvent::ToolFinished {
                call_id: "c1".into(),
                tool_name: "edit".into(),
                output: "a.txt\nhi".into(),
                is_error: false
            }]
        );
    }

    #[test]
    fn a_tool_created_with_arguments_starts_immediately_and_shell_output_is_joined() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        command(&mut protocol, prompt("hola"));

        let (created, _) = feed(
            &mut protocol,
            update(json!({ "sessionUpdate": "tool_call", "toolCallId": "c2",
                "title": "`ls`", "kind": "execute", "status": "pending",
                "rawInput": { "command": "ls" } })),
        );
        let (done, _) = feed(
            &mut protocol,
            update(
                json!({ "sessionUpdate": "tool_call_update", "toolCallId": "c2",
                "status": "completed",
                "rawOutput": { "exitCode": 2, "stdout": "out", "stderr": "err" } }),
            ),
        );

        assert!(matches!(
            &created[1],
            AgentEvent::ToolStarted { tool_name, args, .. }
                if tool_name == "execute" && args == r#"{"command":"ls"}"#
        ));
        assert_eq!(
            done,
            vec![AgentEvent::ToolFinished {
                call_id: "c2".into(),
                tool_name: "execute".into(),
                output: "out\nerr".into(),
                is_error: true
            }]
        );
    }

    fn permission(id: u64, kind: &str, input: Value) -> Value {
        json!({ "jsonrpc": "2.0", "id": id, "method": "session/request_permission",
            "params": { "sessionId": "s1",
                "toolCall": { "toolCallId": "c", "title": "Run it", "kind": kind, "rawInput": input },
                "options": [
                    { "optionId": "allow-once", "name": "Allow", "kind": "allow_once" },
                    { "optionId": "reject-once", "name": "Reject", "kind": "reject_once" }
                ] } })
    }

    #[test]
    fn edit_permissions_are_accepted_without_asking() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());

        let (events, out) = feed(&mut protocol, permission(7, "edit", json!({ "path": "a" })));

        assert!(events.is_empty());
        assert_eq!(
            out,
            vec![json!({ "jsonrpc": "2.0", "id": 7,
                "result": { "outcome": { "outcome": "selected", "optionId": "allow-once" } } })]
        );
    }

    #[test]
    fn other_permissions_ask_with_the_real_command_and_follow_the_answer() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());

        let (events, out) = feed(
            &mut protocol,
            permission(8, "execute", json!({ "command": "rm -rf build" })),
        );
        let allowed: Vec<Value> = command(
            &mut protocol,
            AgentCommand::AnswerDialog {
                dialog_id: "perm-8".into(),
                answer: DialogAnswer::Confirmed { confirmed: true },
            },
        );
        let (_, _) = feed(
            &mut protocol,
            permission(9, "execute", json!({ "command": "x" })),
        );
        let rejected: Vec<Value> = command(
            &mut protocol,
            AgentCommand::AnswerDialog {
                dialog_id: "perm-9".into(),
                answer: DialogAnswer::Confirmed { confirmed: false },
            },
        );

        assert!(out.is_empty());
        assert_eq!(
            events,
            vec![AgentEvent::DialogRequested(DialogRequest {
                id: "perm-8".into(),
                title: "Run it".into(),
                kind: DialogKind::Confirm {
                    message: Some("rm -rf build".into())
                },
                timeout_ms: None
            })]
        );
        assert_eq!(allowed[0]["result"]["outcome"]["optionId"], "allow-once");
        assert_eq!(rejected[0]["result"]["outcome"]["optionId"], "reject-once");
    }

    #[test]
    fn answering_an_unknown_dialog_is_an_error() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());

        let result = protocol.encode(
            &AgentCommand::AnswerDialog {
                dialog_id: "perm-1".into(),
                answer: DialogAnswer::Cancelled,
            },
            "r",
        );

        assert!(result.is_err());
    }

    #[test]
    fn abort_cancels_the_turn_and_every_pending_request() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        command(&mut protocol, prompt("hola"));
        feed(
            &mut protocol,
            permission(8, "execute", json!({ "command": "ls" })),
        );

        let out: Vec<Value> = command(&mut protocol, AgentCommand::Abort);

        assert_eq!(
            out[0],
            json!({ "jsonrpc": "2.0", "id": 8, "result": { "outcome": { "outcome": "cancelled" } } })
        );
        assert_eq!(out[1]["method"], "session/cancel");
        assert!(out[1].get("id").is_none());
        assert_eq!(out[1]["params"]["sessionId"], "s1");
    }

    #[test]
    fn ask_question_becomes_select_dialogs_answered_together() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        let (events, out) = feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": "q", "method": "cursor/ask_question", "params": {
                "toolCallId": "t",
                "questions": [
                    { "id": "q1", "prompt": "Modo", "options": [
                        { "id": "a", "label": "Agent" }, { "id": "p", "label": "Plan" }] },
                    { "id": "q2", "prompt": "Rama", "options": [{ "id": "m", "label": "main" }] }
                ] } }),
        );
        let first: Vec<Value> = command(
            &mut protocol,
            AgentCommand::AnswerDialog {
                dialog_id: "ask-q#1".into(),
                answer: DialogAnswer::Value {
                    value: "main".into(),
                },
            },
        );
        let last: Vec<Value> = command(
            &mut protocol,
            AgentCommand::AnswerDialog {
                dialog_id: "ask-q#0".into(),
                answer: DialogAnswer::Value {
                    value: "Plan".into(),
                },
            },
        );

        assert!(out.is_empty());
        assert_eq!(dialog_ids(&events), vec!["ask-q#0", "ask-q#1"]);
        assert!(matches!(
            &events[0],
            AgentEvent::DialogRequested(DialogRequest { title, kind: DialogKind::Select { options }, .. })
                if title == "Modo" && options == &vec!["Agent".to_string(), "Plan".to_string()]
        ));
        assert!(first.is_empty());
        assert_eq!(
            last[0]["result"]["outcome"],
            json!({ "outcome": "answered", "answers": [
                { "questionId": "q1", "selectedOptionIds": ["p"] },
                { "questionId": "q2", "selectedOptionIds": ["m"] }
            ] })
        );
    }

    #[test]
    fn an_invalid_answer_keeps_the_question_pending_for_a_valid_one() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 5, "method": "cursor/ask_question", "params": {
                "questions": [{ "id": "q1", "prompt": "?", "options": [{ "id": "a", "label": "A" }] }] } }),
        );

        let rejected = protocol.encode(
            &AgentCommand::AnswerDialog {
                dialog_id: "ask-5#0".into(),
                answer: DialogAnswer::Value {
                    value: "no existe".into(),
                },
            },
            "r",
        );
        assert!(rejected.is_err());
        assert!(sent(&mut protocol).is_empty());

        let out: Vec<Value> = command(
            &mut protocol,
            AgentCommand::AnswerDialog {
                dialog_id: "ask-5#0".into(),
                answer: DialogAnswer::Value { value: "A".into() },
            },
        );
        assert_eq!(out[0]["result"]["outcome"]["outcome"], "answered");
    }

    #[test]
    fn cancelling_a_question_cancels_the_whole_request() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 5, "method": "cursor/ask_question", "params": {
                "questions": [{ "id": "q1", "prompt": "?", "options": [{ "id": "a", "label": "A" }] }] } }),
        );

        let out: Vec<Value> = command(
            &mut protocol,
            AgentCommand::AnswerDialog {
                dialog_id: "ask-5#0".into(),
                answer: DialogAnswer::Cancelled,
            },
        );

        assert_eq!(out[0]["result"]["outcome"]["outcome"], "cancelled");
    }

    #[test]
    fn create_plan_asks_for_approval_with_the_plan_text() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        let (events, _) = feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 6, "method": "cursor/create_plan",
                "params": { "name": "Refactor", "plan": "1. hacer", "todos": [] } }),
        );
        let accepted: Vec<Value> = command(
            &mut protocol,
            AgentCommand::AnswerDialog {
                dialog_id: "plan-6".into(),
                answer: DialogAnswer::Confirmed { confirmed: true },
            },
        );

        assert!(matches!(
            &events[0],
            AgentEvent::DialogRequested(DialogRequest { title, kind: DialogKind::Confirm { message }, .. })
                if title == "Refactor" && message.as_deref() == Some("1. hacer")
        ));
        assert_eq!(accepted[0]["result"]["outcome"]["outcome"], "accepted");
    }

    #[test]
    fn todo_updates_replace_or_merge_into_a_widget() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());
        let todos = |items: Value, merge: bool| {
            json!({ "jsonrpc": "2.0", "method": "cursor/update_todos",
                "params": { "toolCallId": "t", "todos": items, "merge": merge } })
        };

        let (first, _) = feed(
            &mut protocol,
            todos(
                json!([{ "id": "1", "content": "uno", "status": "pending" },
                       { "id": "2", "content": "dos", "status": "pending" }]),
                false,
            ),
        );
        let (second, _) = feed(
            &mut protocol,
            todos(
                json!([{ "id": "1", "content": "uno", "status": "completed" }]),
                true,
            ),
        );

        assert_eq!(
            first,
            vec![AgentEvent::WidgetChanged {
                key: "todos".into(),
                lines: Some(vec!["[ ] uno".into(), "[ ] dos".into()])
            }]
        );
        assert_eq!(
            second,
            vec![AgentEvent::WidgetChanged {
                key: "todos".into(),
                lines: Some(vec!["[x] uno".into(), "[ ] dos".into()])
            }]
        );
    }

    #[test]
    fn unknown_agent_requests_get_method_not_found_and_noise_is_ignored() {
        let mut protocol: AcpProtocol = ready(AcpSessionConfig::default());

        let (events, out) = feed(
            &mut protocol,
            json!({ "jsonrpc": "2.0", "id": 3, "method": "fs/read_text_file", "params": {} }),
        );
        let garbage: Vec<AgentEvent> = protocol.decode(TaskId::new(), "no es json");

        assert!(events.is_empty());
        assert_eq!(out[0]["error"]["code"], -32601);
        assert_eq!(out[0]["id"], 3);
        assert!(garbage.is_empty());
    }

    /// Reproduce una sesión real capturada de `agent acp` (handshake, razonamiento, texto,
    /// edición, `ls` y fin de turno) y comprueba los eventos de dominio resultantes.
    #[test]
    fn a_real_captured_session_maps_to_the_expected_events() {
        let path: std::path::PathBuf = std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("tests/fixtures/cursor_acp_session.jsonl");
        let recording: String = std::fs::read_to_string(path).expect("fixture");
        let mut protocol: AcpProtocol = protocol(AcpSessionConfig::default());
        protocol.start();
        let mut events: Vec<AgentEvent> = Vec::new();
        for line in recording
            .lines()
            .filter_map(|l: &str| l.strip_prefix("<< "))
        {
            events.extend(protocol.decode(TaskId::new(), line));
            if protocol.phase == Phase::Ready && protocol.queued.is_empty() && events.is_empty() {
                // Primer instante con la sesión abierta: llega el prompt del usuario.
                protocol
                    .encode(&prompt("crea hello.txt"), "r")
                    .expect("encode");
            }
            protocol.take_outgoing();
        }

        let kinds: Vec<&str> = events
            .iter()
            .map(|e: &AgentEvent| match e {
                AgentEvent::RunStarted => "run_started",
                AgentEvent::ThinkingDelta { .. } => "thinking",
                AgentEvent::TextDelta { .. } => "text",
                AgentEvent::AssistantMessage { .. } => "message",
                AgentEvent::ToolStarted { .. } => "tool_started",
                AgentEvent::ToolFinished { .. } => "tool_finished",
                AgentEvent::RunSettled => "settled",
                _ => "other",
            })
            .collect();
        assert_eq!(kinds.first(), Some(&"run_started"));
        assert_eq!(kinds.last(), Some(&"settled"));
        assert_eq!(
            kinds
                .iter()
                .filter(|k: &&&str| **k == "tool_started")
                .count(),
            2
        );
        assert_eq!(
            kinds
                .iter()
                .filter(|k: &&&str| **k == "tool_finished")
                .count(),
            2
        );
        assert!(events.iter().any(|e: &AgentEvent| matches!(e,
            AgentEvent::ToolFinished { tool_name, output, is_error: false, .. }
                if tool_name == "execute" && output.contains("hello.txt"))));
        assert!(events.iter().any(|e: &AgentEvent| matches!(e,
            AgentEvent::ToolStarted { tool_name, args, .. }
                if tool_name == "edit" && args.contains("/tmp/acp/hello.txt"))));
        assert!(events.iter().any(|e: &AgentEvent| matches!(e,
            AgentEvent::AssistantMessage { text, thinking: Some(_), .. } if text.starts_with("Cre"))));
    }
}
