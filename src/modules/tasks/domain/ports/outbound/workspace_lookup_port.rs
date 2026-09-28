use crate::modules::tasks::domain::errors::task_error::TaskError;
use crate::modules::tasks::domain::models::workspace_ref::WorkspaceRef;

/// Acceso de solo lectura a los workspaces (implementado en la composition root).
#[async_trait::async_trait]
pub trait WorkspaceLookupPort: Send + Sync {
    async fn find(&self, workspace_id: &str) -> Result<Option<WorkspaceRef>, TaskError>;
    async fn list(&self) -> Result<Vec<WorkspaceRef>, TaskError>;
}
