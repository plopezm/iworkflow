use std::collections::HashMap;

use chrono::{DateTime, Utc};

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::issue_local_state::IssueLocalState;
use crate::modules::jira::domain::models::jira_issue::{JiraIssue, RemoteIssue};
use crate::modules::jira::domain::models::jira_lane::LaneBatch;

#[async_trait::async_trait]
pub trait JiraIssueRepositoryPort: Send + Sync {
    /// All imported issues, most recently updated in Jira first.
    async fn list(&self) -> Result<Vec<JiraIssue>, JiraError>;
    async fn find(&self, key: &str) -> Result<Option<JiraIssue>, JiraError>;
    /// Issue keys of every lane, keyed by lane id.
    async fn lane_memberships(&self) -> Result<HashMap<String, Vec<String>>, JiraError>;
    /// Stores the sync result atomically: inserts new issues, refreshes the Jira data of known
    /// ones (keeping their local state), replaces the membership of the lanes in `batches`
    /// (other lanes are untouched) and removes `New` issues that no lane contains any more.
    /// Returns how many issues were not stored before.
    async fn replace_synced(
        &self,
        batches: &[LaneBatch],
        synced_at: DateTime<Utc>,
    ) -> Result<usize, JiraError>;
    /// Refreshes the Jira data of one known issue, keeping its local state.
    async fn update_remote(&self, issue: &RemoteIssue, at: DateTime<Utc>) -> Result<(), JiraError>;
    async fn set_local_state(
        &self,
        key: &str,
        state: IssueLocalState,
        task_id: Option<String>,
    ) -> Result<(), JiraError>;
    async fn delete_all(&self) -> Result<(), JiraError>;
}
