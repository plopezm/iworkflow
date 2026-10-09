use serde::{Deserialize, Serialize};

/// Nivel de un aviso mostrado al usuario.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum NoticeLevel {
    Info,
    Warning,
    Error,
}

/// Tipo de diálogo que el agente (o una extensión) solicita al usuario.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum DialogKind {
    Select { options: Vec<String> },
    Confirm { message: Option<String> },
    Input { placeholder: Option<String> },
    Editor { prefill: Option<String> },
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct DialogRequest {
    pub id: String,
    pub title: String,
    pub kind: DialogKind,
    pub timeout_ms: Option<u64>,
}

/// Respuesta del usuario a un diálogo.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
pub enum DialogAnswer {
    Value { value: String },
    Confirmed { confirmed: bool },
    Cancelled,
}

/// Evento normalizado emitido por un agente gentle-shell en modo RPC.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum AgentEvent {
    RunStarted,
    RunSettled,
    UserMessage {
        text: String,
        /// Número de imágenes adjuntas al mensaje.
        #[serde(default)]
        images: u32,
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
    DialogRequested(DialogRequest),
    DialogResolved {
        dialog_id: String,
    },
    Notice {
        level: NoticeLevel,
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

impl AgentEvent {
    /// Los eventos persistentes forman el historial; el resto solo se emite en vivo.
    pub fn is_persistent(&self) -> bool {
        !matches!(
            self,
            AgentEvent::TextDelta { .. }
                | AgentEvent::ThinkingDelta { .. }
                | AgentEvent::ToolUpdated { .. }
                | AgentEvent::QueueChanged { .. }
                | AgentEvent::StatusChanged { .. }
                | AgentEvent::WidgetChanged { .. }
                | AgentEvent::SubagentsChanged { .. }
        )
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn deltas_are_ephemeral_and_messages_persistent() {
        assert!(!AgentEvent::TextDelta { delta: "x".into() }.is_persistent());
        assert!(
            AgentEvent::AssistantMessage {
                text: "x".into(),
                thinking: None,
                error: None
            }
            .is_persistent()
        );
        assert!(AgentEvent::RunSettled.is_persistent());
    }

    #[test]
    fn serializes_with_type_tag() {
        let json: String = serde_json::to_string(&AgentEvent::RunStarted).expect("json");
        assert_eq!(json, r#"{"type":"run_started"}"#);
    }
}
