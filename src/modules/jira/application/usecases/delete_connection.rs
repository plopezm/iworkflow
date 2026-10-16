use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

/// Disconnects from Jira: forgets the token and the imported issues. Converted iworkflow tasks stay.
pub struct DeleteConnection {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    issues: Arc<dyn JiraIssueRepositoryPort>,
}

impl DeleteConnection {
    pub fn new(
        connections: Arc<dyn JiraConnectionRepositoryPort>,
        issues: Arc<dyn JiraIssueRepositoryPort>,
    ) -> Self {
        Self {
            connections,
            issues,
        }
    }

    pub async fn execute(&self) -> Result<(), JiraError> {
        self.issues.delete_all().await?;
        self.connections.delete().await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::mocks::{MockJiraConnectionRepository, MockJiraIssueRepository};

    #[tokio::test]
    async fn removes_issues_and_connection() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_delete().times(1).returning(|| Ok(()));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_delete_all().times(1).returning(|| Ok(()));
        DeleteConnection::new(Arc::new(connections), Arc::new(issues))
            .execute()
            .await
            .expect("deleted");
    }

    #[tokio::test]
    async fn keeps_the_connection_if_clearing_issues_fails() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_delete().never();
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_delete_all()
            .returning(|| Err(JiraError::Repository("x".to_string())));
        let result = DeleteConnection::new(Arc::new(connections), Arc::new(issues))
            .execute()
            .await;
        assert_eq!(result, Err(JiraError::Repository("x".to_string())));
    }
}
