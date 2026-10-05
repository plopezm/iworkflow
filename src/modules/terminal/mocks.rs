use std::path::Path;

use mockall::mock;
use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_attachment::TerminalAttachment;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::models::workspace_target::WorkspaceTarget;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;
use crate::modules::terminal::domain::ports::outbound::workspace_path_port::WorkspacePathPort;

mock! {
    pub Pty {}

    #[async_trait::async_trait]
    impl PtyPort for Pty {
        async fn spawn(&self, workspace_id: Uuid, workspace_name: &str, cwd: &Path, size: TerminalSize) -> Result<TerminalSession, TerminalError>;
        async fn list(&self) -> Result<Vec<TerminalSession>, TerminalError>;
        async fn attach(&self, id: TerminalSessionId) -> Result<TerminalAttachment, TerminalError>;
        async fn write(&self, id: TerminalSessionId, bytes: Vec<u8>) -> Result<(), TerminalError>;
        async fn resize(&self, id: TerminalSessionId, size: TerminalSize) -> Result<(), TerminalError>;
        async fn close(&self, id: TerminalSessionId) -> Result<(), TerminalError>;
    }
}

mock! {
    pub WorkspacePath {}

    #[async_trait::async_trait]
    impl WorkspacePathPort for WorkspacePath {
        async fn resolve(&self, workspace_id: Uuid) -> Result<WorkspaceTarget, TerminalError>;
    }
}
