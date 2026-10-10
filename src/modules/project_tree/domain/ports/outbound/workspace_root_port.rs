use uuid::Uuid;

use crate::modules::project_tree::domain::errors::project_tree_error::ProjectTreeError;
use crate::modules::project_tree::domain::models::workspace_root::WorkspaceRoot;

#[async_trait::async_trait]
pub trait WorkspaceRootPort: Send + Sync {
    /// Returns name and root directory of the workspace or `WorkspaceNotFound`.
    async fn resolve(&self, workspace_id: Uuid) -> Result<WorkspaceRoot, ProjectTreeError>;
}
