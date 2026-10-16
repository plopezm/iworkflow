use std::collections::HashSet;
use std::sync::Arc;

use chrono::{DateTime, Utc};

use crate::modules::jira::application::usecases::get_connection::require_connection;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::models::jira_issue::RemoteIssue;
use crate::modules::jira::domain::models::jira_lane::{JiraLane, LaneBatch};
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_issue_repository_port::JiraIssueRepositoryPort;
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SyncOutcome {
    /// Distinct issues returned by Jira for the synced lanes.
    pub total: usize,
    /// Issues that were not imported before.
    pub added: usize,
    /// One message per lane whose query failed; those lanes keep their previous issues.
    pub failed_lanes: Vec<String>,
}

/// Imports the issues of every lane (each one a JQL query) and records the outcome on the
/// connection. A lane whose query fails does not prevent the others from syncing.
pub struct SyncIssues {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    issues: Arc<dyn JiraIssueRepositoryPort>,
    lanes: Arc<dyn JiraLaneRepositoryPort>,
    client: Arc<dyn JiraClientPort>,
}

impl SyncIssues {
    pub fn new(
        connections: Arc<dyn JiraConnectionRepositoryPort>,
        issues: Arc<dyn JiraIssueRepositoryPort>,
        lanes: Arc<dyn JiraLaneRepositoryPort>,
        client: Arc<dyn JiraClientPort>,
    ) -> Self {
        Self {
            connections,
            issues,
            lanes,
            client,
        }
    }

    /// Syncs every lane.
    pub async fn execute(&self) -> Result<SyncOutcome, JiraError> {
        let connection: JiraConnection = require_connection(&self.connections).await?;
        let lanes: Vec<JiraLane> = self.lanes.list().await?;
        self.run(&connection, lanes).await
    }

    /// Syncs a single lane (e.g. right after it was created or its query changed).
    pub async fn execute_lane(&self, lane_id: &str) -> Result<SyncOutcome, JiraError> {
        let connection: JiraConnection = require_connection(&self.connections).await?;
        let lane: JiraLane = self
            .lanes
            .find(lane_id)
            .await?
            .ok_or_else(|| JiraError::LaneNotFound(lane_id.to_string()))?;
        self.run(&connection, vec![lane]).await
    }

    async fn run(
        &self,
        connection: &JiraConnection,
        lanes: Vec<JiraLane>,
    ) -> Result<SyncOutcome, JiraError> {
        let now: DateTime<Utc> = Utc::now();
        let mut batches: Vec<LaneBatch> = Vec::new();
        let mut failures: Vec<JiraError> = Vec::new();
        let mut failed_lanes: Vec<String> = Vec::new();
        for lane in lanes {
            match self.client.search(connection, &lane.jql).await {
                Ok(issues) => batches.push(LaneBatch {
                    lane_id: lane.id,
                    issues,
                }),
                Err(error) => {
                    failed_lanes.push(format!("«{}»: {error}", lane.name));
                    failures.push(error);
                }
            }
        }
        let summary: Option<String> =
            Some(failed_lanes.join(" · ")).filter(|s: &String| !s.is_empty());
        if batches.is_empty()
            && let Some(first) = failures.into_iter().next()
        {
            // Keep the failure visible on the connection; the original error wins over a
            // bookkeeping failure.
            let _ = self.connections.record_sync(now, summary).await;
            return Err(first);
        }
        let distinct: HashSet<&str> = batches
            .iter()
            .flat_map(|batch: &LaneBatch| batch.issues.iter())
            .map(|issue: &RemoteIssue| issue.key.as_str())
            .collect();
        let total: usize = distinct.len();
        let added: usize = self.issues.replace_synced(&batches, now).await?;
        // The issues are already stored: failing to note the sync time must not fail the sync.
        let _ = self.connections.record_sync(now, summary).await;
        Ok(SyncOutcome {
            total,
            added,
            failed_lanes,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_issue::sample_remote;
    use crate::modules::jira::domain::models::jira_lane::sample_lane;
    use crate::modules::jira::mocks::{
        MockJiraClient, MockJiraConnectionRepository, MockJiraIssueRepository,
        MockJiraLaneRepository,
    };

    fn connected() -> MockJiraConnectionRepository {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| {
            Ok(Some(
                JiraConnection::new("https://jira.local", "t", false).expect("valid"),
            ))
        });
        connections
    }

    fn lanes_returning(lanes: Vec<JiraLane>) -> MockJiraLaneRepository {
        let mut repository = MockJiraLaneRepository::new();
        repository
            .expect_list()
            .returning(move || Ok(lanes.clone()));
        repository
    }

    fn sync(
        connections: MockJiraConnectionRepository,
        issues: MockJiraIssueRepository,
        lanes: MockJiraLaneRepository,
        client: MockJiraClient,
    ) -> SyncIssues {
        SyncIssues::new(
            Arc::new(connections),
            Arc::new(issues),
            Arc::new(lanes),
            Arc::new(client),
        )
    }

    #[tokio::test]
    async fn imports_each_lane_with_its_own_jql_and_records_success() {
        let mut connections = connected();
        connections
            .expect_record_sync()
            .withf(|_, error| error.is_none())
            .times(1)
            .returning(|_, _| Ok(()));
        let mut mine = sample_lane("mine");
        mine.jql = "mine-jql".to_string();
        let mut other = sample_lane("l2");
        other.jql = "other-jql".to_string();
        let mut client = MockJiraClient::new();
        client.expect_search().returning(|_, jql| {
            Ok(match jql {
                "mine-jql" => vec![sample_remote("A-1"), sample_remote("A-2")],
                _ => vec![sample_remote("A-2"), sample_remote("A-3")],
            })
        });
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_replace_synced()
            .withf(|batches, _| {
                batches.len() == 2
                    && batches[0].lane_id == "mine"
                    && batches[0].issues.len() == 2
                    && batches[1].lane_id == "l2"
            })
            .returning(|_, _| Ok(3));

        let outcome: SyncOutcome = sync(
            connections,
            issues,
            lanes_returning(vec![mine, other]),
            client,
        )
        .execute()
        .await
        .expect("sync");
        assert_eq!(
            outcome,
            SyncOutcome {
                total: 3,
                added: 3,
                failed_lanes: vec![]
            }
        );
    }

    #[tokio::test]
    async fn a_failing_lane_does_not_block_the_others() {
        let mut connections = connected();
        connections
            .expect_record_sync()
            .withf(|_, error| {
                error
                    .as_deref()
                    .is_some_and(|e| e.contains("«Carril l2»") && e.contains("JQL inválido"))
            })
            .times(1)
            .returning(|_, _| Ok(()));
        let mut broken = sample_lane("l2");
        broken.jql = "broken".to_string();
        let mut client = MockJiraClient::new();
        client.expect_search().returning(|_, jql| {
            if jql == "broken" {
                Err(JiraError::Remote("JQL inválido".to_string()))
            } else {
                Ok(vec![sample_remote("A-1")])
            }
        });
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_replace_synced()
            .withf(|batches, _| batches.len() == 1 && batches[0].lane_id == "mine")
            .returning(|_, _| Ok(1));

        let outcome: SyncOutcome = sync(
            connections,
            issues,
            lanes_returning(vec![sample_lane("mine"), broken]),
            client,
        )
        .execute()
        .await
        .expect("partial sync");
        assert_eq!(outcome.total, 1);
        assert_eq!(outcome.failed_lanes.len(), 1);
        assert!(outcome.failed_lanes[0].starts_with("«Carril l2»"));
    }

    #[tokio::test]
    async fn when_every_lane_fails_the_error_is_recorded_and_returned_without_touching_issues() {
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

        let result = sync(
            connections,
            issues,
            lanes_returning(vec![sample_lane("mine")]),
            client,
        )
        .execute()
        .await;
        assert_eq!(result, Err(JiraError::Remote("caído".to_string())));
    }

    #[tokio::test]
    async fn without_a_connection_nothing_is_called() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| Ok(None));
        let result = sync(
            connections,
            MockJiraIssueRepository::new(),
            MockJiraLaneRepository::new(),
            MockJiraClient::new(),
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

        let outcome: SyncOutcome = sync(
            connections,
            issues,
            lanes_returning(vec![sample_lane("mine")]),
            client,
        )
        .execute()
        .await
        .expect("synced");
        assert_eq!(outcome.added, 1);
    }

    #[tokio::test]
    async fn syncing_one_lane_only_queries_that_lane() {
        let mut connections = connected();
        connections.expect_record_sync().returning(|_, _| Ok(()));
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_list().never();
        lanes
            .expect_find()
            .withf(|id| id == "l2")
            .returning(|_| Ok(Some(sample_lane("l2"))));
        let mut client = MockJiraClient::new();
        client
            .expect_search()
            .times(1)
            .returning(|_, _| Ok(vec![sample_remote("B-1")]));
        let mut issues = MockJiraIssueRepository::new();
        issues
            .expect_replace_synced()
            .withf(|batches, _| batches.len() == 1 && batches[0].lane_id == "l2")
            .returning(|_, _| Ok(1));

        let outcome: SyncOutcome = sync(connections, issues, lanes, client)
            .execute_lane("l2")
            .await
            .expect("lane sync");
        assert_eq!(outcome.total, 1);
    }

    #[tokio::test]
    async fn syncing_an_unknown_lane_fails() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_find().returning(|_| Ok(None));
        let result = sync(
            connected(),
            MockJiraIssueRepository::new(),
            lanes,
            MockJiraClient::new(),
        )
        .execute_lane("nope")
        .await;
        assert_eq!(result, Err(JiraError::LaneNotFound("nope".to_string())));
    }
}
