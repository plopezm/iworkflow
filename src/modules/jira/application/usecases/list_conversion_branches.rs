use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::workspace_option::BranchOption;
use crate::modules::jira::domain::ports::outbound::workspace_catalog_port::WorkspaceCatalogPort;

/// Branches of the chosen workspace a converted task can start from.
pub struct ListConversionBranches {
    workspaces: Arc<dyn WorkspaceCatalogPort>,
}

impl ListConversionBranches {
    pub fn new(workspaces: Arc<dyn WorkspaceCatalogPort>) -> Self {
        Self { workspaces }
    }

    pub async fn execute(&self, workspace_id: &str) -> Result<Vec<BranchOption>, JiraError> {
        if workspace_id.trim().is_empty() {
            return Ok(Vec::new());
        }
        self.workspaces.branches(workspace_id.trim()).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::mocks::MockWorkspaceCatalog;

    #[tokio::test]
    async fn blank_workspace_returns_nothing_without_calling_the_catalog() {
        let result = ListConversionBranches::new(Arc::new(MockWorkspaceCatalog::new()))
            .execute(" ")
            .await
            .expect("empty");
        assert!(result.is_empty());
    }

    #[tokio::test]
    async fn delegates_for_a_real_workspace() {
        let mut catalog = MockWorkspaceCatalog::new();
        catalog
            .expect_branches()
            .withf(|id| id == "w1")
            .returning(|_| {
                Ok(vec![BranchOption {
                    name: "main".to_string(),
                    remote: false,
                }])
            });
        let result = ListConversionBranches::new(Arc::new(catalog))
            .execute("w1")
            .await
            .expect("branches");
        assert_eq!(result[0].name, "main");
    }
}
