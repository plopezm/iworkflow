use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::WorkspaceId;

/// Borra en cascada las tareas de un workspace, previo a eliminarlo.
#[async_trait::async_trait]
pub trait WorkspaceTasksPort: Send + Sync {
    /// Borra todas las tareas del workspace. Devuelve `WorkspaceError::InUse` si
    /// alguna sigue activa.
    async fn delete_all(&self, id: WorkspaceId) -> Result<(), WorkspaceError>;
}
