use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::conversion::ConversionDraft;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::models::workspace_option::WorkspaceOption;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use crate::modules::jira::domain::ports::outbound::workspace_catalog_port::WorkspaceCatalogPort;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ConversionPreparation {
    pub issue: JiraIssue,
    pub draft: ConversionDraft,
    pub workspaces: Vec<WorkspaceOption>,
}

/// Everything the conversion screen needs: the issue, the suggested task content and the
/// workspaces to map it onto.
pub struct PrepareConversion {
    issues: Arc<dyn JiraIssueRepositoryPort>,
    workspaces: Arc<dyn WorkspaceCatalogPort>,
}

impl PrepareConversion {
    pub fn new(
        issues: Arc<dyn JiraIssueRepositoryPort>,
        workspaces: Arc<dyn WorkspaceCatalogPort>,
    ) -> Self {
        Self { issues, workspaces }
    }

    pub async fn execute(&self, key: &str) -> Result<ConversionPreparation, JiraError> {
        let issue: JiraIssue = self
            .issues
            .find(key)
            .await?
            .ok_or_else(|| JiraError::IssueNotFound(key.to_string()))?;
        let workspaces: Vec<WorkspaceOption> = self.workspaces.workspaces().await?;
        Ok(ConversionPreparation {
            draft: ConversionDraft::from_issue(&issue),
            issue,
            workspaces,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;
    use crate::modules::jira::mocks::{MockJiraIssueRepository, MockWorkspaceCatalog};

    #[tokio::test]
    async fn builds_the_draft_and_lists_workspaces() {
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_find()
            .returning(|_| Ok(Some(sample_issue("A-1"))));
        let mut catalog = MockWorkspaceCatalog::new();
        catalog.expect_workspaces().returning(|| {
            Ok(vec![WorkspaceOption {
                id: "w1".to_string(),
                name: "app".to_string(),
                base_branch: "main".to_string(),
            }])
        });
        let prepared = PrepareConversion::new(Arc::new(issues), Arc::new(catalog))
            .execute("A-1")
            .await
            .expect("prepared");
        assert_eq!(prepared.draft.title, "A-1: Resumen de A-1");
        assert_eq!(prepared.workspaces.len(), 1);
    }

    #[tokio::test]
    async fn unknown_issue_is_not_found() {
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_find().returning(|_| Ok(None));
        let result =
            PrepareConversion::new(Arc::new(issues), Arc::new(MockWorkspaceCatalog::new()))
                .execute("A-1")
                .await;
        assert_eq!(result, Err(JiraError::IssueNotFound("A-1".to_string())));
    }
}
