use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_filter::IssueFilter;
use crate::modules::jira::domain::models::jira_issue::JiraIssue;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

/// The filtered issues plus what the filter UI needs to offer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct IssueListing {
    pub issues: Vec<JiraIssue>,
    /// Distinct Jira statuses among all imported issues (not only the filtered ones), sorted.
    pub statuses: Vec<String>,
    /// Imported issues before filtering.
    pub total: usize,
}

pub struct ListIssues {
    issues: Arc<dyn JiraIssueRepositoryPort>,
}

impl ListIssues {
    pub fn new(issues: Arc<dyn JiraIssueRepositoryPort>) -> Self {
        Self { issues }
    }

    pub async fn execute(&self, filter: IssueFilter) -> Result<IssueListing, JiraError> {
        let all: Vec<JiraIssue> = self.issues.list().await?;
        let total: usize = all.len();
        let statuses: Vec<String> = all
            .iter()
            .map(|issue: &JiraIssue| issue.remote.status.clone())
            .collect::<std::collections::BTreeSet<String>>()
            .into_iter()
            .collect();
        Ok(IssueListing {
            issues: filter.apply(all),
            statuses,
            total,
        })
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

        let listing: IssueListing = ListIssues::new(Arc::new(issues))
            .execute(IssueFilter {
                local_state: Some(IssueLocalState::New),
                ..Default::default()
            })
            .await
            .expect("list");
        let keys: Vec<&str> = listing.issues.iter().map(|i: &JiraIssue| i.key()).collect();
        assert_eq!(keys, vec!["A-1", "A-3"]);
        assert_eq!(listing.total, 3);
        assert_eq!(listing.statuses, vec!["To Do"]);
    }
}
