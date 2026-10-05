use std::sync::Arc;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::models::workspace_target::WorkspaceTarget;
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
        size: TerminalSize,
    ) -> Result<TerminalSession, TerminalError> {
        let target: WorkspaceTarget = self.paths.resolve(workspace_id).await?;
        self.pty
            .spawn(workspace_id, &target.name, &target.path, size)
            .await
    }
}

#[cfg(test)]
mod tests {
    use std::path::{Path, PathBuf};

    use chrono::Utc;
    use mockall::predicate::eq;

    use super::*;
    use crate::modules::terminal::domain::models::terminal_session::TerminalStatus;
    use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
    use crate::modules::terminal::mocks::{MockPty, MockWorkspacePath};

    const SIZE: TerminalSize = TerminalSize { cols: 80, rows: 24 };

    fn target() -> WorkspaceTarget {
        WorkspaceTarget {
            name: "demo".to_string(),
            path: PathBuf::from("/tmp/ws"),
        }
    }

    #[tokio::test]
    async fn opens_session_with_workspace_snapshot() {
        let workspace_id: Uuid = Uuid::new_v4();
        let expected: TerminalSession = TerminalSession {
            id: TerminalSessionId::new(),
            workspace_id,
            workspace_name: "demo".to_string(),
            created_at: Utc::now(),
            status: TerminalStatus::Running,
        };
        let mut paths: MockWorkspacePath = MockWorkspacePath::new();
        paths
            .expect_resolve()
            .with(eq(workspace_id))
            .returning(|_| Ok(target()));
        let returned: TerminalSession = expected.clone();
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .withf(move |id, name, cwd, size| {
                *id == workspace_id
                    && name == "demo"
                    && cwd == Path::new("/tmp/ws")
                    && size.cols == 80
                    && size.rows == 24
            })
            .returning(move |_, _, _, _| Ok(returned.clone()));

        let usecase: OpenTerminal = OpenTerminal::new(Arc::new(pty), Arc::new(paths));
        let session: TerminalSession = usecase.execute(workspace_id, SIZE).await.unwrap();

        assert_eq!(session, expected);
    }

    #[tokio::test]
    async fn fails_when_workspace_not_found() {
        let mut paths: MockWorkspacePath = MockWorkspacePath::new();
        paths
            .expect_resolve()
            .returning(|_| Err(TerminalError::WorkspaceNotFound));
        let usecase: OpenTerminal = OpenTerminal::new(Arc::new(MockPty::new()), Arc::new(paths));
        let result = usecase.execute(Uuid::new_v4(), SIZE).await;
        assert_eq!(result, Err(TerminalError::WorkspaceNotFound));
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut paths: MockWorkspacePath = MockWorkspacePath::new();
        paths.expect_resolve().returning(|_| Ok(target()));
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .returning(|_, _, _, _| Err(TerminalError::Pty("boom".to_string())));
        let usecase: OpenTerminal = OpenTerminal::new(Arc::new(pty), Arc::new(paths));
        let result = usecase.execute(Uuid::new_v4(), SIZE).await;
        assert_eq!(result, Err(TerminalError::Pty("boom".to_string())));
    }
}
