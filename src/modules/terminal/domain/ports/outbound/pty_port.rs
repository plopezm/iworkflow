use std::path::Path;

use tokio::sync::mpsc::Receiver;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;

#[async_trait::async_trait]
pub trait PtyPort: Send + Sync {
    async fn spawn(
        &self,
        cwd: &Path,
        size: TerminalSize,
    ) -> Result<TerminalSessionId, TerminalError>;
    async fn write(&self, id: TerminalSessionId, bytes: Vec<u8>) -> Result<(), TerminalError>;
    async fn resize(&self, id: TerminalSessionId, size: TerminalSize) -> Result<(), TerminalError>;
    async fn close(&self, id: TerminalSessionId) -> Result<(), TerminalError>;
    async fn subscribe(&self, id: TerminalSessionId) -> Result<Receiver<Vec<u8>>, TerminalError>;
}
