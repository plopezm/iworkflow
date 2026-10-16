use chrono::{DateTime, Utc};

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;

#[async_trait::async_trait]
pub trait JiraConnectionRepositoryPort: Send + Sync {
    async fn get(&self) -> Result<Option<JiraConnection>, JiraError>;
    /// Inserts or replaces the single connection. Sync bookkeeping is reset only when the
    /// server URL changes.
    async fn save(&self, connection: &JiraConnection) -> Result<(), JiraError>;
    async fn delete(&self) -> Result<(), JiraError>;
    /// Records the outcome of the last sync (`error: None` on success).
    async fn record_sync(&self, at: DateTime<Utc>, error: Option<String>) -> Result<(), JiraError>;
}
