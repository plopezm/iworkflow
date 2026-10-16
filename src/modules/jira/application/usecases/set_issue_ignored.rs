use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

/// Marks an issue as ignored, or makes it `New` again (also unlinking a converted task so it can
/// be converted anew). Local only: nothing is written to Jira.
pub struct SetIssueIgnored {
    issues: Arc<dyn JiraIssueRepositoryPort>,
}

impl SetIssueIgnored {
    pub fn new(issues: Arc<dyn JiraIssueRepositoryPort>) -> Self {
        Self { issues }
    }

    pub async fn execute(&self, key: &str, ignored: bool) -> Result<(), JiraError> {
        let issue: JiraIssue = self
            .issues
            .find(key)
            .await?
            .ok_or_else(|| JiraError::IssueNotFound(key.to_string()))?;
        if ignored {
            if issue.local_state == IssueLocalState::Converted {
                return Err(JiraError::AlreadyConverted(key.to_string()));
            }
            self.issues
                .set_local_state(key, IssueLocalState::Ignored, None)
                .await
        } else {
            self.issues
                .set_local_state(key, IssueLocalState::New, None)
                .await
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_issue;
    use crate::modules::jira::mocks::MockJiraIssueRepository;

    fn repo_with(state: IssueLocalState) -> MockJiraIssueRepository {
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_find().returning(move |_| {
            let mut issue = sample_issue("A-1");
            issue.local_state = state;
            Ok(Some(issue))
        });
        issues
    }

    #[tokio::test]
    async fn ignores_a_new_issue() {
        let mut issues = repo_with(IssueLocalState::New);
        issues
            .expect_set_local_state()
            .withf(|key, state, task| {
                key == "A-1" && *state == IssueLocalState::Ignored && task.is_none()
            })
            .times(1)
            .returning(|_, _, _| Ok(()));
        SetIssueIgnored::new(Arc::new(issues))
            .execute("A-1", true)
            .await
            .expect("ok");
    }

    #[tokio::test]
    async fn restoring_makes_it_new_and_clears_the_link() {
        let mut issues = repo_with(IssueLocalState::Converted);
        issues
            .expect_set_local_state()
            .withf(|_, state, task| *state == IssueLocalState::New && task.is_none())
            .times(1)
            .returning(|_, _, _| Ok(()));
        SetIssueIgnored::new(Arc::new(issues))
            .execute("A-1", false)
            .await
            .expect("ok");
    }

    #[tokio::test]
    async fn a_converted_issue_cannot_be_ignored() {
        let mut issues = repo_with(IssueLocalState::Converted);
        issues.expect_set_local_state().never();
        let result = SetIssueIgnored::new(Arc::new(issues))
            .execute("A-1", true)
            .await;
        assert_eq!(result, Err(JiraError::AlreadyConverted("A-1".to_string())));
    }
}
