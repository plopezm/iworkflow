use serde::{Deserialize, Serialize};

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStatusDto;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NoticeLevelDto {
    Info,
    Warning,
    Error,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum DialogKindDto {
    Select { options: Vec<String> },
    Confirm { message: Option<String> },
    Input { placeholder: Option<String> },
    Editor { prefill: Option<String> },
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DialogRequestDto {
    pub id: String,
    pub title: String,
    pub kind: DialogKindDto,
    pub timeout_ms: Option<u64>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum DialogAnswerDto {
    Value { value: String },
    Confirmed { confirmed: bool },
    Cancelled,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum AgentEventDto {
    RunStarted,
    RunSettled,
    UserMessage {
        text: String,
    },
    AssistantMessage {
        text: String,
        thinking: Option<String>,
        error: Option<String>,
    },
    TextDelta {
        delta: String,
    },
    ThinkingDelta {
        delta: String,
    },
    ToolStarted {
        call_id: String,
        tool_name: String,
        args: String,
    },
    ToolUpdated {
        call_id: String,
        output: String,
    },
    ToolFinished {
        call_id: String,
        tool_name: String,
        output: String,
        is_error: bool,
    },
    QueueChanged {
        steering: Vec<String>,
        follow_up: Vec<String>,
    },
    DialogRequested {
        request: DialogRequestDto,
    },
    DialogResolved {
        dialog_id: String,
    },
    Notice {
        level: NoticeLevelDto,
        message: String,
    },
    StatusChanged {
        key: String,
        text: Option<String>,
    },
    WidgetChanged {
        key: String,
        lines: Option<Vec<String>>,
    },
    SubagentsChanged {
        active: u32,
        waiting: u32,
    },
    ProcessExited {
        code: Option<i32>,
    },
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "source", rename_all = "snake_case")]
pub enum TaskEventKindDto {
    Agent {
        event: AgentEventDto,
    },
    Status {
        status: TaskStatusDto,
        error: Option<String>,
    },
    /// Título/prompt editados en backlog. Solo notifica al tablero.
    Changed,
    /// Tarea en backlog eliminada. Solo notifica al tablero.
    Removed,
}

/// Evento que viaja por SSE al cliente.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TaskEventDto {
    pub seq: Option<i64>,
    /// RFC 3339 en UTC.
    pub created_at: String,
    pub kind: TaskEventKindDto,
}

#[cfg(feature = "ssr")]
mod mapping {
    use super::*;
    use crate::modules::tasks::domain::models::agent_event::{
        AgentEvent, DialogAnswer, DialogKind, DialogRequest, NoticeLevel,
    };
    use crate::modules::tasks::domain::models::task::TaskStatus;
    use crate::modules::tasks::domain::models::task_event::{TaskEvent, TaskEventKind};

    impl From<NoticeLevel> for NoticeLevelDto {
        fn from(level: NoticeLevel) -> Self {
            match level {
                NoticeLevel::Info => NoticeLevelDto::Info,
                NoticeLevel::Warning => NoticeLevelDto::Warning,
                NoticeLevel::Error => NoticeLevelDto::Error,
            }
        }
    }

    impl From<DialogKind> for DialogKindDto {
        fn from(kind: DialogKind) -> Self {
            match kind {
                DialogKind::Select { options } => DialogKindDto::Select { options },
                DialogKind::Confirm { message } => DialogKindDto::Confirm { message },
                DialogKind::Input { placeholder } => DialogKindDto::Input { placeholder },
                DialogKind::Editor { prefill } => DialogKindDto::Editor { prefill },
            }
        }
    }

    impl From<DialogRequest> for DialogRequestDto {
        fn from(request: DialogRequest) -> Self {
            Self {
                id: request.id,
                title: request.title,
                kind: request.kind.into(),
                timeout_ms: request.timeout_ms,
            }
        }
    }

    impl From<DialogAnswerDto> for DialogAnswer {
        fn from(answer: DialogAnswerDto) -> Self {
            match answer {
                DialogAnswerDto::Value { value } => DialogAnswer::Value { value },
                DialogAnswerDto::Confirmed { confirmed } => DialogAnswer::Confirmed { confirmed },
                DialogAnswerDto::Cancelled => DialogAnswer::Cancelled,
            }
        }
    }

    impl From<AgentEvent> for AgentEventDto {
        fn from(event: AgentEvent) -> Self {
            match event {
                AgentEvent::RunStarted => AgentEventDto::RunStarted,
                AgentEvent::RunSettled => AgentEventDto::RunSettled,
                AgentEvent::UserMessage { text } => AgentEventDto::UserMessage { text },
                AgentEvent::AssistantMessage {
                    text,
                    thinking,
                    error,
                } => AgentEventDto::AssistantMessage {
                    text,
                    thinking,
                    error,
                },
                AgentEvent::TextDelta { delta } => AgentEventDto::TextDelta { delta },
                AgentEvent::ThinkingDelta { delta } => AgentEventDto::ThinkingDelta { delta },
                AgentEvent::ToolStarted {
                    call_id,
                    tool_name,
                    args,
                } => AgentEventDto::ToolStarted {
                    call_id,
                    tool_name,
                    args,
                },
                AgentEvent::ToolUpdated { call_id, output } => {
                    AgentEventDto::ToolUpdated { call_id, output }
                }
                AgentEvent::ToolFinished {
                    call_id,
                    tool_name,
                    output,
                    is_error,
                } => AgentEventDto::ToolFinished {
                    call_id,
                    tool_name,
                    output,
                    is_error,
                },
                AgentEvent::QueueChanged {
                    steering,
                    follow_up,
                } => AgentEventDto::QueueChanged {
                    steering,
                    follow_up,
                },
                AgentEvent::DialogRequested(request) => AgentEventDto::DialogRequested {
                    request: request.into(),
                },
                AgentEvent::DialogResolved { dialog_id } => {
                    AgentEventDto::DialogResolved { dialog_id }
                }
                AgentEvent::Notice { level, message } => AgentEventDto::Notice {
                    level: level.into(),
                    message,
                },
                AgentEvent::StatusChanged { key, text } => {
                    AgentEventDto::StatusChanged { key, text }
                }
                AgentEvent::WidgetChanged { key, lines } => {
                    AgentEventDto::WidgetChanged { key, lines }
                }
                AgentEvent::SubagentsChanged { active, waiting } => {
                    AgentEventDto::SubagentsChanged { active, waiting }
                }
                AgentEvent::ProcessExited { code } => AgentEventDto::ProcessExited { code },
            }
        }
    }

    impl From<TaskEvent> for TaskEventDto {
        fn from(event: TaskEvent) -> Self {
            let kind: TaskEventKindDto = match event.kind {
                TaskEventKind::Agent { event } => TaskEventKindDto::Agent {
                    event: event.into(),
                },
                TaskEventKind::Status { status, error } => TaskEventKindDto::Status {
                    status: TaskStatus::parse(&status)
                        .map(TaskStatusDto::from)
                        .unwrap_or(TaskStatusDto::Failed),
                    error,
                },
                TaskEventKind::Changed => TaskEventKindDto::Changed,
                TaskEventKind::Removed => TaskEventKindDto::Removed,
            };
            Self {
                seq: event.seq,
                created_at: event.created_at.to_rfc3339(),
                kind,
            }
        }
    }

    #[cfg(test)]
    mod tests {
        use super::*;
        use crate::modules::tasks::domain::models::task::TaskId;

        #[test]
        fn status_event_maps_to_typed_status() {
            let event: TaskEvent = TaskEvent {
                task_id: TaskId::new(),
                seq: Some(4),
                kind: TaskEventKind::status(TaskStatus::Conflict, Some("x".into())),
                created_at: chrono::Utc::now(),
            };

            let dto: TaskEventDto = TaskEventDto::from(event);

            assert_eq!(dto.seq, Some(4));
            assert_eq!(
                dto.kind,
                TaskEventKindDto::Status {
                    status: TaskStatusDto::Conflict,
                    error: Some("x".into())
                }
            );
        }

        #[test]
        fn dialog_request_keeps_options() {
            let dto: AgentEventDto = AgentEvent::DialogRequested(DialogRequest {
                id: "d".into(),
                title: "¿?".into(),
                kind: DialogKind::Select {
                    options: vec!["a".into()],
                },
                timeout_ms: None,
            })
            .into();
            assert!(matches!(
                dto,
                AgentEventDto::DialogRequested { request } if request.kind == DialogKindDto::Select { options: vec!["a".into()] }
            ));
        }

        #[test]
        fn dialog_answer_maps_to_domain() {
            assert_eq!(
                DialogAnswer::from(DialogAnswerDto::Value { value: "v".into() }),
                DialogAnswer::Value { value: "v".into() }
            );
        }
    }
}
