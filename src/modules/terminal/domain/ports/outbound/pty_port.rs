use std::path::Path;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::terminal_attachment::TerminalAttachment;
use crate::modules::terminal::domain::models::terminal_session::TerminalSession;
use crate::modules::terminal::domain::models::terminal_session_id::TerminalSessionId;
use crate::modules::terminal::domain::models::terminal_size::TerminalSize;

#[async_trait::async_trait]
pub trait PtyPort: Send + Sync {
    /// Crea una sesión persistente; el registro conserva `workspace_id` y `workspace_name`
    /// como metadatos y asigna id, fecha de creación y estado.
    async fn spawn(
        &self,
        workspace_id: Uuid,
        workspace_name: &str,
        cwd: &Path,
        size: TerminalSize,
    ) -> Result<TerminalSession, TerminalError>;
    async fn list(&self) -> Result<Vec<TerminalSession>, TerminalError>;
    /// Devuelve scrollback + salida en vivo; puede llamarse varias veces por sesión.
    async fn attach(&self, id: TerminalSessionId) -> Result<TerminalAttachment, TerminalError>;
    async fn write(&self, id: TerminalSessionId, bytes: Vec<u8>) -> Result<(), TerminalError>;
    async fn resize(&self, id: TerminalSessionId, size: TerminalSize) -> Result<(), TerminalError>;
    async fn close(&self, id: TerminalSessionId) -> Result<(), TerminalError>;
}
