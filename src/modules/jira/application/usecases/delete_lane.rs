use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_lane::JiraLane;
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;

/// Removes a custom lane. Issues also present in another lane, converted or ignored are kept.
pub struct DeleteLane {
    lanes: Arc<dyn JiraLaneRepositoryPort>,
}

impl DeleteLane {
    pub fn new(lanes: Arc<dyn JiraLaneRepositoryPort>) -> Self {
        Self { lanes }
    }

    pub async fn execute(&self, id: &str) -> Result<(), JiraError> {
        let lane: JiraLane = self
            .lanes
            .find(id)
            .await?
            .ok_or_else(|| JiraError::LaneNotFound(id.to_string()))?;
        if lane.is_default {
            return Err(JiraError::DefaultLaneImmutable);
        }
        self.lanes.delete(id).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::domain::models::jira_lane::sample_lane;
    use crate::modules::jira::mocks::MockJiraLaneRepository;

    #[tokio::test]
    async fn deletes_a_custom_lane() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes
            .expect_find()
            .returning(|id| Ok(Some(sample_lane(id))));
        lanes
            .expect_delete()
            .withf(|id| id == "l1")
            .times(1)
            .returning(|_| Ok(()));
        DeleteLane::new(Arc::new(lanes))
            .execute("l1")
            .await
            .expect("deleted");
    }

    #[tokio::test]
    async fn refuses_the_default_lane_and_unknown_ids() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes.expect_find().returning(|id| {
            Ok((id == "mine").then(|| JiraLane {
                is_default: true,
                ..sample_lane("mine")
            }))
        });
        lanes.expect_delete().never();
        let use_case: DeleteLane = DeleteLane::new(Arc::new(lanes));
        assert_eq!(
            use_case.execute("mine").await,
            Err(JiraError::DefaultLaneImmutable)
        );
        assert_eq!(
            use_case.execute("zz").await,
            Err(JiraError::LaneNotFound("zz".to_string()))
        );
    }
}
