use std::sync::Arc;

use chrono::Utc;

use crate::modules::jira::application::usecases::get_connection::require_connection;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::models::jira_issue::{JiraIssue, RemoteIssue};
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

/// Moves the issue to another status in Jira and refreshes the local copy.
pub struct TransitionIssue {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    issues: Arc<dyn JiraIssueRepositoryPort>,
    client: Arc<dyn JiraClientPort>,
}

impl TransitionIssue {
    pub fn new(
        connections: Arc<dyn JiraConnectionRepositoryPort>,
        issues: Arc<dyn JiraIssueRepositoryPort>,
        client: Arc<dyn JiraClientPort>,
    ) -> Self {
        Self {
            connections,
            issues,
            client,
        }
    }

    pub async fn execute(&self, key: &str, transition_id: &str) -> Result<JiraIssue, JiraError> {
        let connection: JiraConnection = require_connection(&self.connections).await?;
        if self.issues.find(key).await?.is_none() {
            return Err(JiraError::IssueNotFound(key.to_string()));
        }
        self.client
            .transition(&connection, key, transition_id)
            .await?;
        let refreshed: RemoteIssue = self.client.get_issue(&connection, key).await?;
        self.issues.update_remote(&refreshed, Utc::now()).await?;
        self.issues
            .find(key)
            .await?
            .ok_or_else(|| JiraError::IssueNotFound(key.to_string()))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::{sample_issue, sample_remote};
    use crate::modules::jira::mocks::{
        MockJiraClient, MockJiraConnectionRepository, MockJiraIssueRepository,
    };

    fn connected() -> MockJiraConnectionRepository {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| {
            Ok(Some(
                JiraConnection::new("https://j", "t", "", false).expect("valid"),
            ))
        });
        connections
    }

    #[tokio::test]
    async fn transitions_in_jira_then_refreshes_the_local_copy() {
        let mut client = MockJiraClient::new();
        let mut seq = mockall::Sequence::new();
        client
            .expect_transition()
            .withf(|_, key, id| key == "A-1" && id == "31")
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, _, _| Ok(()));
        client
            .expect_get_issue()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|_, key| {
                let mut remote = sample_remote(key);
                remote.status = "Done".to_string();
                Ok(remote)
            });
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_find()
            .returning(|_| Ok(Some(sample_issue("A-1"))));
        issues
            .expect_update_remote()
            .withf(|remote, _| remote.status == "Done")
            .times(1)
            .returning(|_, _| Ok(()));

        TransitionIssue::new(Arc::new(connected()), Arc::new(issues), Arc::new(client))
            .execute("A-1", "31")
            .await
            .expect("transitioned");
    }

    #[tokio::test]
    async fn unknown_issue_never_reaches_jira() {
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_find().returning(|_| Ok(None));
        let mut client = MockJiraClient::new();
        client.expect_transition().never();
        let result =
            TransitionIssue::new(Arc::new(connected()), Arc::new(issues), Arc::new(client))
                .execute("A-1", "31")
                .await;
        assert_eq!(result, Err(JiraError::IssueNotFound("A-1".to_string())));
    }

    #[tokio::test]
    async fn a_jira_failure_leaves_the_local_copy_untouched() {
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_find()
            .returning(|_| Ok(Some(sample_issue("A-1"))));
        issues.expect_update_remote().never();
        let mut client = MockJiraClient::new();
        client
            .expect_transition()
            .returning(|_, _, _| Err(JiraError::Remote("HTTP 400".to_string())));
        let result =
            TransitionIssue::new(Arc::new(connected()), Arc::new(issues), Arc::new(client))
                .execute("A-1", "31")
                .await;
        assert_eq!(result, Err(JiraError::Remote("HTTP 400".to_string())));
    }
}
