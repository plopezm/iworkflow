use serde::{Deserialize, Serialize};

#[cfg(feature = "ssr")]
use crate::modules::terminal::domain::models::terminal_session::{TerminalSession, TerminalStatus};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TerminalStatusDto {
    Running,
    Exited,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct TerminalSessionDto {
    pub id: String,
    pub workspace_id: String,
    pub workspace_name: String,
    /// Instante de apertura en RFC3339.
    pub created_at: String,
    pub status: TerminalStatusDto,
}

#[cfg(feature = "ssr")]
impl From<TerminalStatus> for TerminalStatusDto {
    fn from(status: TerminalStatus) -> Self {
        match status {
            TerminalStatus::Running => Self::Running,
            TerminalStatus::Exited => Self::Exited,
        }
    }
}

#[cfg(feature = "ssr")]
impl From<TerminalSession> for TerminalSessionDto {
    fn from(session: TerminalSession) -> Self {
        Self {
            id: session.id.to_string(),
            workspace_id: session.workspace_id.to_string(),
            workspace_name: session.workspace_name,
            created_at: session.created_at.to_rfc3339(),
            status: session.status.into(),
        }
    }
}

#[cfg(all(test, feature = "ssr"))]
mod tests {
    use chrono::{TimeZone, Utc};
    use uuid::Uuid;

    use super::*;
    use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;

    fn session(status: TerminalStatus) -> TerminalSession {
        TerminalSession {
            id: TerminalSessionId::from_uuid(Uuid::from_u128(1)),
            workspace_id: Uuid::from_u128(2),
            workspace_name: "demo".to_string(),
            created_at: Utc.with_ymd_and_hms(2026, 1, 2, 3, 4, 5).unwrap(),
            status,
        }
    }

    #[test]
    fn maps_session_to_dto() {
        let dto: TerminalSessionDto = session(TerminalStatus::Running).into();
        assert_eq!(dto.id, Uuid::from_u128(1).to_string());
        assert_eq!(dto.workspace_id, Uuid::from_u128(2).to_string());
        assert_eq!(dto.workspace_name, "demo");
        assert_eq!(dto.created_at, "2026-01-02T03:04:05+00:00");
        assert_eq!(dto.status, TerminalStatusDto::Running);
    }

    #[test]
    fn maps_exited_status() {
        let dto: TerminalSessionDto = session(TerminalStatus::Exited).into();
        assert_eq!(dto.status, TerminalStatusDto::Exited);
    }

    #[test]
    fn dto_roundtrips_through_json() {
        let dto: TerminalSessionDto = session(TerminalStatus::Exited).into();
        let json: String = serde_json::to_string(&dto).unwrap();
        let back: TerminalSessionDto = serde_json::from_str(&json).unwrap();
        assert_eq!(back, dto);
    }
}
