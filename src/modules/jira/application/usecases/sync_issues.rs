use std::sync::Arc;

use chrono::{DateTime, Utc};

use crate::modules::jira::application::usecases::get_connection::require_connection;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::models::jira_issue::RemoteIssue;
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SyncOutcome {
    /// Issues returned by Jira for the connection's JQL.
    pub total: usize,
    /// Issues that were not imported before.
    pub added: usize,
}

/// Imports the issues matching the connection's JQL and records the outcome on the connection.
pub struct SyncIssues {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    issues: Arc<dyn JiraIssueRepositoryPort>,
    client: Arc<dyn JiraClientPort>,
}

impl SyncIssues {
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

    pub async fn execute(&self) -> Result<SyncOutcome, JiraError> {
        let connection: JiraConnection = require_connection(&self.connections).await?;
        let now: DateTime<Utc> = Utc::now();
        let fetched: Result<Vec<RemoteIssue>, JiraError> =
            self.client.search(&connection, &connection.jql).await;
        let remote: Vec<RemoteIssue> = match fetched {
            Ok(remote) => remote,
            Err(error) => {
                // Keep the failure visible on the connection; the original error wins over a
                // bookkeeping failure.
                let _ = self
                    .connections
                    .record_sync(now, Some(error.to_string()))
                    .await;
                return Err(error);
            }
        };
        let added: usize = self.issues.replace_synced(&remote, now).await?;
        // The issues are already stored: failing to note the sync time must not fail the sync.
        let _ = self.connections.record_sync(now, None).await;
        Ok(SyncOutcome {
            total: remote.len(),
            added,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_remote;
    use crate::modules::jira::mocks::{
        MockJiraClient, MockJiraConnectionRepository, MockJiraIssueRepository,
    };

    fn connected() -> MockJiraConnectionRepository {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| {
            Ok(Some(
                JiraConnection::new("https://jira.local", "t", "project = X", false)
                    .expect("valid"),
            ))
        });
        connections
    }

    #[tokio::test]
    async fn imports_with_the_connection_jql_and_records_success() {
        let mut connections = connected();
        connections
            .expect_record_sync()
            .withf(|_, error| error.is_none())
            .times(1)
            .returning(|_, _| Ok(()));
        let mut client = MockJiraClient::new();
        client
            .expect_search()
            .withf(|_, jql| jql == "project = X")
            .returning(|_, _| Ok(vec![sample_remote("A-1"), sample_remote("A-2")]));
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_replace_synced()
            .withf(|remote, _| remote.len() == 2)
            .returning(|_, _| Ok(1));

        let outcome: SyncOutcome =
            SyncIssues::new(Arc::new(connections), Arc::new(issues), Arc::new(client))
                .execute()
                .await
                .expect("sync");
        assert_eq!(outcome, SyncOutcome { total: 2, added: 1 });
    }

    #[tokio::test]
    async fn a_failing_search_is_recorded_and_returned_without_touching_issues() {
        let mut connections = connected();
        connections
            .expect_record_sync()
            .withf(|_, error| error.as_deref().is_some_and(|e| e.contains("caído")))
            .times(1)
            .returning(|_, _| Ok(()));
        let mut client = MockJiraClient::new();
        client
            .expect_search()
            .returning(|_, _| Err(JiraError::Remote("caído".to_string())));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_replace_synced().never();

        let result = SyncIssues::new(Arc::new(connections), Arc::new(issues), Arc::new(client))
            .execute()
            .await;
        assert_eq!(result, Err(JiraError::Remote("caído".to_string())));
    }

    #[tokio::test]
    async fn without_a_connection_nothing_is_called() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| Ok(None));
        let result = SyncIssues::new(
            Arc::new(connections),
            Arc::new(MockJiraIssueRepository::new()),
            Arc::new(MockJiraClient::new()),
        )
        .execute()
        .await;
        assert_eq!(result, Err(JiraError::NotConfigured));
    }

    #[tokio::test]
    async fn a_bookkeeping_failure_after_storing_the_issues_does_not_fail_the_sync() {
        let mut connections = connected();
        connections
            .expect_record_sync()
            .returning(|_, _| Err(JiraError::Repository("bloqueada".to_string())));
        let mut client = MockJiraClient::new();
        client
            .expect_search()
            .returning(|_, _| Ok(vec![sample_remote("A-1")]));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_replace_synced().returning(|_, _| Ok(1));

        let outcome: SyncOutcome =
            SyncIssues::new(Arc::new(connections), Arc::new(issues), Arc::new(client))
                .execute()
                .await
                .expect("synced");
        assert_eq!(outcome, SyncOutcome { total: 1, added: 1 });
    }
}
