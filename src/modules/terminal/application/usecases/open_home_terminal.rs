use std::path::PathBuf;
use std::sync::Arc;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;

/// Nombre con el que se registran las sesiones que no pertenecen a ningún workspace.
pub const HOME_TERMINAL_NAME: &str = "Home";

/// Abre una sesión de terminal en el directorio personal del usuario (sin workspace).
pub struct OpenHomeTerminal {
    pty: Arc<dyn PtyPort>,
    home_dir: PathBuf,
}

impl OpenHomeTerminal {
    pub fn new(pty: Arc<dyn PtyPort>, home_dir: PathBuf) -> Self {
        Self { pty, home_dir }
    }

    pub async fn execute(&self, size: TerminalSize) -> Result<TerminalSession, TerminalError> {
        self.pty
            .spawn(Uuid::nil(), HOME_TERMINAL_NAME, &self.home_dir, size)
            .await
    }
}

#[cfg(test)]
mod tests {
    use std::path::Path;

    use chrono::Utc;

    use super::*;
    use crate::modules::terminal::domain::models::terminal_session::TerminalStatus;
    use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
    use crate::modules::terminal::mocks::MockPty;

    const SIZE: TerminalSize = TerminalSize { cols: 80, rows: 24 };

    #[tokio::test]
    async fn spawns_in_home_without_workspace() {
        let expected: TerminalSession = TerminalSession {
            id: TerminalSessionId::new(),
            workspace_id: Uuid::nil(),
            workspace_name: HOME_TERMINAL_NAME.to_string(),
            created_at: Utc::now(),
            status: TerminalStatus::Running,
        };
        let returned: TerminalSession = expected.clone();
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .withf(|id, name, cwd, size| {
                id.is_nil()
                    && name == HOME_TERMINAL_NAME
                    && cwd == Path::new("/home/u")
                    && size.cols == 80
                    && size.rows == 24
            })
            .returning(move |_, _, _, _| Ok(returned.clone()));

        let usecase: OpenHomeTerminal =
            OpenHomeTerminal::new(Arc::new(pty), PathBuf::from("/home/u"));
        let session: TerminalSession = usecase.execute(SIZE).await.unwrap();

        assert_eq!(session, expected);
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .returning(|_, _, _, _| Err(TerminalError::Pty("boom".to_string())));
        let usecase: OpenHomeTerminal =
            OpenHomeTerminal::new(Arc::new(pty), PathBuf::from("/home/u"));
        let result = usecase.execute(SIZE).await;
        assert_eq!(result, Err(TerminalError::Pty("boom".to_string())));
    }
}
