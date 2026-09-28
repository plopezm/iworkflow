use std::path::Path;

use crate::modules::workspaces::domain::errors::workspace_error::WorkspaceError;
use crate::modules::workspaces::domain::models::workspace::{Workspace, WorkspaceId};

#[async_trait::async_trait]
pub trait WorkspaceRepositoryPort: Send + Sync {
    async fn save(&self, workspace: &Workspace) -> Result<(), WorkspaceError>;
    async fn list(&self) -> Result<Vec<Workspace>, WorkspaceError>;
    async fn find_by_id(&self, id: WorkspaceId) -> Result<Option<Workspace>, WorkspaceError>;
    async fn find_by_path(&self, path: &Path) -> Result<Option<Workspace>, WorkspaceError>;
    /// Devuelve `NotFound` si no existe e `InUse` si hay tareas que lo referencian.
    async fn delete(&self, id: WorkspaceId) -> Result<(), WorkspaceError>;
}
