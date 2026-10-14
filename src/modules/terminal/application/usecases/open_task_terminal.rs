use std::sync::Arc;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::task_target::TaskTarget;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;
use crate::modules::terminal::domain::ports::outbound::task_path_port::TaskPathPort;

/// Abre una sesión de terminal en el worktree de una tarea.
pub struct OpenTaskTerminal {
    pty: Arc<dyn PtyPort>,
    paths: Arc<dyn TaskPathPort>,
}

impl OpenTaskTerminal {
    pub fn new(pty: Arc<dyn PtyPort>, paths: Arc<dyn TaskPathPort>) -> Self {
        Self { pty, paths }
    }

    pub async fn execute(
        &self,
        task_id: Uuid,
        size: TerminalSize,
    ) -> Result<TerminalSession, TerminalError> {
        let target: TaskTarget = self.paths.resolve(task_id).await?;
        self.pty
            .spawn(target.workspace_id, &target.name, &target.path, size)
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
    use crate::modules::terminal::mocks::{MockPty, MockTaskPath};

    const SIZE: TerminalSize = TerminalSize { cols: 80, rows: 24 };

    fn target(workspace_id: Uuid) -> TaskTarget {
        TaskTarget {
            workspace_id,
            name: "fix bug".to_string(),
            path: PathBuf::from("/tmp/wt"),
        }
    }

    #[tokio::test]
    async fn opens_session_in_the_task_worktree() {
        let task_id: Uuid = Uuid::new_v4();
        let workspace_id: Uuid = Uuid::new_v4();
        let expected: TerminalSession = TerminalSession {
            id: TerminalSessionId::new(),
            workspace_id,
            workspace_name: "fix bug".to_string(),
            created_at: Utc::now(),
            status: TerminalStatus::Running,
        };
        let mut paths: MockTaskPath = MockTaskPath::new();
        paths
            .expect_resolve()
            .with(eq(task_id))
            .returning(move |_| Ok(target(workspace_id)));
        let returned: TerminalSession = expected.clone();
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .withf(move |id, name, cwd, size| {
                *id == workspace_id
                    && name == "fix bug"
                    && cwd == Path::new("/tmp/wt")
                    && size.cols == 80
                    && size.rows == 24
            })
            .returning(move |_, _, _, _| Ok(returned.clone()));

        let usecase: OpenTaskTerminal = OpenTaskTerminal::new(Arc::new(pty), Arc::new(paths));
        let session: TerminalSession = usecase.execute(task_id, SIZE).await.unwrap();

        assert_eq!(session, expected);
    }

    #[tokio::test]
    async fn fails_when_task_not_found() {
        let mut paths: MockTaskPath = MockTaskPath::new();
        paths
            .expect_resolve()
            .returning(|_| Err(TerminalError::TaskNotFound));
        let usecase: OpenTaskTerminal =
            OpenTaskTerminal::new(Arc::new(MockPty::new()), Arc::new(paths));
        let result = usecase.execute(Uuid::new_v4(), SIZE).await;
        assert_eq!(result, Err(TerminalError::TaskNotFound));
    }

    #[tokio::test]
    async fn propagates_pty_error() {
        let mut paths: MockTaskPath = MockTaskPath::new();
        paths
            .expect_resolve()
            .returning(|_| Ok(target(Uuid::new_v4())));
        let mut pty: MockPty = MockPty::new();
        pty.expect_spawn()
            .returning(|_, _, _, _| Err(TerminalError::Pty("boom".to_string())));
        let usecase: OpenTaskTerminal = OpenTaskTerminal::new(Arc::new(pty), Arc::new(paths));
        let result = usecase.execute(Uuid::new_v4(), SIZE).await;
        assert_eq!(result, Err(TerminalError::Pty("boom".to_string())));
    }
}
