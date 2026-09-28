use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};

use crate::modules::tasks::domain::models::agent_event::AgentEvent;
use crate::modules::tasks::domain::models::task::{TaskId, TaskStatus};

/// Contenido de un evento de tarea.
#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "source", rename_all = "snake_case")]
pub enum TaskEventKind {
    Agent {
        event: AgentEvent,
    },
    Status {
        status: String,
        error: Option<String>,
    },
}

impl TaskEventKind {
    pub fn status(status: TaskStatus, error: Option<String>) -> Self {
        TaskEventKind::Status {
            status: status.as_str().to_string(),
            error,
        }
    }

    pub fn is_persistent(&self) -> bool {
        match self {
            TaskEventKind::Agent { event } => event.is_persistent(),
            TaskEventKind::Status { .. } => true,
        }
    }
}

/// Evento de tarea. `seq` existe solo para eventos persistidos (orden global creciente).
#[derive(Debug, Clone, PartialEq)]
pub struct TaskEvent {
    pub task_id: TaskId,
    pub seq: Option<i64>,
    pub kind: TaskEventKind,
    pub created_at: DateTime<Utc>,
}

impl TaskEvent {
    pub fn ephemeral(task_id: TaskId, kind: TaskEventKind) -> Self {
        Self {
            task_id,
            seq: None,
            kind,
            created_at: Utc::now(),
        }
    }
}
