use std::sync::Arc;

use crate::modules::jira::application::usecases::sync_issues::SyncIssues;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_lane::{JiraLane, LaneDefinition};
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;

/// A lane that was stored, plus the failure of its first sync (the lane itself is kept).
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct SavedLane {
    pub lane: JiraLane,
    pub sync_error: Option<String>,
}

/// Adds a custom lane and fills it right away.
pub struct CreateLane {
    lanes: Arc<dyn JiraLaneRepositoryPort>,
    sync: Arc<SyncIssues>,
}

impl CreateLane {
    pub fn new(lanes: Arc<dyn JiraLaneRepositoryPort>, sync: Arc<SyncIssues>) -> Self {
        Self { lanes, sync }
    }

    pub async fn execute(&self, name: &str, jql: &str) -> Result<SavedLane, JiraError> {
        let definition: LaneDefinition = LaneDefinition::new(name, jql)?;
        let lane: JiraLane = self.lanes.create(&definition).await?;
        let sync_error: Option<String> = self
            .sync
            .execute_lane(&lane.id)
            .await
            .err()
            .map(|e: JiraError| e.to_string());
        Ok(SavedLane { lane, sync_error })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_connection::JiraConnection;
    use crate::modules::jira::domain::models::jira_issue::sample_remote;
    use crate::modules::jira::domain::models::jira_lane::sample_lane;
    use crate::modules::jira::mocks::{
        MockJiraClient, MockJiraConnectionRepository, MockJiraIssueRepository,
        MockJiraLaneRepository,
    };

    fn use_case(client: MockJiraClient, mut lanes: MockJiraLaneRepository) -> CreateLane {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| {
            Ok(Some(
                JiraConnection::new("https://jira.local", "t", false).expect("valid"),
            ))
        });
        connections.expect_record_sync().returning(|_, _| Ok(()));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_replace_synced().returning(|_, _| Ok(1));
        lanes
            .expect_find()
            .returning(|id| Ok(Some(sample_lane(id))));
        let lanes: Arc<MockJiraLaneRepository> = Arc::new(lanes);
        let sync: Arc<SyncIssues> = Arc::new(SyncIssues::new(
            Arc::new(connections),
            Arc::new(issues),
            lanes.clone(),
            Arc::new(client),
        ));
        CreateLane::new(lanes, sync)
    }

    #[tokio::test]
    async fn stores_the_trimmed_definition_and_syncs_the_new_lane() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes
            .expect_create()
            .withf(|d| d.name == "Backend" && d.jql == "project = B")
            .times(1)
            .returning(|_| Ok(sample_lane("new")));
        let mut client = MockJiraClient::new();
        client
            .expect_search()
            .times(1)
            .returning(|_, _| Ok(vec![sample_remote("B-1")]));

        let saved: SavedLane = use_case(client, lanes)
            .execute(" Backend ", " project = B ")
            .await
            .expect("created");
        assert_eq!(saved.lane.id, "new");
        assert_eq!(saved.sync_error, None);
    }

    #[tokio::test]
    async fn a_failing_first_sync_keeps_the_lane_and_reports_the_error() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_create().returning(|_| Ok(sample_lane("new")));
        let mut client = MockJiraClient::new();
        client
            .expect_search()
            .returning(|_, _| Err(JiraError::Remote("JQL inválido".to_string())));

        let saved: SavedLane = use_case(client, lanes)
            .execute("Backend", "bad jql")
            .await
            .expect("created");
        assert!(saved.sync_error.is_some_and(|e| e.contains("JQL inválido")));
    }

    #[tokio::test]
    async fn invalid_input_stores_nothing() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_create().never();
        let result = use_case(MockJiraClient::new(), lanes)
            .execute(" ", "x = y")
            .await;
        assert_eq!(result, Err(JiraError::EmptyLaneName));
    }
}
