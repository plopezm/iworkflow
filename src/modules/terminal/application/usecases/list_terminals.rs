use std::sync::Arc;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

/// Lista las sesiones de terminal abiertas, de la más reciente a la más antigua.
pub struct ListTerminals {
    pty: Arc<dyn PtyPort>,
}

impl ListTerminals {
    pub fn new(pty: Arc<dyn PtyPort>) -> Self {
        Self { pty }
    }

    pub async fn execute(&self) -> Result<Vec<TerminalSession>, TerminalError> {
        let mut sessions: Vec<TerminalSession> = self.pty.list().await?;
        sessions.sort_by_key(|session: &TerminalSession| std::cmp::Reverse(session.created_at));
        Ok(sessions)
    }
}

#[cfg(test)]
mod tests {
    use chrono::{DateTime, Duration, Utc};
    use uuid::Uuid;

    use super::*;
    use crate::modules::terminal::domain::models::terminal_session::TerminalStatus;
    use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
    use crate::modules::terminal::mocks::MockPty;

    fn session(name: &str, created_at: DateTime<Utc>) -> TerminalSession {
        TerminalSession {
            id: TerminalSessionId::new(),
            workspace_id: Uuid::new_v4(),
            workspace_name: name.to_string(),
            created_at,
            status: TerminalStatus::Running,
        }
    }

    #[tokio::test]
    async fn lists_newest_first() {
        let now: DateTime<Utc> = Utc::now();
        let old: TerminalSession = session("old", now - Duration::minutes(5));
        let new: TerminalSession = session("new", now);
        let returned: Vec<TerminalSession> = vec![old.clone(), new.clone()];
        let mut pty: MockPty = MockPty::new();
        pty.expect_list().returning(move || Ok(returned.clone()));

        let usecase: ListTerminals = ListTerminals::new(Arc::new(pty));
        let sessions: Vec<TerminalSession> = usecase.execute().await.unwrap();

        assert_eq!(sessions, vec![new, old]);
    }

    #[tokio::test]
    async fn returns_empty_list() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_list().returning(|| Ok(Vec::new()));
        let usecase: ListTerminals = ListTerminals::new(Arc::new(pty));
        assert_eq!(usecase.execute().await, Ok(Vec::new()));
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_list()
            .returning(|| Err(TerminalError::Pty("boom".to_string())));
        let usecase: ListTerminals = ListTerminals::new(Arc::new(pty));
        assert_eq!(
            usecase.execute().await,
            Err(TerminalError::Pty("boom".to_string()))
        );
    }
}
