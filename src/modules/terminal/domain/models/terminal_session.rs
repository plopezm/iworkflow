use uuid::Uuid;

use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TerminalSession {
    pub id: TerminalSessionId,
    pub workspace_id: Uuid,
}
