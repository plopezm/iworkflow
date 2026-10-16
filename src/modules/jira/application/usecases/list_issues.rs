use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_filter::IssueFilter;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

pub struct ListIssues {
    issues: Arc<dyn JiraIssueRepositoryPort>,
}

impl ListIssues {
    pub fn new(issues: Arc<dyn JiraIssueRepositoryPort>) -> Self {
        Self { issues }
    }

    pub async fn execute(&self, filter: IssueFilter) -> Result<Vec<JiraIssue>, JiraError> {
        Ok(filter.apply(self.issues.list().await?))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;
    use crate::modules::jira::mocks::MockJiraIssueRepository;

    #[tokio::test]
    async fn applies_the_filter_keeping_repository_order() {
        let mut ignored: JiraIssue = sample_issue("A-2");
        ignored.local_state = IssueLocalState::Ignored;
        let all: Vec<JiraIssue> = vec![sample_issue("A-1"), ignored, sample_issue("A-3")];
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_list().returning(move || Ok(all.clone()));

        let result: Vec<JiraIssue> = ListIssues::new(Arc::new(issues))
            .execute(IssueFilter {
                local_state: Some(IssueLocalState::New),
                ..Default::default()
            })
            .await
            .expect("list");
        let keys: Vec<&str> = result.iter().map(|i: &JiraIssue| i.key()).collect();
        assert_eq!(keys, vec!["A-1", "A-3"]);
    }
}
