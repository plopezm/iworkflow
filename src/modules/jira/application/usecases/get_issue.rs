use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

pub struct GetIssue {
    issues: Arc<dyn JiraIssueRepositoryPort>,
}

impl GetIssue {
    pub fn new(issues: Arc<dyn JiraIssueRepositoryPort>) -> Self {
        Self { issues }
    }

    pub async fn execute(&self, key: &str) -> Result<JiraIssue, JiraError> {
        self.issues
            .find(key)
            .await?
            .ok_or_else(|| JiraError::IssueNotFound(key.to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;
    use crate::modules::jira::mocks::MockJiraIssueRepository;

    #[tokio::test]
    async fn returns_the_issue_or_not_found() {
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_find()
            .returning(|key| Ok((key == "A-1").then(|| sample_issue("A-1"))));
        let usecase: GetIssue = GetIssue::new(Arc::new(issues));
        assert_eq!(usecase.execute("A-1").await.expect("found").key(), "A-1");
        assert_eq!(
            usecase.execute("B-2").await,
            Err(JiraError::IssueNotFound("B-2".to_string()))
        );
    }
}
