use std::path::PathBuf;

use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;

#[async_trait::async_trait]
pub trait WorkspacePathPort: Send + Sync {
    /// Devuelve el directorio del workspace o `WorkspaceNotFound`.
    async fn resolve(&self, workspace_id: Uuid) -> Result<PathBuf, TerminalError>;
}
