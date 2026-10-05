use chrono::{DateTime, Utc};
use uuid::Uuid;

use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TerminalStatus {
    Running,
    Exited,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TerminalSession {
    pub id: TerminalSessionId,
    pub workspace_id: Uuid,
    /// Nombre del workspace capturado al abrir la sesión (no se actualiza después).
    pub workspace_name: String,
    pub created_at: DateTime<Utc>,
    pub status: TerminalStatus,
}
