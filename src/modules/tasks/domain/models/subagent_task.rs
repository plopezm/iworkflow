/// Estado de un subagente, neutro respecto al runtime que lo ejecuta.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SubagentStatus {
    Queued,
    Running,
    /// Bloqueado esperando una respuesta del usuario.
    Waiting,
    Completed,
    Failed,
    Cancelled,
    TimedOut,
    /// Estado que este cliente no reconoce.
    Unknown,
}

impl SubagentStatus {
    /// Sigue en cola, ejecutándose o esperando respuesta.
    pub fn is_active(&self) -> bool {
        matches!(self, Self::Queued | Self::Running | Self::Waiting)
    }
}

/// Elemento del hilo de actividad de un subagente.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SubagentThreadItem {
    Text(String),
    Thinking(String),
    Note(String),
    Tool {
        name: String,
        args: Option<String>,
        running: bool,
        is_error: bool,
        output: Option<String>,
    },
}

/// Un subagente lanzado por el agente de una tarea, con su actividad. Las marcas de
/// tiempo son milisegundos desde la época Unix.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SubagentTask {
    pub id: String,
    pub agent: String,
    pub label: Option<String>,
    pub prompt: Option<String>,
    pub status: SubagentStatus,
    pub created_at: Option<i64>,
    pub started_at: Option<i64>,
    pub ended_at: Option<i64>,
    pub last_step: Option<String>,
    pub last_activity_at: Option<i64>,
    pub turns: u32,
    pub tool_calls: u32,
    pub error: Option<String>,
    pub thread: Vec<SubagentThreadItem>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn only_queued_running_and_waiting_are_active() {
        assert!(SubagentStatus::Queued.is_active());
        assert!(SubagentStatus::Running.is_active());
        assert!(SubagentStatus::Waiting.is_active());
        assert!(!SubagentStatus::Completed.is_active());
        assert!(!SubagentStatus::Failed.is_active());
        assert!(!SubagentStatus::Cancelled.is_active());
        assert!(!SubagentStatus::TimedOut.is_active());
        assert!(!SubagentStatus::Unknown.is_active());
    }
}
