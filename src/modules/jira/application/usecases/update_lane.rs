use std::sync::Arc;

use crate::modules::jira::application::usecases::create_lane::SavedLane;
use crate::modules::jira::application::usecases::sync_issues::SyncIssues;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_lane::{JiraLane, LaneDefinition};
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;

/// Renames or re-queries a custom lane; a changed query is synced right away.
pub struct UpdateLane {
    lanes: Arc<dyn JiraLaneRepositoryPort>,
    sync: Arc<SyncIssues>,
}

impl UpdateLane {
    pub fn new(lanes: Arc<dyn JiraLaneRepositoryPort>, sync: Arc<SyncIssues>) -> Self {
        Self { lanes, sync }
    }

    pub async fn execute(&self, id: &str, name: &str, jql: &str) -> Result<SavedLane, JiraError> {
        let definition: LaneDefinition = LaneDefinition::new(name, jql)?;
        let current: JiraLane = self
            .lanes
            .find(id)
            .await?
            .ok_or_else(|| JiraError::LaneNotFound(id.to_string()))?;
        if current.is_default {
            return Err(JiraError::DefaultLaneImmutable);
        }
        let lane: JiraLane = self.lanes.update(id, &definition).await?;
        let sync_error: Option<String> = if lane.jql == current.jql {
            None
        } else {
            self.sync
                .execute_lane(&lane.id)
                .await
                .err()
                .map(|e: JiraError| e.to_string())
        };
        Ok(SavedLane { lane, sync_error })
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_connection::JiraConnection;
    use crate::modules::jira::domain::models::jira_lane::sample_lane;
    use crate::modules::jira::mocks::{
        MockJiraClient, MockJiraConnectionRepository, MockJiraIssueRepository,
        MockJiraLaneRepository,
    };

    fn use_case(client: MockJiraClient, lanes: MockJiraLaneRepository) -> UpdateLane {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| {
            Ok(Some(
                JiraConnection::new("https://jira.local", "t", false).expect("valid"),
            ))
        });
        connections.expect_record_sync().returning(|_, _| Ok(()));
        let mut issues = MockJiraIssueRepository::new();
        issues.expect_replace_synced().returning(|_, _| Ok(0));
        let lanes: Arc<MockJiraLaneRepository> = Arc::new(lanes);
        let sync: Arc<SyncIssues> = Arc::new(SyncIssues::new(
            Arc::new(connections),
            Arc::new(issues),
            lanes.clone(),
            Arc::new(client),
        ));
        UpdateLane::new(lanes, sync)
    }

    #[tokio::test]
    async fn a_changed_query_is_synced() {
        let mut lanes = MockJiraLaneRepository::new();
        let mut seq = mockall::Sequence::new();
        // First lookup: the lane as stored; second one (by the sync): the lane as updated.
        lanes
            .expect_find()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|id| Ok(Some(sample_lane(id))));
        lanes
            .expect_update()
            .in_sequence(&mut seq)
            .returning(|id, d| {
                let mut lane: JiraLane = sample_lane(id);
                lane.name = d.name.clone();
                lane.jql = d.jql.clone();
                Ok(lane)
            });
        lanes
            .expect_find()
            .times(1)
            .in_sequence(&mut seq)
            .returning(|id| {
                let mut lane: JiraLane = sample_lane(id);
                lane.jql = "project = NEW".to_string();
                Ok(Some(lane))
            });
        let mut client = MockJiraClient::new();
        client
            .expect_search()
            .withf(|_, jql| jql == "project = NEW")
            .times(1)
            .returning(|_, _| Ok(vec![]));

        let saved: SavedLane = use_case(client, lanes)
            .execute("l1", "Nuevo", "project = NEW")
            .await
            .expect("updated");
        assert_eq!(saved.lane.name, "Nuevo");
    }

    #[tokio::test]
    async fn a_rename_alone_does_not_hit_jira() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes
            .expect_find()
            .returning(|id| Ok(Some(sample_lane(id))));
        lanes.expect_update().returning(|id, d| {
            let mut lane: JiraLane = sample_lane(id);
            lane.name = d.name.clone();
            Ok(lane)
        });
        let mut client = MockJiraClient::new();
        client.expect_search().never();

        use_case(client, lanes)
            .execute("l1", "Otro nombre", "project = X")
            .await
            .expect("renamed");
    }

    #[tokio::test]
    async fn the_default_lane_cannot_be_edited() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_find().returning(|id| {
            let mut lane: JiraLane = sample_lane(id);
            lane.is_default = true;
            Ok(Some(lane))
        });
        lanes.expect_update().never();
        let result = use_case(MockJiraClient::new(), lanes)
            .execute("mine", "x", "a = b")
            .await;
        assert_eq!(result, Err(JiraError::DefaultLaneImmutable));
    }

    #[tokio::test]
    async fn an_unknown_lane_is_reported() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_find().returning(|_| Ok(None));
        let result = use_case(MockJiraClient::new(), lanes)
            .execute("zz", "x", "a = b")
            .await;
        assert_eq!(result, Err(JiraError::LaneNotFound("zz".to_string())));
    }
}
