use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::ports::outbound::jira_lane_repository_port::JiraLaneRepositoryPort;

/// Remembers whether a lane (any, including the default one) is folded away.
pub struct SetLaneCollapsed {
    lanes: Arc<dyn JiraLaneRepositoryPort>,
}

impl SetLaneCollapsed {
    pub fn new(lanes: Arc<dyn JiraLaneRepositoryPort>) -> Self {
        Self { lanes }
    }

    pub async fn execute(&self, id: &str, collapsed: bool) -> Result<(), JiraError> {
        self.lanes.set_collapsed(id, collapsed).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::mocks::MockJiraLaneRepository;

    #[tokio::test]
    async fn forwards_the_flag() {
        let mut lanes = MockJiraLaneRepository::new();
        lanes
            .expect_set_collapsed()
            .withf(|id, collapsed| id == "mine" && *collapsed)
            .times(1)
            .returning(|_, _| Ok(()));
        SetLaneCollapsed::new(Arc::new(lanes))
            .execute("mine", true)
            .await
            .expect("set");
    }
}
