use std::path::Path;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};

#[async_trait::async_trait]
pub trait WorkspaceRepositoryPort: Send + Sync {
    async fn save(&self, workspace: &Workspace) -> Result<(), WorkspaceError>;
    async fn list(&self) -> Result<Vec<Workspace>, WorkspaceError>;
    async fn find_by_id(&self, id: WorkspaceId) -> Result<Option<Workspace>, WorkspaceError>;
    async fn find_by_path(&self, path: &Path) -> Result<Option<Workspace>, WorkspaceError>;
    /// Devuelve `NotFound` si no existe. Si quedara alguna tarea referenciándolo (el
    /// FK `tasks.workspace_id ON DELETE RESTRICT` como red de seguridad ante carreras;
    /// en el camino normal ya se han borrado vía `WorkspaceTasksPort::delete_all`),
    /// devuelve `InUse`.
    async fn delete(&self, id: WorkspaceId) -> Result<(), WorkspaceError>;
}
