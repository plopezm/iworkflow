use uuid::Uuid;

use crate::modules::terminal::domain::errors::terminal_error::TerminalError;
use crate::modules::terminal::domain::models::workspace_target::WorkspaceTarget;

#[async_trait::async_trait]
pub trait WorkspacePathPort: Send + Sync {
    /// Devuelve nombre y directorio del workspace o `WorkspaceNotFound`.
    async fn resolve(&self, workspace_id: Uuid) -> Result<WorkspaceTarget, TerminalError>;
}
