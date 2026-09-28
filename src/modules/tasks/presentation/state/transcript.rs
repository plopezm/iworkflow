//! Vista de la conversación de una tarea reconstruida a partir de sus eventos.

use std::collections::BTreeMap;

use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_dto::TaskStatusDto;
use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::{
    AgentEventDto, DialogRequestDto, NoticeLevelDto, TaskEventDto, TaskEventKindDto,
};

#[derive(Debug, Clone, PartialEq)]
pub enum TranscriptItem {
    User {
        text: String,
    },
    Assistant {
        text: String,
        thinking: Option<String>,
        error: Option<String>,
    },
    Tool {
        call_id: String,
        name: String,
        args: String,
        output: String,
        /// `None` mientras la herramienta se está ejecutando.
        is_error: Option<bool>,
    },
    Notice {
        level: NoticeLevelDto,
        message: String,
    },
    Status {
        status: TaskStatusDto,
        error: Option<String>,
    },
    Dialog {
        request: DialogRequestDto,
        resolved: bool,
    },
    Exit {
        code: Option<i32>,
    },
}

/// Estado de pantalla derivado del stream de eventos.
#[derive(Debug, Clone, Default, PartialEq)]
pub struct Transcript {
    pub items: Vec<TranscriptItem>,
    /// Texto del mensaje del asistente que se está generando.
    pub streaming_text: String,
    pub streaming_thinking: String,
    /// Entradas de estado de las extensiones (`setStatus`).
    pub statuses: BTreeMap<String, String>,
    /// Widgets de las extensiones (`setWidget`).
    pub widgets: BTreeMap<String, Vec<String>>,
    pub queued_steering: Vec<String>,
    pub queued_follow_up: Vec<String>,
}

impl Transcript {
    pub fn apply(&mut self, event: TaskEventDto) {
        match event.kind {
            TaskEventKindDto::Status { status, error } => {
                self.items.push(TranscriptItem::Status { status, error })
            }
            TaskEventKindDto::Agent { event } => self.apply_agent(event),
        }
    }

    fn apply_agent(&mut self, event: AgentEventDto) {
        match event {
            AgentEventDto::RunStarted => {}
            AgentEventDto::RunSettled => self.clear_streaming(),
            AgentEventDto::UserMessage { text } => self.items.push(TranscriptItem::User { text }),
            AgentEventDto::AssistantMessage {
                text,
                thinking,
                error,
            } => {
                self.clear_streaming();
                self.items.push(TranscriptItem::Assistant {
                    text,
                    thinking,
                    error,
                });
            }
            AgentEventDto::TextDelta { delta } => self.streaming_text.push_str(&delta),
            AgentEventDto::ThinkingDelta { delta } => self.streaming_thinking.push_str(&delta),
            AgentEventDto::ToolStarted {
                call_id,
                tool_name,
                args,
            } => {
                // Las tools empiezan tras cerrar el bloque de texto previo.
                self.clear_streaming();
                self.items.push(TranscriptItem::Tool {
                    call_id,
                    name: tool_name,
                    args,
                    output: String::new(),
                    is_error: None,
                });
            }
            AgentEventDto::ToolUpdated { call_id, output } => {
                if let Some(TranscriptItem::Tool {
                    output: current, ..
                }) = self.tool_mut(&call_id)
                {
                    *current = output;
                }
            }
            AgentEventDto::ToolFinished {
                call_id,
                tool_name,
                output,
                is_error,
            } => match self.tool_mut(&call_id) {
                Some(TranscriptItem::Tool {
                    output: current,
                    is_error: current_error,
                    ..
                }) => {
                    *current = output;
                    *current_error = Some(is_error);
                }
                _ => self.items.push(TranscriptItem::Tool {
                    call_id,
                    name: tool_name,
                    args: String::new(),
                    output,
                    is_error: Some(is_error),
                }),
            },
            AgentEventDto::QueueChanged {
                steering,
                follow_up,
            } => {
                self.queued_steering = steering;
                self.queued_follow_up = follow_up;
            }
            AgentEventDto::DialogRequested { request } => self.items.push(TranscriptItem::Dialog {
                request,
                resolved: false,
            }),
            AgentEventDto::DialogResolved { dialog_id } => self.resolve_dialogs(Some(&dialog_id)),
            AgentEventDto::Notice { level, message } => {
                self.items.push(TranscriptItem::Notice { level, message })
            }
            AgentEventDto::StatusChanged { key, text } => match text {
                Some(text) if !text.is_empty() => {
                    self.statuses.insert(key, text);
                }
                _ => {
                    self.statuses.remove(&key);
                }
            },
            AgentEventDto::WidgetChanged { key, lines } => match lines {
                Some(lines) if !lines.is_empty() => {
                    self.widgets.insert(key, lines);
                }
                _ => {
                    self.widgets.remove(&key);
                }
            },
            AgentEventDto::ProcessExited { code } => {
                self.clear_streaming();
                self.resolve_dialogs(None);
                self.queued_steering.clear();
                self.queued_follow_up.clear();
                self.statuses.clear();
                self.widgets.clear();
                self.items.push(TranscriptItem::Exit { code });
            }
        }
    }

    /// Diálogo abierto más reciente, si lo hay.
    pub fn pending_dialog(&self) -> Option<DialogRequestDto> {
        self.items
            .iter()
            .rev()
            .find_map(|item: &TranscriptItem| match item {
                TranscriptItem::Dialog {
                    request,
                    resolved: false,
                } => Some(request.clone()),
                _ => None,
            })
    }

    pub fn is_streaming(&self) -> bool {
        !self.streaming_text.is_empty() || !self.streaming_thinking.is_empty()
    }

    fn clear_streaming(&mut self) {
        self.streaming_text.clear();
        self.streaming_thinking.clear();
    }

    fn tool_mut(&mut self, call_id: &str) -> Option<&mut TranscriptItem> {
        self.items.iter_mut().rev().find(|item: &&mut TranscriptItem| {
            matches!(item, TranscriptItem::Tool { call_id: id, .. } if id == call_id)
        })
    }

    /// Marca como resueltos el diálogo indicado o, con `None`, todos los abiertos.
    fn resolve_dialogs(&mut self, dialog_id: Option<&str>) {
        for item in &mut self.items {
            if let TranscriptItem::Dialog { request, resolved } = item
                && dialog_id.is_none_or(|id: &str| request.id == id)
            {
                *resolved = true;
            }
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::tasks::infrastructure::adapters::inbound::dto::task_event_dto::DialogKindDto;

    fn agent(event: AgentEventDto) -> TaskEventDto {
        TaskEventDto {
            seq: None,
            created_at: String::new(),
            kind: TaskEventKindDto::Agent { event },
        }
    }

    fn dialog(id: &str) -> AgentEventDto {
        AgentEventDto::DialogRequested {
            request: DialogRequestDto {
                id: id.to_string(),
                title: "¿Seguir?".to_string(),
                kind: DialogKindDto::Confirm { message: None },
                timeout_ms: None,
            },
        }
    }

    #[test]
    fn streaming_text_is_replaced_by_final_message() {
        let mut transcript: Transcript = Transcript::default();
        transcript.apply(agent(AgentEventDto::TextDelta { delta: "Ho".into() }));
        transcript.apply(agent(AgentEventDto::TextDelta { delta: "la".into() }));
        assert_eq!(transcript.streaming_text, "Hola");

        transcript.apply(agent(AgentEventDto::AssistantMessage {
            text: "Hola".into(),
            thinking: None,
            error: None,
        }));

        assert!(!transcript.is_streaming());
        assert_eq!(transcript.items.len(), 1);
    }

    #[test]
    fn tool_lifecycle_updates_a_single_item() {
        let mut transcript: Transcript = Transcript::default();
        transcript.apply(agent(AgentEventDto::ToolStarted {
            call_id: "c1".into(),
            tool_name: "bash".into(),
            args: "{}".into(),
        }));
        transcript.apply(agent(AgentEventDto::ToolUpdated {
            call_id: "c1".into(),
            output: "parcial".into(),
        }));
        transcript.apply(agent(AgentEventDto::ToolFinished {
            call_id: "c1".into(),
            tool_name: "bash".into(),
            output: "final".into(),
            is_error: false,
        }));

        assert_eq!(
            transcript.items,
            vec![TranscriptItem::Tool {
                call_id: "c1".into(),
                name: "bash".into(),
                args: "{}".into(),
                output: "final".into(),
                is_error: Some(false),
            }]
        );
    }

    #[test]
    fn dialogs_are_pending_until_resolved_or_process_exit() {
        let mut transcript: Transcript = Transcript::default();
        transcript.apply(agent(dialog("d1")));
        transcript.apply(agent(dialog("d2")));
        assert_eq!(
            transcript.pending_dialog().map(|d| d.id),
            Some("d2".to_string())
        );

        transcript.apply(agent(AgentEventDto::DialogResolved {
            dialog_id: "d2".into(),
        }));
        assert_eq!(
            transcript.pending_dialog().map(|d| d.id),
            Some("d1".to_string())
        );

        transcript.apply(agent(AgentEventDto::ProcessExited { code: Some(0) }));
        assert_eq!(transcript.pending_dialog(), None);
    }

    #[test]
    fn extension_status_and_widgets_can_be_cleared() {
        let mut transcript: Transcript = Transcript::default();
        transcript.apply(agent(AgentEventDto::StatusChanged {
            key: "mcp".into(),
            text: Some("ok".into()),
        }));
        transcript.apply(agent(AgentEventDto::WidgetChanged {
            key: "todo".into(),
            lines: Some(vec!["- a".into()]),
        }));
        assert_eq!(
            transcript.statuses.get("mcp").map(String::as_str),
            Some("ok")
        );

        transcript.apply(agent(AgentEventDto::StatusChanged {
            key: "mcp".into(),
            text: None,
        }));
        transcript.apply(agent(AgentEventDto::WidgetChanged {
            key: "todo".into(),
            lines: None,
        }));

        assert!(transcript.statuses.is_empty());
        assert!(transcript.widgets.is_empty());
    }

    #[test]
    fn status_events_are_listed_in_order() {
        let mut transcript: Transcript = Transcript::default();
        transcript.apply(TaskEventDto {
            seq: Some(1),
            created_at: String::new(),
            kind: TaskEventKindDto::Status {
                status: TaskStatusDto::Running,
                error: None,
            },
        });
        assert_eq!(
            transcript.items,
            vec![TranscriptItem::Status {
                status: TaskStatusDto::Running,
                error: None
            }]
        );
    }
}
