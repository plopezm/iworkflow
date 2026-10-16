use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_lane::{JiraLane, LaneDefinition};

#[async_trait::async_trait]
pub trait JiraLaneRepositoryPort: Send + Sync {
    /// All lanes in display order: the default lane first, then custom ones by creation.
    async fn list(&self) -> Result<Vec<JiraLane>, JiraError>;
    async fn find(&self, id: &str) -> Result<Option<JiraLane>, JiraError>;
    /// Appends a new custom lane (expanded) and returns it.
    async fn create(&self, definition: &LaneDefinition) -> Result<JiraLane, JiraError>;
    /// Renames / re-queries a custom lane; the default lane is not editable.
    async fn update(&self, id: &str, definition: &LaneDefinition) -> Result<JiraLane, JiraError>;
    /// Removes a custom lane together with its issue membership, and the `New` issues no
    /// other lane contains.
    async fn delete(&self, id: &str) -> Result<(), JiraError>;
    async fn set_collapsed(&self, id: &str, collapsed: bool) -> Result<(), JiraError>;
}
