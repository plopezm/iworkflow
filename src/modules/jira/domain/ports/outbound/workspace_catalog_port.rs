use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::workspace_option::{BranchOption, WorkspaceOption};

#[async_trait::async_trait]
pub trait WorkspaceCatalogPort: Send + Sync {
    async fn workspaces(&self) -> Result<Vec<WorkspaceOption>, JiraError>;
    async fn branches(&self, workspace_id: &str) -> Result<Vec<BranchOption>, JiraError>;
}
