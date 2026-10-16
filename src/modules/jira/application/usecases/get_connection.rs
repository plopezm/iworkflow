use std::sync::Arc;

use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_connection::JiraConnection;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;

pub struct GetConnection {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
}

impl GetConnection {
    pub fn new(connections: Arc<dyn JiraConnectionRepositoryPort>) -> Self {
        Self { connections }
    }

    pub async fn execute(&self) -> Result<Option<JiraConnection>, JiraError> {
        self.connections.get().await
    }
}

/// Loads the connection or fails with `NotConfigured`; shared by every use case that talks to Jira.
pub(crate) async fn require_connection(
    connections: &Arc<dyn JiraConnectionRepositoryPort>,
) -> Result<JiraConnection, JiraError> {
    connections.get().await?.ok_or(JiraError::NotConfigured)
}
