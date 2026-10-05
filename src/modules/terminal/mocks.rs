use std::path::{Path, PathBuf};

use mockall::mock;
use tokio::sync::mpsc::Receiver;
use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;
use crate::modules::terminal::domain::ports::outbound::pty_port::PtyPort;
use crate::modules::terminal::domain::ports::outbound::workspace_path_port::WorkspacePathPort;

mock! {
    pub Pty {}

    #[async_trait::async_trait]
    impl PtyPort for Pty {
        async fn spawn(&self, cwd: &Path, size: TerminalSize) -> Result<TerminalSessionId, TerminalError>;
        async fn write(&self, id: TerminalSessionId, bytes: Vec<u8>) -> Result<(), TerminalError>;
        async fn resize(&self, id: TerminalSessionId, size: TerminalSize) -> Result<(), TerminalError>;
        async fn close(&self, id: TerminalSessionId) -> Result<(), TerminalError>;
        async fn subscribe(&self, id: TerminalSessionId) -> Result<Receiver<Vec<u8>>, TerminalError>;
    }
}

mock! {
    pub WorkspacePath {}

    #[async_trait::async_trait]
    impl WorkspacePathPort for WorkspacePath {
        async fn resolve(&self, workspace_id: Uuid) -> Result<PathBuf, TerminalError>;
    }
}
