use std::sync::Arc;

use crate::modules::jira::application::usecases::get_connection::require_connection;
use crate::modules::jira::domain::errors::jira_error::JiraError;
use crate::modules::jira::domain::models::jira_transition::JiraTransition;
use crate::modules::jira::domain::ports::outbound::jira_client_port::JiraClientPort;
use crate::modules::jira::domain::ports::outbound::jira_connection_repository_port::JiraConnectionRepositoryPort;

/// Status changes Jira allows right now for the issue (they depend on its workflow).
pub struct ListIssueTransitions {
    connections: Arc<dyn JiraConnectionRepositoryPort>,
    client: Arc<dyn JiraClientPort>,
}

impl ListIssueTransitions {
    pub fn new(
        connections: Arc<dyn JiraConnectionRepositoryPort>,
        client: Arc<dyn JiraClientPort>,
    ) -> Self {
        Self {
            connections,
            client,
        }
    }

    pub async fn execute(&self, key: &str) -> Result<Vec<JiraTransition>, JiraError> {
        let connection = require_connection(&self.connections).await?;
        self.client.transitions(&connection, key).await
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::modules::jira::mocks::{MockJiraClient, MockJiraConnectionRepository};

    #[tokio::test]
    async fn fails_without_connection() {
        let mut connections = MockJiraConnectionRepository::new();
        connections.expect_get().returning(|| Ok(None));
        let result =
            ListIssueTransitions::new(Arc::new(connections), Arc::new(MockJiraClient::new()))
                .execute("A-1")
                .await;
        assert_eq!(result, Err(JiraError::NotConfigured));
    }
}
