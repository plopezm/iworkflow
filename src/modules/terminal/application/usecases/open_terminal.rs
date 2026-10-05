use std::path::PathBuf;
use std::sync::Arc;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;
use crate::modules::terminal::domain::ports::outbound::workspace_path_port::WorkspacePathPort;

/// Abre una sesión de terminal en el directorio de un workspace.
pub struct OpenTerminal {
    pty: Arc<dyn PtyPort>,
    paths: Arc<dyn WorkspacePathPort>,
}

impl OpenTerminal {
    pub fn new(pty: Arc<dyn PtyPort>, paths: Arc<dyn WorkspacePathPort>) -> Self {
        Self { pty, paths }
    }

    pub async fn execute(
        &self,
        workspace_id: Uuid,
        cols: u16,
        rows: u16,
    ) -> Result<TerminalSession, TerminalError> {
        let size: TerminalSize = TerminalSize::new(cols, rows)?;
        let cwd: PathBuf = self.paths.resolve(workspace_id).await?;
        let id: TerminalSessionId = self.pty.spawn(&cwd, size).await?;
        Ok(TerminalSession { id, workspace_id })
    }
}

#[cfg(test)]
mod tests {
    use mockall::predicate::{always, eq};

    use super::*;
    use crate::modules::terminal::mocks::{MockPty, MockWorkspacePath};

    #[tokio::test]
    async fn opens_session_in_workspace_dir() {
        let workspace_id: Uuid = Uuid::new_v4();
        let session_id: TerminalSessionId = TerminalSessionId::new();
        let mut paths: MockWorkspacePath = MockWorkspacePath::new();
        paths
            .expect_resolve()
            .with(eq(workspace_id))
            .returning(|_| Ok(PathBuf::from("/tmp/ws")));
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .withf(|cwd, size| {
                cwd == std::path::Path::new("/tmp/ws") && size.cols == 80 && size.rows == 24
            })
            .returning(move |_, _| Ok(session_id));

        let usecase: OpenTerminal = OpenTerminal::new(Arc::new(pty), Arc::new(paths));
        let session: TerminalSession = usecase.execute(workspace_id, 80, 24).await.unwrap();

        assert_eq!(
            session,
            TerminalSession {
                id: session_id,
                workspace_id
            }
        );
    }

    #[tokio::test]
    async fn rejects_zero_size_without_calling_ports() {
        let usecase: OpenTerminal =
            OpenTerminal::new(Arc::new(MockPty::new()), Arc::new(MockWorkspacePath::new()));
        let result = usecase.execute(Uuid::new_v4(), 0, 24).await;
        assert_eq!(result, Err(TerminalError::InvalidSize));
    }

    #[tokio::test]
    async fn fails_when_workspace_not_found() {
        let mut paths: MockWorkspacePath = MockWorkspacePath::new();
        paths
            .expect_resolve()
            .returning(|_| Err(TerminalError::WorkspaceNotFound));
        let usecase: OpenTerminal = OpenTerminal::new(Arc::new(MockPty::new()), Arc::new(paths));
        let result = usecase.execute(Uuid::new_v4(), 80, 24).await;
        assert_eq!(result, Err(TerminalError::WorkspaceNotFound));
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut paths: MockWorkspacePath = MockWorkspacePath::new();
        paths
            .expect_resolve()
            .returning(|_| Ok(PathBuf::from("/tmp/ws")));
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .with(always(), always())
            .returning(|_, _| Err(TerminalError::Pty("boom".to_string())));
        let usecase: OpenTerminal = OpenTerminal::new(Arc::new(pty), Arc::new(paths));
        let result = usecase.execute(Uuid::new_v4(), 80, 24).await;
        assert_eq!(result, Err(TerminalError::Pty("boom".to_string())));
    }
}
